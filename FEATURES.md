# WhatsApp Client Remake

> **AI-generated code disclaimer**: The frontend (React components, styles, mock data) and most documentation were initially generated with AI assistance. Architecture decisions, protocol integration, and backend implementation are human-directed. All generated code is reviewed and modified by a human.

A desktop-first, power-user WhatsApp client focused on better organization, bulk actions, automation, and privacy controls.

This project is intended to explore a non-browser WhatsApp client experience with stronger UX than the official client, especially for people managing a lot of chats, media, and accounts.

## Goals

- Make chat organization dramatically better than the default WhatsApp client.
- Support workflows the official client makes difficult or impossible.
- Build a fast, reliable desktop experience.
- Keep risky and protocol-sensitive behavior clearly separated from normal client features.

## Feature List

### Better Grouping

Create custom chat groups and filters with their own behavior.

Examples:

- User-defined chat groups
- Saved filters
- Per-group notification priority
- Per-group ping behavior
- Mute or badge-only modes
- Rules for how different categories of chats behave on desktop

Tag: `safe`

### Actions on Multiple Items

Support bulk actions across messages, media, and possibly chats.

Examples:

- Multi-select messages
- Multi-select media
- Bulk forward
- Bulk share/export
- Bulk download
- Bulk archive/delete where supported

Tag: `safe`

### One-Time Media

Handle view-once images, videos, and similar temporary media more cleanly in the client UX.

Examples:

- Clear identification of view-once content
- Better viewing flow
- Better metadata and state tracking

Tag: `risky`
Reason: handling or preserving one-time media beyond intended platform behavior may violate WhatsApp TOS or expected product behavior.

### Multiple Accounts

Allow multiple WhatsApp accounts in one desktop client.

Examples:

- Fast switching between accounts
- Separate local state per account
- Unified inbox or split mode

Tag: `risky`
Reason: technically feasible, but multi-account behavior may interact with WhatsApp account/device policies in ways that increase product risk.

### Scheduling Messages

Compose a message now and send it later at a specific time.

Examples:

- Local scheduled send when this client is online
- Optional relay through another trusted device
- Optional relay through a phone or server when the current device is offline
- Retry and failure handling for missed schedules

Tag: `risky`
Reason: automated or deferred sending may violate WhatsApp TOS depending on how it is implemented, especially when routing through background services or remote relays.

### Transcribing Voice Messages

Generate transcripts for voice notes.

Examples:

- Local transcription
- Searchable transcript text
- Export with transcript attached

Tag: `safe`

### Better Downloading

Make it easier to download and export everything the client can access.

Examples:

- Download messages and media in bulk
- Better organization of downloaded files
- Export attachments from a chat or filter
- Better handling of media retention and re-download where possible

Tag: `risky`
Reason: aggressive downloading, bulk export, or retention of content may create TOS, privacy, or data-handling risk depending on implementation.

### Disable Typing and View Status

Provide privacy controls around outgoing behavioral signals.

Examples:

- Suppress typing indicators
- Suppress read receipts
- Control presence-like behavior where possible
- Per-chat or per-group privacy rules

Tag: `risky`
Reason: intentionally suppressing protocol-level status signals may conflict with WhatsApp platform expectations or TOS.

### Better Search

Make chats, messages, media, and transcripts easier to search.

Examples:

- Fast full-text search
- Search across filters/groups/accounts
- Search by media type, sender, date, transcript, or attachment

Tag: `safe`

### Desktop Behavior

Make the app feel like a real desktop client instead of a thin wrapper.

Examples:

- Keyboard shortcuts
- Fast startup
- Reliable window restore behavior
- Consistent open state
- Better desktop notifications

Tag: `safe`

## Risk Tags

- `safe`: mostly client-side UX or local data features with low obvious platform risk.
- `risky`: may depend on reverse-engineered protocol behavior, may be fragile, or may violate WhatsApp TOS.

## Risk Notes

The following categories should be treated carefully:

- Message automation
- Remote relay or proxy sending
- Read receipt or typing suppression
- View-once media handling beyond official behavior
- Large-scale export or download workflows
- Any feature that changes or hides protocol-level behavior from WhatsApp

These features may be technically possible while still being operationally fragile or in conflict with WhatsApp's Terms of Service.

## Product Direction

This project is aiming for a power-user client with:

- Better organization
- Better desktop UX
- Better media workflows
- Better privacy controls
- Better automation

The safest product path is to keep the architecture split between:

- core client UX features
- protocol-sensitive features
- explicitly risky features

## Current Priorities

- Define the protocol/core architecture
- Build the desktop-first client shell
- Implement grouping, search, and bulk actions first
- Isolate risky features behind clear boundaries
