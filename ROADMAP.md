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

The Go backend exists and compiles, but isn't connected to the Tauri shell.

| Task | Detail |
|------|--------|
| 1.1 | Add `"bundle": { "externalBin": ["src-go/backend"] }` to `tauri.conf.json` |
| 1.2 | Add `tauri-plugin-shell` dependency for sidecar process management |
| 1.3 | Update `client.ts` to `fetch("http://localhost:8090/api/...")` instead of Tauri `invoke()` |
| 1.4 | Drop the Rust `backend_health` command (moves to Go) |
| 1.5 | Remove the mock fallback in `client.ts` — no more silent error swallowing |
| 1.6 | Update `package.json` scripts to build Go binary before `tauri dev` / `tauri build` |
| 1.7 | Resolve the `src-go/backend` naming confusion (binary vs source directory) |

**Deliverable**: App starts, spawns Go sidecar, frontend fetches mock data from Go over HTTP. `mode` badge in title bar reads `"go"` or `"mock"`.

**File changes**: `tauri.conf.json`, `client.ts`, `package.json`, `TitleBar.tsx`

---

## Phase 2: Real WhatsApp auth via whatsmeow

Replace the cosmetic login screen with a real QR pairing flow.

| Task | Detail |
|------|--------|
| 2.1 | Add `github.com/tulir/whatsmeow` to `go.mod` |
| 2.2 | Implement QR provisioning endpoint: `POST /api/auth/qr` — triggers whatsmeow connect, returns QR code |
| 2.3 | Replace the static QR image in `LoginScreen.tsx` with real QR from the Go API |
| 2.4 | Polling endpoint: `GET /api/auth/status` — frontend waits for phone scan |
| 2.5 | Store session credentials on success (encrypted file or `tauri-plugin-store`) |
| 2.6 | Update health endpoint to return `mode: "whatsmeow"` after auth |

**Deliverable**: Real QR code on login screen, phone scan authenticates, session persists across restarts.

---

## Phase 3: Real chat/message data

Replace mock data with whatsmeow-synced data persisted in SQLite.

| Task | Detail |
|------|--------|
| 3.1 | Add SQLite to the Go sidecar (`modernc.org/sqlite` or `mattn/go-sqlite3`) |
| 3.2 | Wire whatsmeow event handlers → SQLite writes (messages, chats, contacts, presence) |
| 3.3 | Replace Go mock handlers with real DB queries for `/api/chats` and `/api/chats/{id}` |
| 3.4 | Add real-time push from Go to frontend (Server-Sent Events or WebSocket) |
| 3.5 | Support offline startup — load from SQLite first, sync when connected |

**Deliverable**: Real chats and messages appear in the UI. New messages arrive in real time.

---

## Phase 4: Feature parity — sending & interactions

| Task | Detail |
|------|--------|
| 4.1 | Send messages: `POST /api/chats/{id}/send` → `client.SendMessage()` |
| 4.2 | Typing indicators: `PUT /api/chats/{id}/typing` → `client.SendChatPresence()` |
| 4.3 | Read receipts: mark read on chat open |
| 4.4 | Attachments: Tauri file dialog → whatsmeow media upload |
| 4.5 | Archive / star: persist to SQLite + sync via whatsmeow app state |
| 4.6 | Desktop notifications: Tauri notification plugin on incoming messages |

**Deliverable**: Feature-complete basic messaging client.

---

## Phase 5: Power-user features (the product differentiator)

These are the project's actual goals per the README. Ordered by priority:

| Priority | Feature | Tag | Implementation |
|----------|---------|-----|----------------|
| 1 | **Custom chat groups & filters** | safe | SQLite `groups` table + UI in Sidebar; frontend state only, no protocol changes |
| 2 | **Bulk actions** (multi-select, batch forward/export) | safe | Multi-select state in MessageArea + batch API endpoints |
| 3 | **Better search** (full-text across messages) | safe | SQLite FTS5 for full-text search |
| 4 | **Voice transcription** | safe | Sidecar runs whisper.cpp or llama.cpp for local transcription |
| 5 | **Scheduled messages** | risky | Cron-like timer in Go, fires `SendMessage` at given time |
| 6 | **Multiple accounts** | risky | Multiple whatsmeow clients, separate SQLite DBs, account switcher UI |
| 7 | **Privacy controls** (suppress typing/read receipts) | risky | Intercept outgoing presence signals in whatsmeow event pipeline |
| 8 | **One-time media handling** | risky | Detect view-once messages, controlled UX for viewing |
| 9 | **Better downloading / export** | risky | Bulk media download, organized exports |

Each risky feature should be behind a **feature flag** and isolated from core protocol code.

---

## Phase 6: Polish & ship

| Task | Detail |
|------|--------|
| 6.1 | Tauri packaging: AppImage (Linux), .dmg (macOS), NSIS/MSI (Windows) with sidecar bundled |
| 6.2 | Auto-update via `tauri-plugin-updater` |
| 6.3 | Tests: Go unit tests (handler logic), Playwright e2e (UI) |
| 6.4 | Performance: virtual scrolling for message lists, lazy media loading, DB query optimization |
| 6.5 | Accessibility: keyboard nav, contrast verification, screen reader support |

---

## Current codebase issues to address

- [ ] `src-go/backend` is a compiled binary sitting alongside source — rename or restructure
- [ ] `src-go/backend/` is an unresolved path (possibly a submodule placeholder)
- [ ] Mock data duplicated across `constants.ts` and `mock_data.go` (intentional for now)
- [ ] `client.ts` silently swallows errors in the mock fallback
- [ ] Go backend port `:8090` is hardcoded — should use dynamic port discovery for sidecar
- [ ] No tests anywhere in the project

## Tooling decisions

| Concern | Choice |
|---------|--------|
| Backend language | Go |
| Protocol library | [whatsmeow](https://github.com/tulir/whatsmeow) |
| Sidecar method | Tauri `externalBin` + `tauri-plugin-shell` |
| Local storage (Go) | SQLite |
| Frontend framework | React 19 |
| Styling | Tailwind CSS |
| Desktop shell | Tauri v2 |
| Desktop notifications | `tauri-plugin-notification` |
| Auto-update | `tauri-plugin-updater` |
