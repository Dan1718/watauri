# Roadmap — WhatsApp Tauri Desktop Client

> **AI-generated disclaimer**: The frontend code (React components, styles, mock data) and most documentation (including this roadmap) were initially generated with AI assistance. Architecture decisions, protocol integration, and backend implementation are human-directed. This project uses AI as a productivity tool — all generated code is reviewed and modified by a human.

## Architecture

```
┌──────────────────────────────────────────────┐
│  Tauri Shell (Rust)                          │
│  ┌────────────────────────────────────────┐  │
│  │  Frontend (React + Tailwind)           │  │
│  │  └─ client.ts (fetch to :8090)         │  │
│  └──────────┬─────────────────────────────┘  │
│             │ HTTP                           │
│  ┌──────────▼─────────────────────────────┐  │
│  │  Sidecar: Go binary (src-go/backend)   │  │
│  │  ├── HTTP API handlers                 │  │
│  │  ├── whatsmeow client                  │  │
│  │  └── SQLite store                      │  │
│  └────────────────────────────────────────┘  │
└──────────────────────────────────────────────┘
```

**Decisions made:**

- **Backend**: Go, running as a Tauri sidecar (bundled binary, auto-spawned)
- **Protocol library**: [whatsmeow](https://github.com/tulir/whatsmeow) (native Go, ~10-20 MB RAM, battle-tested via Mautrix bridge)
- **Mock data**: Keep both frontend (`constants.ts`) and Go (`mock_data.go`) mocks during development, remove after real protocol integration

---

## Phase 1: Wire the Go sidecar into Tauri

| Task | Detail                                                                                     | Status  |
| ---- | ------------------------------------------------------------------------------------------ | ------- |
| 1.1  | Add `"bundle": { "externalBin": ["src-go/backend"] }` to `tauri.conf.json`                 | ✅ Done |
| 1.2  | Add `tauri-plugin-shell` dependency for sidecar process management                         | ✅ Done |
| 1.3  | Update `client.ts` to `fetch("http://localhost:8090/api/...")` instead of Tauri `invoke()` | ✅ Done |
| 1.4  | Drop the Rust `backend_health` command (moves to Go)                                       | ✅ Done |
| 1.5  | Remove the mock fallback in `client.ts` — no more silent error swallowing                  | ✅ Done |
| 1.6  | Update `package.json` scripts to build Go binary before `tauri dev` / `tauri build`        | ✅ Done |
| 1.7  | Resolve the `src-go/backend` naming confusion (binary vs source directory)                 | ✅ Done |

**Deliverable**: App starts, spawns Go sidecar, frontend fetches mock data from Go over HTTP. `mode` badge reads `"go"` or `"mock"`.

---

## Phase 2: Real WhatsApp auth via whatsmeow

| Task | Detail                                                                                                                 | Status  |
| ---- | ---------------------------------------------------------------------------------------------------------------------- | ------- |
| 2.1  | Add `github.com/tulir/whatsmeow` to `go.mod`                                                                           | ✅ Done |
| 2.2  | Implement QR provisioning endpoint: `POST /api/auth/start` — triggers whatsmeow connect, returns QR code with rotation | ✅ Done |
| 2.3  | Replace the static QR image in `LoginScreen.tsx` with real QR from the Go API                                          | ✅ Done |
| 2.4  | Polling endpoint: `GET /api/auth/status` — frontend waits for phone scan                                               | ✅ Done |
| 2.5  | Store session credentials on success (whatsmeow auto-saves to SQLite)                                                  | ✅ Done |
| 2.6  | Update health endpoint to return `mode: "whatsmeow"` after auth                                                        | ✅ Done |

**Deliverable**: Real QR code on login screen with auto-rotation, phone scan authenticates, session persists across restarts, logout revokes device, reset for dev testing.

---

## Phase 3: Real chat/message data

| Task | Detail                                                                               | Status    |
| ---- | ------------------------------------------------------------------------------------ | --------- |
| 3.1  | Add SQLite to the Go sidecar (`modernc.org/sqlite` + `mattn/go-sqlite3`)            | ✅ Done   |
| 3.2  | Wire whatsmeow event handlers → SQLite writes (messages, chats, contacts, presence)  | ⚠️ Partial |
| 3.3  | Replace Go mock handlers with real DB queries for `/api/chats` and `/api/chats/{id}` | ✅ Done   |
| 3.4  | Add real-time push from Go to frontend using Server-Sent Events                      | Planned |
| 3.5  | Support offline startup — load from SQLite first, sync when connected                | ✅ Done   |

**3.2 details**:
| Event | Handled | Notes |
|-------|---------|-------|
| `*events.Message` | ✅ | Text + media stored, placeholder chat upserted, reactions/receipts skipped |
| `*events.Receipt` | ✅ | Status updated (delivered → read) |
| `*events.PushName` | ✅ | Upserted to `contacts` table |
| `*events.Presence` | ⚠️ | Logged only, no status persisted |
| `*events.HistorySync` | ❌ | **Skipped** — contains all group names, bulk messages, participant lists, avatars |
| `*events.GroupInfo` | ❌ | Skipped — group name/topic changes not persisted |
| `*events.JoinedGroup` | ❌ | Skipped |
| `*events.LoggedOut` | ✅ | Status + QR reset |
| `*events.PairSuccess` | ❌ | Skipped |
| `*events.PairError` | ❌ | Skipped |
| `*events.Connected` | ✅ | Status set |
| `*events.Disconnected` | ✅ | Status reset |
| All others (KeepAliveTimeout, StreamReplaced, TemporaryBan, ConnectFailure, ChatPresence, Picture, etc.) | ❌ | Fall to `default: unhandled` |

**Known issues in Phase 3**:
- `GetChats()` never joins with `contacts` → 1-on-1 chats have `participants: null` and no display name (frontend works around with `chatName()` deriving from JID)
- `UpsertChat()` only inserts bare JID — no name/avatar — groups show as bare JIDs until HistorySync is processed
- `HistorySync` skipped → no bulk offline-history insert after pairing

**SSE scope**:

| Task | Detail | Status |
| ---- | ------ | ------ |
| SSE endpoint | Add `GET /api/events` with proper event-stream headers | Planned |
| Event broadcaster | Add backend pub/sub for connected frontend clients | Planned |
| Message events | Emit events when live or history messages are stored | Planned |
| Receipt events | Emit events when message statuses change | Planned |
| Chat events | Emit events when chat metadata changes | Planned |
| Sync events | Emit history sync progress and completion events | Planned |
| Frontend reconnect | Add EventSource reconnect and polling fallback behavior | Planned |

**Deliverable**: Real chats and messages appear in the UI (with fallback JID names). New messages arrive through polling until SSE is implemented.

---

## Phase 4: Feature parity — sending & interactions

| Task | Detail                                                                        | Status |
| ---- | ----------------------------------------------------------------------------- | ------ |
| 4.1  | Send messages: `POST /api/chats/{id}/send` → `client.SendMessage()`           | 🔜 Planned |
| 4.2  | Typing indicators: `PUT /api/chats/{id}/typing` → `client.SendChatPresence()` | 🔜 Planned |
| 4.3  | Read receipts: mark read on chat open                                         | 🔜 Planned |
| 4.4  | Attachments: Tauri file dialog → whatsmeow media upload                       | 🔜 Planned |
| 4.5  | Archive / star: persist to SQLite + sync via whatsmeow app state              | 🔜 Planned |
| 4.6  | Reactions: send/receive message reactions via whatsmeow                       | 🔜 Planned |
| 4.7  | Desktop notifications: Tauri notification plugin on incoming messages         | 🔜 Planned |

**Deliverable**: Feature-complete basic messaging client.

---

## Phase 5: Power-user features (the product differentiator)

These are the project's actual goals per the README. Ordered by priority:

| Priority | Feature                                               | Implementation                                                                  |
| -------- | ----------------------------------------------------- | ------------------------------------------------------------------------------- |
| 1        | **Custom chat groups & filters**                      | SQLite `groups` table + UI in Sidebar; frontend state only, no protocol changes |
| 2        | **Bulk actions** (multi-select, batch forward/export) | Multi-select state in MessageArea + batch API endpoints                         |
| 3        | **Better search** (full-text across messages)         | SQLite FTS5 for full-text search                                                |
| 4        | **Voice transcription**                               | Sidecar runs whisper.cpp or llama.cpp for local transcription                   |
| 5        | **Scheduled messages**                                | Cron-like timer in Go, fires `SendMessage` at given time                        |
| 6        | **Multiple accounts**                                 | Multiple whatsmeow clients, separate SQLite DBs, account switcher UI            |
| 7        | **Privacy controls** (suppress typing/read receipts)  | Intercept outgoing presence signals in whatsmeow event pipeline                 |
| 8        | **One-time media handling**                           | Detect view-once messages, controlled UX for viewing                            |
| 9        | **Better downloading / export**                       | Bulk media download, organized exports                                          |

Each risky feature should be behind a **feature flag** and isolated from core protocol code.

---

## Phase 6: Polish & ship

| Task | Detail                                                                                      |
| ---- | ------------------------------------------------------------------------------------------- |
| 6.1  | Tauri packaging: AppImage (Linux), .dmg (macOS), NSIS/MSI (Windows) with sidecar bundled    |
| 6.2  | Auto-update via `tauri-plugin-updater`                                                      |
| 6.3  | Tests: Go unit tests (handler logic), Playwright e2e (UI)                                   |
| 6.4  | Performance: virtual scrolling for message lists, lazy media loading, DB query optimization |
| 6.5  | Accessibility: keyboard nav, contrast verification, screen reader support                   |

---

## Tooling decisions

| Concern               | Choice                                          |
| --------------------- | ----------------------------------------------- |
| Backend language      | Go                                              |
| Protocol library      | [whatsmeow](https://github.com/tulir/whatsmeow) |
| Sidecar method        | Tauri `externalBin` + `tauri-plugin-shell`      |
| Local storage (Go)    | SQLite                                          |
| Frontend framework    | React 19                                        |
| Styling               | Tailwind CSS                                    |
| Desktop shell         | Tauri v2                                        |
| Desktop notifications | `tauri-plugin-notification`                     |
| Auto-update           | `tauri-plugin-updater`                          |

## Completed but unplanned

| Task | Detail |
| ---- | ------ |
| Frontend logging | `[api]`, `[sse]`, `[app]`, `[login]`, `[msgarea]`, `[sidebar]`, `[titlebar]`, `[nav]` — timing, state transitions, dedup |
| Backend debug logging | `[store]` DB timing, `[http]` request/response, `[wa]` event types |
| API reference | `src-go/backend.md` with 26 endpoints |
| Bruno collection | 24 `.bru` files under `docs/bruno/` (7 original + 17 planned) |
| Cross-compilation | Go build script, binary naming for target triple, `modernc.org/sqlite` avoids CGo |
| Blank-screen fixes | `getCurrentWindow()` try-catch, null-participant guards, message URL path fix |
| SSE removed | `GET /api/events` dropped (user decision), `subscribeToEvents()` kept in code, polling (5s) as temp replacement |
| Chat display name | `chatName()` helper — derives from JID when name/participants are null |
