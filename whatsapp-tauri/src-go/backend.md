# Backend API Reference

Base URL: `http://127.0.0.1:8090` for local clients. **Current implementation caveat:** the server actually binds to `:8090` (all interfaces). Middleware sets `Access-Control-Allow-Origin: *`, allows `Content-Type`, and advertises `GET, POST, OPTIONS`; this permissive configuration is a known security issue. This document distinguishes implemented routes from proposed API contracts.

---

## Health

### `GET /health`

Returns server status. Used by the title bar badge and frontend connectivity checks.

**Response `200`**
```json
{
  "name": "whatsapp-tauri",
  "status": "ok",
  "mode": "go"
}
```

| Field | Values |
|-------|--------|
| `mode` | `"go"` — sidecar HTTP API |
| `status` | `"ok"` |

**Status**: Implemented (`GET`)

---

## Auth

### `GET /api/auth/status`

Returns current authentication state and any available QR code.

**Response `200`**
```json
{
  "status": "unauthenticated",
  "qr": ""
}
```

| Field | Values |
|-------|--------|
| `status` | `"unauthenticated"`, `"connecting"`, `"connected"` |
| `qr` | base64 PNG data URL (`data:image/png;base64,...`) or empty string |

**Behavior**:
- If status is `"connected"`, frontend should immediately transition to chat view.
- If status is `"connecting"`, `qr` should contain the current QR code.
- Called on login screen mount and every 2s during polling.

**Status**: Implemented (`GET`; method validation is not enforced on this route)

---

### `POST /api/auth/start`

Triggers new QR pairing. If already connected/connecting, should be a no-op (not error).

**Request**: No body.

**Response `200`**
```json
{
  "status": "connecting",
  "qr": "data:image/png;base64,..."
}
```

**Behavior**:
- Must check `wa.status` before calling `Connect()`. If status is already `"connected"` or `"connecting"`, return immediately without action.
- Must wrap `wa.status = "connecting"` in the mutex.
- `Connect()` is blocking — must run in a goroutine.
- QR codes arrive via `events.QR` event handler which sets `wa.qrCode` and `wa.status`.

**Status**: Implemented (`POST`; pairing is a no-op unless unauthenticated)

---

### `POST /api/auth/logout`

Disconnects the client and asks whatsmeow to revoke the device session. The handler does not explicitly delete the database file.

**Response `200`** — empty body.

**Behavior**:
- Calls whatsmeow `client.Logout()` to revoke the device session.
- Resets `wa.qrCode` and `wa.status`.

**Status**: Implemented (`POST`)

---

### `POST /api/auth/reset`

Development convenience endpoint. Disconnects and deletes the stored whatsmeow device/session data without sending the normal logout/revoke flow.

**Response `200`**

**Behavior**:
- Calls `Disconnect()`
- Deletes the stored device data through whatsmeow (the handler does not directly remove the database file)
- Resets `wa.status = "unauthenticated"`, `wa.qrCode = ""`
- Useful during development to start fresh without scanning again

**Status**: Implemented (`POST`; development reset)

---

## Chats

### `GET /api/chats`

Returns all chats for the authenticated user from local SQLite storage.

**Response `200`**
```json
[
  {
    "id": "chat-1",
    "participants": [
      { "id": "user-1", "name": "Alice", "avatar": "https://...", "status": "online" }
    ],
    "lastMessage": {
      "id": "msg-123",
      "chatJid": "chat-1",
      "senderId": "user-1",
      "text": "Hey!",
      "timestamp": "2026-07-19T12:00:00Z",
      "status": "delivered",
      "isFromMe": false
    },
    "unreadCount": 2,
    "isGroup": false,
    "name": null,
    "avatar": null,
    "isArchived": false,
    "isStarred": false,
    "isCommunity": false
  }
]
```

**Behavior**:
- Reads chat metadata from local SQLite.
- Includes archived chats; the frontend filters them.
- Direct-chat names are resolved from stored contacts when available.
- Group participants are returned when stored; otherwise `participants` is `[]`.
- A `lastMessage`, when present, is loaded from its stored message row and includes its chat JID, sender, text, timestamp, status, media type when non-empty, and `isFromMe` value. If the chat references a missing message row, `lastMessage` is omitted.
- Metadata-only chat updates preserve the stored `unreadCount`; history-sync conversation counts are authoritative, and only newly inserted live inbound messages increment the count.
- Real-time updates are not yet available; the frontend currently polls.

**Status**: Implemented (`GET`, SQLite-backed)

---

### `GET /api/chats/:id`

Returns a cursor-paginated message page for a specific chat. Messages are ordered oldest-to-newest in the response.

**Response `200`**
```json
{
  "messages": [
    {
    "id": "msg-456",
    "senderId": "me",
    "text": "Sure, see you then!",
    "timestamp": "10:31 AM",
    "status": "read"
    }
  ],
  "nextCursor": "...",
  "latestCursor": "...",
  "hasMore": true,
  "olderCursor": "...",
  "newerCursor": "...",
  "hasOlder": true,
  "hasNewer": false
}
```

