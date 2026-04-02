# Standing Desk Accountability App

## Concept

A lightweight cross-platform desktop app for keeping a group of people accountable when using standing desks. When one person stands up, everyone knows — and when time is up, everyone gets notified.

## Architecture

**Client:** Electron app (Mac + Windows)  
**Server:** Node.js + WebSocket server, self-hosted on local NAS (alcove)  
**Discovery:** Hardcoded local IP — LAN only, no DNS needed

## How It Works

1. A user opens the app and clicks **"I Stand"**
2. The client sends a `STAND` event to the WebSocket server
3. The server starts an authoritative timer and broadcasts a `STAND_STARTED` event to all connected clients
4. All clients show a native OS notification: *"Misha is standing 🧍"*
5. When the timer expires, the server broadcasts `TIME_UP` to all clients
6. All clients notify their users to sit down (or stand up if they haven't)
7. A user can also click **"I Sit"** early, which broadcasts `STAND_ENDED`

## Message Protocol

```json
// Client → Server
{ "type": "STAND", "user": "Misha", "duration": 2700 }
{ "type": "SIT",   "user": "Misha" }

// Server → All Clients
{ "type": "STAND_STARTED", "user": "Misha", "endsAt": 1234567890 }
{ "type": "STAND_ENDED",   "user": "Misha", "reason": "timer" | "manual" }
{ "type": "TIME_UP",       "user": "Misha" }
{ "type": "STATE_SYNC",    "standings": [{ "user": "Misha", "endsAt": 1234567890 }] }
```

`STATE_SYNC` is sent to a client on reconnect so it catches up with any active sessions.

## Project Structure

```
stand/
├── server/
│   ├── index.js          # Express + ws WebSocket server
│   └── roomManager.js    # Tracks active users & timers
└── client/
    ├── main.js           # Electron main process, tray, notifications
    ├── ws-client.js      # WebSocket connection + reconnect logic
    └── renderer/
        ├── index.html
        └── app.js        # UI logic
```

## Tech Stack

| Layer      | Choice                          |
|------------|---------------------------------|
| Client     | Electron                        |
| Server     | Node.js + `ws` library          |
| Packaging  | `electron-builder` (dmg + exe)  |
| Hosting    | alcove (local NAS, Debian)      |
| Network    | Hardcoded LAN IP                |

## Key Design Decisions

- **Authoritative server timer** — timer lives on the server, not the client, so all users see consistent state even if someone joins mid-session
- **No database** — all state is in-memory; sessions are ephemeral by design
- **Native notifications** — uses Electron's `Notification` API, maps to OS-native alerts on both Mac and Windows
- **Tray icon** — icon reflects current state (standing / sitting) as a passive ambient signal
- **Reconnect-safe** — `STATE_SYNC` on connect means late joiners or reconnectors get current state immediately

## Scope

- ~300–400 lines of code total
- No auth (LAN-only, trust implied)
- No persistence
- Single "room" (everyone shares one session space)
