# Scrunch

Standing desk accountability app. When one person stands, everyone in the group gets notified.

## Components

- **server/** - Go WebSocket server that tracks who's standing
- **client/** - Gio immediate-mode UI client

## Server Setup

### Run locally

```bash
cd server
go build -o scrunch-server .
./scrunch-server
```

Server listens on port 9000 by default. Set `PORT` env var to change.

### Run with Docker

```bash
cd server
docker build -t scrunch-server .
docker run -p 9000:9000 scrunch-server
```

## Client Setup

### Build

```bash
cd client
go build -o scrunch-client .
```

On Windows, add `-ldflags "-H windowsgui"` to suppress the console window:

```bash
go build -ldflags "-H windowsgui" -o scrunch-client.exe .
```

### Run

```bash
./scrunch-client -user YourName
```

### Flags

- `-user` (required) - Your display name
- `-server` - Server address (default: `localhost:9000`)
- `-duration` - Standing duration in seconds (default: 2700 / 45 minutes)

### Example

```bash
./scrunch-client -user Misha -server admins-mac-mini:9000
```

## Protocol

WebSocket messages are JSON.

### Client to Server

```json
{"type": "STAND", "user": "Misha", "duration": 2700}
{"type": "SIT", "user": "Misha"}
```

### Server to Client

```json
{"type": "STATE_SYNC", "standings": [...]}
{"type": "STAND_STARTED", "user": "Misha", "startedAt": 1234567890, "endsAt": 1234570590}
{"type": "STAND_ENDED", "user": "Misha", "reason": "manual"}
{"type": "TIME_UP", "user": "Misha"}
```
