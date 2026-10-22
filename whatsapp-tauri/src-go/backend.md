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
  "qr": "",
  "error": ""
}
```

| Field | Values |
|-------|--------|
| `status` | `"unauthenticated"`, `"connecting"`, `"connected"`, `"client_outdated"` |
| `qr` | base64 PNG data URL (`data:image/png;base64,...`) or empty string |
| `error` | explanatory message when authentication failed due to an outdated client; otherwise empty string |

**Behavior**:
- If status is `"connected"`, frontend should immediately transition to chat view.
- During pairing, status may be `"connecting"` with an empty `qr` while the initial QR is being prepared; this phase times out after 30 seconds.
- Pairing failures, QR-channel errors/closure, and QR expiry clear the QR and return status to `"unauthenticated"`, allowing the frontend to retry.
- At startup, the backend fetches the current WhatsApp Web client version before creating the whatsmeow client. If the fetch fails, whatsmeow's built-in version is used.
- `ClientOutdated` (`err-client-outdated`) sets status to `"client_outdated"`, clears the QR, and returns an explanatory `error`. Restart the backend to retry fetching the current version; resetting the saved session does not resolve this error.
- Once a QR is received, the 30-second initial-QR timeout no longer applies; WhatsApp's QR-channel terminal events determine the outcome.
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
- Connection is started asynchronously while the QR channel is monitored.
- If the initial QR is not received within 30 seconds, or pairing fails/ends without success, clear the QR and restore `"unauthenticated"` so a retry can start.
- A stale pairing attempt cannot overwrite the state of a newer attempt.

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
- Null contact fields in placeholder rows are treated as empty when resolving direct-chat names.
- Group participants are returned when stored; otherwise `participants` is `[]`.
- Each group participant includes numeric `rank`: `0` member, `1` admin, `2` superadmin. This field is omitted from direct-chat participants and contact responses. No new endpoint is required.
- Participants come from joined-group snapshots, explicit history conversation `participant` records, and live group-info join/leave/promote/demote lists. History inserts missing members only: repeated, partial, or participant-free chunks never delete members or overwrite existing ranks. Missing history ranks default to member for newly inserted rows. Message senders and `pastParticipants` are not treated as current membership evidence.
- Participant IDs are non-device phone JIDs where a reliable mapping is available, otherwise valid LIDs are retained. Legacy phone servers are normalized. Learning a mapping merges participant aliases transactionally, preserving the higher rank on collisions; subsequent live/snapshot updates can explicitly demote members. This normalization does not change direct-chat identity.
- Contact enrichment is optional. Empty contact placeholders satisfy storage foreign keys; names fall back to push name, phone number, or the participant identifier. Unmapped participants remain visible.
- Live participant changes publish the existing chat-scoped SSE invalidation after persistence. History chunks use the existing chat/resync invalidations, including mapping-only chunks.
- Live leaves atomically delete active membership and persist a group/participant departure marker in `participant_departures`. History and role-only events cannot restore departed members, including after restarts. An explicit live join clears the marker atomically with its participant upsert.
- Requested joined-group snapshots capture the durable `participant_revision` before the network request. A snapshot can restore an explicitly listed member only if their departure was already recorded when that request began. Departures processed while the snapshot was in flight remain protected. Unsolicited `JoinedGroup` snapshots lack this boundary and cannot clear departure markers. Snapshots never infer removals from omitted members.
- Departure markers follow reliable LID/phone mappings, including mappings learned later. Startup migration also reconciles participant aliases already stored by older versions. When previously unmapped aliases have contradictory membership evidence, a departure wins conservatively until a known rejoin or eligible snapshot. Markers are not time-expired; they are removed on confirmed restoration or deletion of the group.
- Limitations: history is partial; existing ranks are intentionally not refreshed from history. In-flight snapshot protection covers departures, not all rank updates or out-of-order live events. Unknown identity mappings, missed events, and incomplete snapshots still prevent a guaranteed complete current roster. Departure markers protect observed leaves only; older versions did not retain departures, so those cannot be reconstructed on upgrade. No avatar downloading or frontend rendering is included; UI follow-up: [#72](https://github.com/Dan1718/watauri/issues/72).
- Group chats include a nested `groupMetadata` object when group metadata has been received. It includes available description, owner/creation, restrictions, disappearing-message, approval, community relationship, suspension, and deletion fields. The existing top-level `isAnnouncement` and `isSuspended` values remain compatibility aliases.
- Group metadata is populated from joined-group sync, available history-sync conversation fields, and live group-info events. Partial updates modify only supplied values; absent fields preserve stored values.
- A `lastMessage`, when present, is loaded from its stored message row and includes its chat JID, sender, text, timestamp, status, media type when non-empty, and `isFromMe` value. If the chat references a missing message row, `lastMessage` is omitted.
- Metadata-only chat updates preserve the stored `unreadCount`; history-sync conversation counts are authoritative, and only newly inserted live inbound messages increment the count.
- Real-time invalidations are streamed over `GET /api/events`; polling remains the fallback when SSE is disconnected.

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
- `anchor=oldestUnread` — returns a window around the inbound message at the unread boundary. The boundary is determined by the chat's `unreadCount`: for a count of `N`, it is the Nth newest inbound message, regardless of individual message statuses. Outgoing messages do not contribute to the count.

**Behavior**:
- Returns `[]` in `messages` for unknown or empty chats.
- Message responses include `chatJid`, `senderId`, `text`, `timestamp`, `status`, and `isFromMe`; `mediaType` is omitted when empty because it uses `omitempty`.
- Message responses include `mentionedJids`, an ordered array of the JIDs listed in the message payload's `ContextInfo`. Messages without mention metadata return an empty array. Live messages and parsed history-sync messages use the same extraction and persistence path.
- Mention JIDs are retained as supplied by WhatsApp, including LIDs. Existing LID/phone-JID mappings remain available in group-participant identity data so callers can match aliases to saved contacts or use the full phone JID as a fallback identifier.
- For a zero unread count, returns the latest page. If the unread count exceeds the number of stored inbound messages, anchors at the oldest available inbound message.

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

**Behavior**: Populated from whatsmeow contact/history sync events and push-name events. Null fields in placeholder rows are returned as empty values.

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

Server-Sent Events stream. Notifications identify changed data; clients refetch through the existing SQLite-backed endpoints.

**Response**: `text/event-stream`

**Named event types and JSON data**:
```
event: message
data: {"type":"message","chatId":"chat-1","messageIds":["msg-123"]}

