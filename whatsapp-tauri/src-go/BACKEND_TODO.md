# Backend TODO

This file tracks backend work needed to match the frontend contract and make real WhatsApp data reliable. The frontend now tolerates incomplete data, but these backend changes are still required for correct behavior.

## API Contract Required By Frontend

### `GET /api/chats`

Each chat should return this shape:

```json
{
  "id": "123@s.whatsapp.net",
  "participants": [],
  "lastMessage": {
    "id": "message-id",
    "chatJid": "123@s.whatsapp.net",
    "senderId": "123@s.whatsapp.net",
    "text": "Hello",
    "timestamp": "2026-07-19T12:00:00Z",
    "status": "received",
    "mediaType": "",
    "isFromMe": false
  },
  "unreadCount": 0,
  "isGroup": false,
  "name": "Alice",
  "avatar": "",
  "isArchived": false,
  "isStarred": false,
  "isCommunity": false
}
```

Required details:

- Return `participants: []` instead of `null` when participants are unknown. ✅ Done
- Include `lastMessage.isFromMe` so frontend can render sent/received bubbles correctly.
- Use RFC3339 timestamps consistently.
- Keep `lastMessage.status` as one of `received`, `sent`, `delivered`, or `read`.
- Fill `name` for groups and known direct contacts when possible. ✅ Done for stored contacts/groups
- Fill `isGroup` based on the JID server, not only history metadata. ✅ Done

### `GET /api/chats/:id`

Returns a cursor-paginated message page for a chat, ordered oldest-to-newest in the response.

Required details:

- Include `isFromMe` on every message.
- Include `chatJid`, `senderId`, `text`, `timestamp`, `status`, and `mediaType`.
- Return `[]` for unknown/empty chats, not `null`.
- Escape or route chat IDs safely because JIDs contain `@` and may contain other URL-sensitive characters.
- Support `limit`, `before`, and `after` cursors for older-page loading and polling deltas.
- Support `anchor=oldestUnread` for loading a window around the oldest unread inbound message. ✅ Done

### `GET /api/contacts`

Return contacts independently from chats:

```json
[
  {
    "id": "123@s.whatsapp.net",
    "name": "Alice",
    "avatar": "",
    "status": ""
  }
]
```

Required details:

- Return `[]` when no contacts are available.
- Populate names from push names, history sync, contact sync, or a readable fallback.
- Avoid requiring chat participants for contacts. The frontend now calls this endpoint directly.

### `POST /api/chats/:id/send` - ✅ Done

Frontend now calls this endpoint from the composer.

Request:

```json
{
  "text": "Hello"
}
```

Response should be the stored/sent message:

```json
{
  "id": "message-id",
  "chatJid": "123@s.whatsapp.net",
  "senderId": "me-or-own-jid",
  "text": "Hello",
  "timestamp": "2026-07-19T12:00:00Z",
  "status": "sent",
  "mediaType": "",
  "isFromMe": true
}
```

Required behavior:

- Reject empty text with `400`.
- Return `503` if WhatsApp is not connected.
- Parse the chat JID and call `wa.client.SendMessage()`.
- Persist the sent message to SQLite.
- Update chat metadata in the same operation or transaction.
- Return JSON, not plain text.

## Critical Backend Fixes

### 1. Update Chat Metadata When Inserting Messages - ✅ Done

Current issue:

- `InsertMessage` writes to `messages` only.
- Chat list depends on `chats.last_message_*`.
- Result: real messages can be stored but invisible in the chat list.

Required implementation:

- Change message insertion to also upsert/update the parent chat.
- Update `last_message_id`, `last_message_text`, `last_message_timestamp`, `last_message_sender`, and `updated_at`.
- Increment `unread_count` for received messages when appropriate.
- Do this transactionally so chats and messages cannot diverge.
- Keep `INSERT OR IGNORE` duplicate handling, but still consider whether chat metadata should update when a duplicate newer last message is seen.

Files:

- `userdata.go`
- `wa.go`

Acceptance criteria:

- A new incoming message appears in `/api/chats` as `lastMessage`.
- The same message appears in `/api/chats/:id`.
- Replaying the same message does not duplicate rows.

### 2. Persist History Sync - ⚠️ Partial

Current issue:

- `events.HistorySync` now persists conversations and parseable messages.
- Initial pairing can backfill old chats/messages.
- Group participants from joined-group sync, avatars, and richer group metadata are still incomplete.

Required implementation:

- ✅ Iterate over `v.Data.GetConversations()`.
- ✅ For each conversation, upsert a chat using conversation ID, name/display name, unread count, archive status, and group status.
- ✅ For each `HistorySyncMsg`, use `wa.client.ParseWebMessage(chatJID, historyMsg.GetMessage())` to convert it into the same shape used for live messages.
- ✅ Reuse the same message persistence path used by `events.Message`.
- ⚠️ Store participants for groups from joined-group sync; history-sync conversation participant extraction remains incomplete.
- ✅ Store push names/contact names where available.
- ✅ Log sync type, chunk order, progress, conversations inserted, messages inserted, and skipped messages.

Files:

