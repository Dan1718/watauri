# WhatsApp Tauri

## Compiling the backend seperately

```bash
cd src-go && go build -o backend .
```

This has been changed to now run automatically when tauri is called, however it does not recompile on every save.

## Running the dev environment

Prerequisites: [Bun](https://bun.sh), the [Rust toolchain](https://www.rust-lang.org/tools/install), and the [Tauri system dependencies](https://tauri.app/start/prerequisites/) for your OS.

```bash
# install dependencies
bun install

# run the full desktop app (Tauri shell + Vite frontend)
bun run tauri dev

# or run just the web frontend in the browser
bun run dev
```

Other useful scripts: `bun run build` (typecheck + build frontend), `bun run preview` (preview the production build).

## Tauri + Preact + Typescript

This template should help get you started developing with Tauri, Preact and Typescript in Vite.

## Recommended IDE Setup

- [VS Code](https://code.visualstudio.com/) + [Tauri](https://marketplace.visualstudio.com/items?itemName=tauri-apps.tauri-vscode) + [rust-analyzer](https://marketplace.visualstudio.com/items?itemName=rust-lang.rust-analyzer)