event: receipt
data: {"type":"receipt","chatId":"chat-1","messageIds":["msg-123"]}

event: chat
data: {"type":"chat","chatId":"chat-1"}

event: sync
data: {"type":"sync","progress":75,"syncType":"INITIAL_BOOTSTRAP"}

event: resync
data: {"type":"resync"}
```

**Behavior**:
- Sends a keep-alive comment every 20 seconds; disconnected clients are removed.
- EventSource reconnects automatically. On each connection/reconnection, the frontend refreshes current state through existing APIs.
- Message and receipt events identify the affected chat and message IDs; the active conversation refetches changes using its revision cursor. The sidebar refreshes from `GET /api/chats`.
- Chat invalidations may include a chat ID; history sync can emit a global `resync` invalidation.
- A slow client's bounded queue is replaced with a `resync` event rather than blocking WhatsApp event processing.
- Event delivery is best-effort, not replayable. Polling remains active while SSE is unavailable as a fallback.

**Status**: Implemented.

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

#### Local SQLite metadata schema

- `chats` includes avatar identifiers/status/update time and pin/mute fields in addition to existing archive/star/activity state.
- `group_metadata` stores group description, owner/creation details, group settings, and parent/subgroup relationships in a one-to-one row linked to `chats.jid`; deleting a chat cascades to this row.
- Group metadata is populated from joined-group sync, available history-sync conversation fields, and live group-info events, and exposed as the nested `groupMetadata` field on group chats.
- Partial history/live updates preserve metadata fields omitted from the incoming payload.

1. **Phase 3** — SQLite schema, event persistence, real chat/message queries, and SSE invalidations are implemented in part. History sync is partial.

2. **Phase 4** — Text sending and read receipts are implemented. Typing, archive/star, and media upload/download are not implemented.

3. **Phase 5** — FTS5 storage/query code exists, but there is no search HTTP endpoint; custom groups and bulk actions are not implemented.