- `wa.go`
- `userdata.go`
- possibly `types.go`

Acceptance criteria:

- After pairing, `/api/chats` contains historical chats.
- Opening a historical chat returns old messages.
- Groups show a group name when history sync provides one.
- Duplicate chunks do not duplicate messages.

### 3. Return Participants Safely - ✅ Done

Current issue:

- Older versions returned `Chat.Participants` as `null`; current `GetChats()` initializes it to an empty array.

Required implementation:

- Initialize `Participants` to `[]User{}` in `GetChats()`.
- Prefer joining contact/group participant data when available.
- If participants are unknown, return an empty array.
- For direct chats, resolve the stored direct contact once known.

Files:

- `types.go`
- `userdata.go`

Acceptance criteria:

- `/api/chats` never returns `participants: null`.

### 4. Use `isFromMe` As First-Class Data - ⚠️ Partial

Current issue:

- Backend stores `is_from_me` and message pages expose it.
- Chat list `lastMessage` still does not expose the full message contract.

Required implementation:

- ❌ Ensure `GetChats()` includes `is_from_me` when creating `LastMessage`.
- ✅ Ensure all message responses include `isFromMe`.
- ✅ Keep `senderId` as the real WhatsApp sender JID.

Files:

- `userdata.go`
- `types.go`

Acceptance criteria:

- Own messages from `/api/chats` and `/api/chats/:id` contain `isFromMe: true`.
- Incoming messages contain `isFromMe: false`.

### 5. Add Real Routing And Method Validation - ✅ Done

Current issue:

- `http.HandleFunc("/api/chats/", handleMessages)` catches all nested chat paths.
- Future routes like `/api/chats/:id/send` will conflict.

Required implementation:

- Route based on path segments and HTTP method.
- Support at least:
  - `GET /api/chats`
  - `GET /api/chats/:id`
  - `POST /api/chats/:id/send`
  - future: `POST /api/chats/:id/typing`
  - future: `POST /api/chats/:id/read`
- Return `405` for unsupported methods.
- Return JSON errors consistently.

Files:

- `main.go`
- `handlers.go`

Acceptance criteria:

- `/api/chats/:id/send` does not hit the message-list handler.
- Unknown nested paths return `404` JSON.

### 6. Lock Down Local API Exposure

Current issue:

- Backend listens on all interfaces with permissive CORS.

Required implementation:

- Change listen address from `:8090` to `127.0.0.1:8090`.
- Restrict CORS to the Tauri/Next frontend origin used in dev and production.
- Consider adding a random per-run token shared with the frontend/Tauri shell before exposing sensitive endpoints.

Files:

- `main.go`
- `handlers.go`

Acceptance criteria:

- Backend is not reachable from another machine on the LAN.
- Browser origins outside the app/dev origin cannot freely call the API.

## Important Follow-Up Work

### Message Pagination - ✅ Done

Implemented query params and cursor response fields for `GET /api/chats/:id`:

- `limit` with a bounded maximum.
- `before` cursor for older messages.
- `after` cursor for polling message/status deltas.
- Responses include `messages`, `nextCursor`, `latestCursor`, and `hasMore`.

### Read Receipts

Implement `POST /api/chats/:id/read`:

- Mark local unread count as zero.
- Call the appropriate whatsmeow read receipt method.
- Return `204` or a JSON result.

### Typing Indicators

Implement `POST /api/chats/:id/typing`:

- Accept `typing`, `recording`, `pause`, or `off`.
- Call `SendChatPresence`.
- Later, persist remote typing/presence state if needed.

### Search Endpoint

Storage already has FTS support. Add an HTTP endpoint:

- `GET /api/search?q=...&sender=...&mediaType=...&after=...&limit=...&offset=...`
- Validate and sanitize FTS queries.
- Return matching messages with chat IDs.

Current status: SQLite FTS storage and search queries exist, but the HTTP endpoint and frontend integration are still missing.

### Group Metadata

Persist group details from history sync and group events:

- Group name/topic.
- Group participants.
- Group avatar when available.
- Group update events.

### Profile/Current User Endpoint - ✅ Done

Added `GET /api/profile`:

- Returns own JID and push name when available.
- Frontend uses this instead of guessing the current user from chat participants.
- Avatar and richer connection/session info are still future enhancements.

## Testing To Add

- ✅ Message pagination boundaries, cursors, equal timestamps, deltas, and receipt-status changes.
- ✅ Route matching sends `/api/chats/:id/send` to the send handler, not the list-messages handler.
- ✅ `POST /api/chats/:id/send` validates malformed/empty/oversized text before network calls.
- ✅ `POST /api/chats/:id/send` returns structured JSON errors for unavailable WhatsApp and invalid chat IDs.
- ✅ Migration backfills existing message timestamp metadata and revisions.
- ❌ `InsertMessage` updates chat metadata.
- ❌ Duplicate message insert does not duplicate rows.
- ✅ `GetChats()` returns `participants: []`, not `null`.
- ❌ `GetChats()` includes `lastMessage.isFromMe`.
- ❌ History sync sample data inserts chats/messages idempotently.
