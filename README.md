# WaTauri

## Overview 

WaTauri is an experimental, local-first desktop Whatsapp Client built with Tauri. 

It connects to WhatsApp Web, syncs chats and messages into a local SQLite database, and exposes them through a desktop UI. The project is open source and currently under active development.

## Why this exists 

I have an ick for the official WhatsApp Web experience. 

I'm basically trying to explore a desktop-first experience, with stronger organization, more power user features like, better search, better bulk actions, improved media workflows, and more control in general. 

Obviously, this should do everything that WhatsApp Web currently does (yet to be achieved), but faster and more reliable and then some. 

## Status 

This project is early-stage and experimental. 

Core chat/message syncing is being built out, but the app is not ready yet. 
## Features 
Look at [FEATURES.md](./FEATURES.md) for everything. 
- [x] WhatsApp QR pairing
- [x] Local SQLite message storage
- [x] Chat list from local data
- [x] Message history view
- [x] History sync persistence
- [x] Contact storage basics
- [x] Sending messages
- [x] Message pagination
- [ ] Real-time updates with Server-Sent Events
- [x] Contact name sync
- [ ] Group metadata
- [ ] Group participants
- [ ] Contact and group avatars
- [ ] Communities and related group metadata
- [ ] Pinned chats
- [ ] Pinned messages
- [ ] Favourite chats and contacts
- [ ] Archive workflows
- [ ] Safer local API routing
- [ ] Custom chat groups and filters
- [ ] Fast full-text search
- [ ] Bulk message/media actions
- [ ] Better media download/export
- [ ] Voice message transcription
- [ ] Desktop notifications
- [ ] Keyboard shortcuts
- [ ] Message scheduling
- [ ] Multiple accounts
- [ ] Privacy controls for typing/read signals

## Screenshots

None yet. 

## Tech Stack 

Desktop: Tauri   
Frontend: Next.js, React   
Backend: Go   
Database: SQLite   
Whatsapp Client: whatsmeow   
Package manager: bun   

## Requirements 

- Go   
- Bun  
- Rust and Cargo  
- Tauri CLI dependencies  

## Getting started 

Clone the repo 

```bash
git clone https://github.com/your-username/whatsapp-tauri.git
cd whatsapp-tauri/whatsapp-tauri
``` 

Install the dependencies: 
```bash
bun install
```

Run the app in development: 

```bash
bun run tauri dev
``` 

Other useful scripts: `bun run build` (typecheck + build frontend), `bun run preview` (preview the production build).

Compiling the backend seperately: 

```bash
cd src-go && go build -o backend .
```

This has been changed to now run automatically when tauri is called, however it does not recompile on every save.

## Usage 

1. Start the app with `bun run tauri dev`. 
2. Open the desktop window. 
3. Pair your whatsapp account using the QR Code. 
4. Voila, (it'll take some time to load all the messages)

Runtime data is stored locally using SQLite. 

## Development 

This project has three main parts: 
- Next.js frontend 
- Tauri Desktopo shell 
- Go backend using whatsmeow 

More info will be added later. 


## Project Structure 
``` 
whatsapp-tauri/
  app/                 Next.js frontend
  public/              Static assets
  src-go/              Go backend and WhatsApp client
  src-tauri/           Tauri desktop shell
  docs/                Project notes and API docs
``` 

## Roadmap 

### Phase 1: Reliable Core Client

- [x] WhatsApp pairing
- [x] Local message persistence
- [x] Historical chat sync
- [x] Sending messages
- [ ] Contacts, group metadata, participants, and avatars
- [x] Message pagination
- [ ] Real-time updates with Server-Sent Events
- [ ] Read receipts

### Phase 2: Better Desktop Client

- [ ] Desktop notifications
- [ ] Keyboard shortcuts
- [ ] Fast startup from local cache
- [ ] Better window/session restore
- [ ] Attachment handling
- [ ] Media previews

### Phase 3: Power-User Organization

- [ ] Custom chat groups
- [ ] Saved filters
- [ ] Per-group notification rules
- [ ] Archive workflows
- [ ] Pinned chats
- [ ] Pinned messages
- [ ] Favourite chats and contacts
- [ ] Starred message workflows
- [ ] Multi-select chats/messages

### Phase 4: Search and Export

- [ ] Full-text message search
- [ ] Search by sender/date/media type
- [ ] Voice note transcription
- [ ] Bulk media download
- [ ] Chat/message export

### Phase 5: Experimental Features

- [ ] Scheduled messages
- [ ] Multiple accounts
- [ ] Privacy controls
- [ ] One-time media UX

## Security 

This app runs a local backend and stores WhatsApp session data locally.
Do not commit:
*.db
*.db-wal
*.db-shm
wa-session.db
userdata.db
The backend should only be exposed locally. Do not run it on a public interface unless you understand the security implications.

## Contributing 

Contributions are welcome. I know that it's not properly organized yet, but I plan on using issues with a proper roadmap later on after the initial stage of development. 

## Disclaimer. 

There is a fair bit of AI generated code and documentation. 
This project is not affiliated with WhatsApp, Meta, or any official WhatsApp product.
It uses WhatsApp Web behavior through whatsmeow. Use at your own risk.