Unknown chat IDs currently return an empty message page (`200`), not `404`.

**Query parameters**:
- `limit` — bounded page size.
- `before` — cursor for older messages.
- `after` — revision cursor for polling deltas, or a time cursor for newer messages.
- `anchor=oldestUnread` — returns a window around the oldest unread inbound message.

**Behavior**:
- Returns `[]` in `messages` for unknown or empty chats.
- Message responses include `chatJid`, `senderId`, `text`, `timestamp`, `status`, and `isFromMe`; `mediaType` is omitted when empty because it uses `omitempty`.

**Status**: Implemented (`GET`; SQLite, pagination, oldest-unread anchor)

---

### `POST /api/chats/:id/send`

Sends and persists a text message to a chat. Attachment sending is not implemented.

**Request**
```json
{
  "text": "Hello! How are you?"
}
```

**Response `200`**
```json
{
  "id": "msg-789",
  "status": "sent",
  "chatJid": "123@s.whatsapp.net",
  "senderId": "me-or-own-jid",
  "text": "Hello! How are you?",
  "timestamp": "2026-07-19T12:00:00Z",
  "mediaType": "",
  "isFromMe": true
}
```

**Response `400`** — empty text.

**Response `503`** — WhatsApp client unavailable. Invalid chat IDs return `400`, persistence failures `500`, and send failures `502`.

**Behavior**:
- Calls `wa.client.SendMessage()`.
- Returns the stored/sent message as JSON.
- Persists the sent message to local SQLite and updates chat metadata.

**Status**: Implemented (`POST`)

---

### `POST /api/chats/:id/typing`

Sends a typing/presence indicator.

**Request**
```json
{
  "action": "typing" | "recording" | "pause" | "off"
}
```

**Response `204`** — no content.

**Behavior**:
- `"typing"` and `"recording"` start the indicator with a 10s timeout.
- `"pause"` resets the timeout.
- `"off"` immediately stops.
- Calls `wa.client.SendChatPresence()`.

**Status**: Not implemented; proposed contract only.

---

### `POST /api/chats/:id/read`

Marks all messages in a chat as read.

**Response `200`**

**Request**: Optional JSON body with `sendReceipt` and `messageIds`.

**Behavior**:
- Called when user opens a chat.
- Calls `wa.client.MarkRead()` when `sendReceipt` is enabled.
- Marks selected unread inbound messages read and updates local `unreadCount`.

**Status**: Implemented (`POST`; optional `sendReceipt` and `messageIds` body)

---

### `PUT /api/chats/:id/archive`

Toggles archive status.

**Request**
```json
{
  "archived": true
}
```

**Response `204`**

**Behavior**:
- Persists to local SQLite.
- Phase 4: Sync via whatsmeow app state mutation when available.

**Status**: Not implemented; proposed contract only.

---

### `PUT /api/chats/:id/star`

Toggles starred status.

**Request**
```json
{
  "starred": true
}
```

**Response `204`**

**Behavior**: Same as archive — local persistence first, protocol sync when available.

**Status**: Not implemented; proposed contract only.

---

### `GET /api/chats/:id/media`

Returns media messages in a chat.

**Response `200`**
```json
[
  {
    "id": "media-001",
    "type": "image" | "video" | "audio" | "document",
    "caption": null,
    "mimeType": "image/jpeg",
    "fileSize": 245760,
    "timestamp": "2024-06-15T14:30:00Z",
    "senderId": "user-1",
    "mediaUrl": "/api/media/media-001"
  }
]
```

**Status**: Not implemented; proposed contract only.

---

## Contacts

### `GET /api/contacts`

Returns all known contacts.

**Response `200`**
```json
[
  { "id": "user-1", "name": "Alice", "avatar": "https://...", "status": "online" }
]
```

**Behavior**: Populated from whatsmeow contact/history sync events and push-name events.

**Status**: Implemented (`GET`)

---

### `GET /api/contacts/:id`

Returns a single contact.

**Response `200`** — single user object.
**Response `404`**

**Status**: Not implemented; proposed contract only.

---

## Profile

### `GET /api/profile`

Returns the authenticated user's own JID and push name when available.

**Response `200`**
```json
{
  "id": "123@s.whatsapp.net",
  "pushName": "Alex"
}
```

**Status**: Implemented (`GET`)

---

## Media

### `POST /api/media/upload`

Uploads a file to be sent as a message attachment.

**Request**: `multipart/form-data`
| Field | Type |
|-------|------|
| `file` | binary |
| `caption` | string (optional) |

**Response `201`**
```json
{
  "id": "media-002",
  "url": "/api/media/media-002"
}
```

**Behavior**:
- Frontend uses Tauri file dialog, reads file, sends to this endpoint.
- Go sidecar buffers to a temp directory.
- Actual send is triggered by `POST /api/chats/:id/send` with `mediaId` field.

