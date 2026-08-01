# Features

WhatsApp Tauri is a desktop-first, power-user WhatsApp client focused on better organization, search, bulk workflows, media handling, and privacy controls.

This file tracks user-facing product features. For implementation details, architecture phases, and developer tasks, see [DEV_ROADMAP.md](./DEV_ROADMAP.md).

## Status Legend

- `done`: implemented
- `in-progress`: actively being built
- `planned`: intended but not started
- `experimental`: exploratory or uncertain
- `blocked`: waiting on another feature or technical decision

## Core Client

| Feature | Status | Notes |
| ------- | ------ | ----- |
| WhatsApp QR pairing | done | Pair account using the WhatsApp Web QR flow |
| Persistent session | done | Session survives app restart |
| Local SQLite storage | done | Stores chats and messages locally |
| Chat list | done | Loaded from local database |
| Message history view | done | Opens stored messages for a chat |
| History sync persistence | in-progress | Backfills historical chats and messages after pairing; batching is planned for large syncs |
| Contact names | in-progress | Saved names, push names, and LID/phone display fallback are being wired into backend chat/contact display |
| Contact and group avatars | planned | Fetch and persist WhatsApp profile picture URLs for contacts and groups |
| Group metadata | in-progress | Group names are partially supported through history sync; live metadata changes and avatars are pending |
| Group participants | in-progress | Joined-group sync persists participants; history-sync extraction and live updates remain incomplete |
| Communities | planned | Support WhatsApp community parent groups and related announcement/subgroup metadata |
| Community subgroups | planned | Display linked community groups and parent/subgroup relationships when available |
| Send text messages | done | Composer endpoint and whatsmeow send path |
| Message pagination | done | Cursor pagination avoids loading entire chats at once |
| Real-time updates | planned | Server-Sent Events for new messages, receipt changes, chat metadata updates, and sync progress |
| Read receipts | done | Opening a chat can mark inbound messages read locally and send WhatsApp read receipts; endpoint supports a `sendReceipt` privacy flag |
| Typing indicators | planned | Risky: protocol-sensitive behavior |
| Attachments | planned | Media upload and download flow |

## Desktop Experience

| Feature | Status | Notes |
| ------- | ------ | ----- |
| Desktop notifications | planned | Native notifications for incoming messages |
| Keyboard shortcuts | planned | Faster desktop navigation |
| Fast startup from local cache | planned | Load local data before remote sync |
| Window/session restore | planned | Better desktop behavior across restarts |
| Native file picker | planned | Attachment selection through Tauri |
| Better offline behavior | planned | App should remain useful from local data when disconnected |

## Organization

| Feature | Status | Notes |
| ------- | ------ | ----- |
| Custom chat groups | planned | User-defined chat groupings |
| Saved filters | planned | Reusable filters for chats and messages |
| Per-group notification priority | planned | Desktop-side notification rules |
| Mute or badge-only modes | planned | Local notification behavior |
| Chat tags or labels | planned | Local organization layer |
| Archive chats | planned | Archive and unarchive chats locally and eventually sync protocol state |
| Pinned chats | planned | Keep important chats at the top of the chat list |
| Pinned messages | planned | Pin important messages inside a chat |
| Favourite chats | planned | Mark important chats for quick access and filtering |
| Favourite contacts | planned | Mark contacts for quick access and filtering |
| Starred messages | planned | Mark important messages and expose starred-message views |

## Search

| Feature | Status | Notes |
| ------- | ------ | ----- |
| Full-text message search | planned | SQLite FTS storage/query support exists; the HTTP endpoint and frontend search UI are not wired |
| Search by sender | planned | Filter messages by sender |
| Search by date | planned | Time-based filtering |
| Search by media type | planned | Images, videos, audio, and documents |
| Search across groups/filters | planned | Search scoped to custom organization |
| Search voice transcripts | planned | Depends on voice note transcription |

## Bulk Actions

| Feature | Status | Notes |
| ------- | ------ | ----- |
| Multi-select messages | planned | Select many messages at once |
| Multi-select media | planned | Select many media items |
| Bulk forward | planned | Risky: can become automation-like |
| Bulk download | planned | Risky: privacy and data-handling implications |
| Bulk export | planned | Risky: privacy and data-handling implications |
| Bulk archive/delete | planned | Depends on local and protocol behavior |

## Media

| Feature | Status | Notes |
| ------- | ------ | ----- |
| Better media previews | planned | Improved viewing experience |
| Bulk media download | planned | Risky: data handling and retention concerns |
| Organized media exports | planned | Risky: export workflows need clear boundaries |
| Media retention handling | planned | Needs protocol and storage investigation |
| View-once media UX | experimental | Risky: must avoid violating expected platform behavior |

## Voice Notes

| Feature | Status | Notes |
| ------- | ------ | ----- |
| Local voice transcription | planned | Prefer local model/runtime |
| Searchable transcripts | planned | Store transcript text for search |
| Export transcripts | planned | Include transcript in export workflow |

## Privacy Controls

| Feature | Status | Notes |
| ------- | ------ | ----- |
| Suppress typing indicators | experimental | Risky: protocol-sensitive behavior |
| Suppress read receipts | experimental | Risky: protocol-sensitive behavior |
| Per-chat privacy rules | experimental | Risky: requires careful protocol boundaries |
| Per-group privacy rules | experimental | Risky: requires careful protocol boundaries |

## Automation

| Feature | Status | Notes |
| ------- | ------ | ----- |
| Scheduled messages | experimental | Risky: automated sending may conflict with platform expectations |
| Retry missed scheduled sends | experimental | Risky: needs clear local-only behavior |
| Local automation rules | experimental | Risky: must avoid spam or abuse behavior |

## Multiple Accounts

| Feature | Status | Notes |
| ------- | ------ | ----- |
| Multiple WhatsApp accounts | experimental | Risky: may interact with device/account policies |
| Account switcher | experimental | Depends on multi-account architecture |
| Separate local state per account | experimental | Requires account-scoped local databases |
| Unified inbox | experimental | Depends on multi-account support |

## Risky Features

Some planned features may depend on fragile protocol behavior or conflict with WhatsApp platform expectations. These are marked with `Risky:` in the notes column.

Risky areas include:

- Message automation
- Multiple accounts
- Typing/read receipt suppression
- View-once media handling
- Large-scale export/download workflows
- Any feature that changes or hides protocol-level behavior from WhatsApp

Risky does not always mean impossible, but it means the feature needs extra review, clear boundaries, and careful implementation.

## Issue Tracking

Each feature should eventually map to a GitHub issue.

Suggested labels:

- `feature`
- `frontend`
- `backend`
- `desktop`
- `database`
- `experimental`
- `risky`
- `blocked`
- `good first issue`
- `docs`

Suggested issue title format:

```text
Feature: Send text messages
Feature: Add custom chat groups
Feature: Implement full-text search
Feature: Store group participants
Feature: Bulk download media from a chat
```

Suggested issue body:

```md
## Feature

Short description of the feature.

## User Value

Why this matters.

## Scope

- [ ] Frontend
- [ ] Backend
- [ ] Database
- [ ] Tests
- [ ] Documentation

## Acceptance Criteria

- [ ] User can ...
- [ ] State persists after restart
- [ ] Errors are handled clearly

## Risk

Risky: yes/no

Notes:
```
