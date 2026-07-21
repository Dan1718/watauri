# WaTauri

## Overview 

WaTauri is an experimental, local-first desktop Whatsapp Client built with Tauri. 

It connects to WhatsApp Web, syncs chats and messages into a local SQLite database, and exposes them through a desktop UI. The project is open source and currently under active development.


## Status 

This project is early-stage and experimental. 

Core chat/message syncing is being built out, but the app is not ready yet. 
## Features 

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
- 
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

- Send messages from the frontend
- Message pagination
- Read receipts
- Typing indicators
- Search endpoint
- Contact sync improvements
- Group participant persistence
- Group metadata support
- Profile/current user endpoint
- Safer local API exposure
- Better error handling and API responses
- More backend tests
 
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