**Status**: Not implemented; proposed contract only.

---

### `GET /api/media/:id`

Downloads a media blob (received or sent).

**Response `200`** — binary with `Content-Type` header.

**Status**: Not implemented; proposed contract only.

---

## Real-time Events (SSE)

### `GET /api/events`

Server-Sent Events stream. Pushes real-time updates to the frontend.

**Response**: `text/event-stream`

**Event types**:
```
event: message
data: {"chatId":"chat-1","type":"new","message":{...}}

event: presence
data: {"userId":"user-1","status":"typing...","chatId":"chat-1"}

event: status
data: {"status":"connected"}

event: read
data: {"chatId":"chat-1","userId":"user-1","upToMessageId":"msg-123"}
```

**Behavior**:
- Keep-alive every 30s.
- Frontend connects on chat view mount, disconnects on unmount.
- Go sidecar maintains a map of connected SSE clients and broadcasts to all.

**Status**: Not implemented; proposed contract only.

---

## Search (Phase 5)

### `GET /api/search?q=keyword&type=messages&chatId=optional&limit=50`

Full-text search across messages and chats.

**Parameters**:
| Param | Values |
|-------|--------|
| `q` | search query (required) |
| `type` | `"messages"`, `"chats"`, `"media"`, `"all"` (default `"all"`) |
| `chatId` | scope to a single chat (optional) |
| `limit` | max results (default 50) |
| `offset` | pagination offset (default 0) |

**Response `200`**
```json
{
  "messages": [...],
  "chats": [...],
  "total": 142
}
```

**Behavior**:
- Backend uses SQLite FTS5.
- The current store query returns matching message records only; there is no endpoint or highlight-snippet implementation.

**Status**: Not implemented; proposed contract only.

---

## Custom Groups (Phase 5)

### `POST /api/groups`

Create a local custom chat group (not a WhatsApp group — a UI filter set).

**Request**
```json
{
  "name": "Work",
  "chatIds": ["chat-1", "chat-2"],
  "color": "#4CAF50",
  "icon": "briefcase"
}
```

**Response `201`**
```json
{
  "id": "group-1",
  "name": "Work",
  "chatIds": ["chat-1", "chat-2"]
}
```

**Status**: Not implemented; proposed contract only.

---

### `GET /api/groups`

List all custom groups.

**Response `200`**
```json
[
  { "id": "group-1", "name": "Work", "chatIds": ["chat-1"], "color": "#4CAF50" }
]
```

**Status**: Not implemented; proposed contract only.

---

### `PUT /api/groups/:id`

Update a custom group.

**Request**: Same shape as POST, all fields optional (partial update).

**Response `200`** — updated group object.

**Status**: Not implemented; proposed contract only.

---

### `DELETE /api/groups/:id`

Delete a custom group. Does not affect the actual chats.

**Response `204`**

**Status**: Not implemented; proposed contract only.

---

## Bulk Actions (Phase 5)

### `POST /api/messages/batch`

Perform an action on multiple messages at once.

**Request**
```json
{
  "messageIds": ["msg-1", "msg-2", "msg-3"],
  "action": "forward" | "delete" | "star" | "export"
}
```

**Response** varies by action:
- `forward`: `201` with `{ "chatId": "chat-5", "messageIds": [...] }`
- `delete`: `204`
- `star`: `204`
- `export`: `201` with `{ "downloadUrl": "/api/export/..." }`

**Status**: Not implemented; proposed contract only.

---

## Implementation Notes

### Current registered routes

Registered routes are `GET /health`, `GET /api/chats`, `GET /api/chats/:id`, `POST /api/chats/:id/send`, `POST /api/chats/:id/read`, `GET /api/contacts`, `GET /api/profile`, `GET /api/auth/status`, `POST /api/auth/start`, `POST /api/auth/logout`, and `POST /api/auth/reset`. Other endpoint descriptions above are proposed contracts, not live routes.

### Error Responses

Error formats are not yet consistent. Some handlers return JSON error objects, while others use `http.Error` and method errors are plain text. The current JSON error shape has no `code` field:
```json
{
  "error": "human-readable message"
}
```

| HTTP Status | When |
|-------------|------|
| `400` | Bad request (missing/invalid params) |
| `404` | Resource not found |
| `500` | Internal server error |
| `503` | Service unavailable (not connected to WhatsApp) |

These are common statuses, not an exhaustive mapping. For example, invalid cursors return `400`, unknown routes return `404`, and unsupported methods return `405` with a plain-text body.

### Historical phase notes (current status)

1. **Phase 3** — SQLite schema, event persistence, and real chat/message queries are implemented in part. History sync is partial; SSE is not implemented.

2. **Phase 4** — Text sending and read receipts are implemented. Typing, archive/star, and media upload/download are not implemented.

3. **Phase 5** — FTS5 storage/query code exists, but there is no search HTTP endpoint; custom groups and bulk actions are not implemented.
