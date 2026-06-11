# Reverse Engineering WhatsApp Web

This repo starts with observation, not implementation. The first goal is to build a map of how the web client is structured before trying to clone any behavior.

## Initial targets

1. Capture the boot sequence.
2. Identify static assets and chunk naming patterns.
3. Inspect service worker registration and cache behavior.
4. Record storage primitives: localStorage, sessionStorage, IndexedDB.
5. Observe network classes: HTML, JS chunks, media, websocket, XHR/fetch.
6. Separate app shell concerns from protocol concerns.

## Working assumptions

1. The app is a bundled SPA with aggressive code splitting.
2. Authentication and message sync depend on a persistent browser-side state layer.
3. Some behavior is UI-only and can be reproduced from DOM/state observation.
4. Some behavior is protocol-specific and should be treated as a separate subsystem.

## Reverse engineering workflow

1. Run the inspector script to collect a baseline snapshot.
2. Save output after key states:
   - fresh load
   - QR login screen
   - authenticated shell
   - chat open
3. Diff snapshots to learn which assets, stores, and connections change by state.
4. Build a feature inventory:
   - app bootstrap
   - auth/session
   - chat list
   - message timeline
   - composer
   - attachments
   - presence/typing
   - notifications
5. Rebuild in layers:
   - shell and layout
   - local state model
   - transport abstraction
   - chat primitives

## Things to record manually in DevTools

1. DOM landmarks for main panes and overlays.
2. React root containers and hydration behavior, if exposed.
3. IndexedDB database names and object stores.
4. WebSocket endpoints and upgrade headers.
5. Service worker script URL and cache names.
6. CSS variables, theme tokens, spacing, and breakpoint behavior.

## Deliverables for this phase

1. A repeatable asset/network snapshot.
2. A state/storage inventory.
3. A UI anatomy map.
4. A transport/protocol boundary hypothesis.

## Notes

- Reproducing the interface is a different task from reproducing WhatsApp's private backend protocol.
- Treat protocol work as optional and isolated. We can still build a high-fidelity client UI and state model without it.
