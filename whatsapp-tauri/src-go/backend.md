# Backend API Reference

Base URL: `http://localhost:8090`
All endpoints require CORS headers (`Access-Control-Allow-Origin: *`).

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

**Status**: ✅ Existing

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

**Status**: ✅ Existing

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

**Status**: ✅ Existing, needs concurrency fix

---

### `POST /api/auth/logout`

Disconnects the client and revokes the device session. Deletes session DB.

**Response `200`** — empty body.

**Behavior**:
- Currently only calls `Disconnect()`.
- **Needs**: Call whatsmeow `client.Logout()` to revoke device, delete `wa-session.db`, reset `wa.qrCode` and `wa.status`.

**Status**: ⚠️ Needs fix (currently disconnect-only, no revocation)

---

### `POST /api/auth/reset`

Convenience endpoint for testing. Clears session without revoking — just disconnects and deletes the session DB file.

**Response `200`**

**Behavior**:
- Calls `Disconnect()`
- Deletes `wa-session.db`
- Resets `wa.status = "unauthenticated"`, `wa.qrCode = ""`
- Useful during development to start fresh without scanning again

**Status**: ❌ Missing

---

## Chats

### `GET /api/chats`

Returns all chats for the authenticated user.

**Response `200`**
```json
[
  {
    "id": "chat-1",
    "participants": [
      { "id": "user-1", "name": "Alice", "avatar": "https://...", "status": "online" }
    ],
    "lastMessage": { "id": "msg-123", "senderId": "user-1", "text": "Hey!", "timestamp": "10:30 AM", "status": "read" },
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
- Phase 3: Replace `mockChats` with real SQLite query.
- Phase 3: Add real-time updates via SSE.
- Archived chats should be included but filtered by frontend.

**Status**: ✅ Existing (mocks), ⚠️ Needs real DB query

---

### `GET /api/chats/:id`

Returns messages for a specific chat.

**Response `200`**
```json
[
  {
    "id": "msg-456",
    "senderId": "me",
    "text": "Sure, see you then!",
    "timestamp": "10:31 AM",
    "status": "read"
  }
]
```

**Response `404`** — chat not found.

**Behavior**:
- Phase 3: Replace `mockMessages` with real SQLite query.
- Messages ordered by timestamp ascending.
- Consider pagination for large chats (`?limit=50&before=msg-xxx`).

**Status**: ✅ Existing (mocks), ⚠️ Needs real DB query + pagination

---

### `POST /api/chats/:id/send`

Sends a text message to a chat. Attachments handled via separate media flow.

**Request**
```json
{
  "text": "Hello! How are you?"
}
```

**Response `201`**
```json
{
  "id": "msg-789",
  "status": "sent",
  "timestamp": "10:32 AM"
}
```

**Response `400`** — empty text.

**Response `503`** — not connected to WhatsApp.

**Behavior**:
- Calls `wa.client.SendMessage()`.
- Returns the message ID and server timestamp.
- Message should also be persisted to local SQLite.

**Status**: ❌ Missing

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

**Status**: ❌ Missing

---

### `POST /api/chats/:id/read`

Marks all messages in a chat as read.

**Request**: No body.

**Response `204`**

**Behavior**:
- Called when user opens a chat.
- Calls `wa.client.MarkRead()` with the last message ID.
- Resets `unreadCount` in local DB.

**Status**: ❌ Missing

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

**Status**: ❌ Missing

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

**Status**: ❌ Missing

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

**Status**: ❌ Missing

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

**Behavior**: Phase 3 — populated from whatsmeow contact sync events.

**Status**: ❌ Missing

---

### `GET /api/contacts/:id`

Returns a single contact.

**Response `200`** — single user object.
**Response `404`**

**Status**: ❌ Missing

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

**Status**: ❌ Missing

---

### `GET /api/media/:id`

Downloads a media blob (received or sent).

**Response `200`** — binary with `Content-Type` header.

**Status**: ❌ Missing

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

**Status**: ❌ Missing

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
- Highlight snippets are returned as part of results.

**Status**: ❌ Missing

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

**Status**: ❌ Missing

---

### `GET /api/groups`

List all custom groups.

**Response `200`**
```json
[
  { "id": "group-1", "name": "Work", "chatIds": ["chat-1"], "color": "#4CAF50" }
]
```

**Status**: ❌ Missing

---

### `PUT /api/groups/:id`

Update a custom group.

**Request**: Same shape as POST, all fields optional (partial update).

**Response `200`** — updated group object.

**Status**: ❌ Missing

---

### `DELETE /api/groups/:id`

Delete a custom group. Does not affect the actual chats.

**Response `204`**

**Status**: ❌ Missing

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

**Status**: ❌ Missing

---

## Implementation Notes

### Error Responses

All endpoints return errors in a consistent format:
```json
{
  "error": "human-readable message",
  "code": "ERROR_CODE"
}
```

| HTTP Status | When |
|-------------|------|
| `400` | Bad request (missing/invalid params) |
| `404` | Resource not found |
| `409` | Conflict (e.g., auth already in progress) |
| `500` | Internal server error |
| `503` | Service unavailable (not connected to WhatsApp) |

### Phase Migration Plan

1. **Phase 3** — Add SQLite schema (`chats`, `messages`, `contacts`, `media` tables). Wire whatsmeow event handlers to persist data. Replace mock handlers with real queries. Add SSE endpoint.

2. **Phase 4** — Add send, typing, read receipts, archive/star, media upload/download, local-only sync version.

3. **Phase 5** — Add search (FTS5), custom groups, bulk actions behind feature flags.

### Types (Go structs)

```go
type User struct {
  ID     string `json:"id"`
  Name   string `json:"name"`
  Avatar string `json:"avatar"`
  Status string `json:"status"`
}

type Message struct {
  ID        string `json:"id"`
  SenderID  string `json:"senderId"`
  Text      string `json:"text"`
  Timestamp string `json:"timestamp"`
  Status    string `json:"status"`
  MediaID   string `json:"mediaId,omitempty"`
}

type Chat struct {
  ID           string   `json:"id"`
  Participants []User   `json:"participants"`
  LastMessage  *Message `json:"lastMessage,omitempty"`
  UnreadCount  int      `json:"unreadCount"`
  IsGroup      bool     `json:"isGroup"`
  Name         *string  `json:"name,omitempty"`
  Avatar       *string  `json:"avatar,omitempty"`
  IsArchived   bool     `json:"isArchived"`
  IsStarred    bool     `json:"isStarred,omitempty"`
  IsCommunity  bool     `json:"isCommunity,omitempty"`
}
```
