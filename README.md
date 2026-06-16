# ChatApp

A **production-ready** real-time chat application built with Go, featuring WebSocket connections, JWT authentication, room management, and PostgreSQL persistence.

## Architecture

```
chatapp/
├── cmd/
│   └── main.go                 # Entry point, DI, graceful shutdown
├── internal/
│   ├── domain/
│   │   ├── models.go           # Entities: User, Room, Message, DTOs
│   │   └── repository.go       # Repository interfaces
│   ├── database/
│   │   └── postgres.go         # DB connection & auto-migration
│   ├── repository/
│   │   ├── user_repository.go  # GORM implementations
│   │   ├── room_repository.go  # incl. DM room queries
│   │   └── message_repository.go
│   ├── service/
│   │   ├── auth_service.go     # Business logic: auth
│   │   ├── room_service.go     # Business logic: rooms
│   │   ├── message_service.go  # Business logic: messages
│   │   └── dm_service.go        # Direct message logic
│   ├── handler/
│   │   ├── handlers.go         # HTTP handlers (REST API)
│   │   └── dm_handler.go       # DM endpoints
│   ├── middleware/
│   │   ├── auth.go             # JWT middleware (header + query token)
│   │   ├── rate_limiter.go     # Token-bucket rate limiting
│   │   ├── logger.go           # Request ID, request logger, recovery
│   │   └── validation.go       # Input validation helpers
│   ├── logger/
│   │   └── logger.go           # zerolog setup
│   ├── websocket/
│   │   ├── hub.go              # Connection manager & broadcaster
│   │   └── handler.go          # WS upgrade & event routing
│   └── router/
│       └── router.go           # Route definitions
├── static/
│   └── index.html              # Interactive WebSocket test client
├── test/
│   ├── mock/                   # testify mocks for repositories
│   └── service/                # unit tests (auth, room, message, DM)
├── .github/workflows/ci.yml    # GitHub Actions CI
├── docker-compose.yml
├── Dockerfile
└── Makefile
```

## Tech Stack

| Layer         | Technology               |
|---------------|--------------------------|
| Framework     | Gin                      |
| WebSocket     | gorilla/websocket        |
| ORM           | GORM                     |
| Database      | PostgreSQL               |
| Auth          | JWT (golang-jwt/jwt v5)  |
| Password      | bcrypt                   |
| UUID          | google/uuid              |

## Quick Start

### 1. Clone & setup environment
```bash
cp .env.example .env
# Edit .env with your DB credentials

# Download dependencies (first time only)
go mod tidy
```

### 2. Start PostgreSQL (Docker)
```bash
make docker-up
```

### 3. Run the app
```bash
make run
```

Server starts at `http://localhost:8080`

---

## REST API Endpoints

### Auth
| Method | Endpoint              | Description       | Auth |
|--------|-----------------------|-------------------|------|
| POST   | /api/v1/auth/register | Register new user |    |
| POST   | /api/v1/auth/login    | Login             |    |
| GET    | /api/v1/auth/me       | Get my profile    |    |

### Rooms
| Method | Endpoint                    | Description          | Auth |
|--------|-----------------------------|----------------------|------|
| GET    | /api/v1/rooms               | List public rooms    |    |
| POST   | /api/v1/rooms               | Create room          |    |
| GET    | /api/v1/rooms/me            | My joined rooms      |    |
| GET    | /api/v1/rooms/:id           | Get room detail      |    |
| DELETE | /api/v1/rooms/:id           | Delete room          |    |
| POST   | /api/v1/rooms/:id/join      | Join room            |    |
| POST   | /api/v1/rooms/:id/leave     | Leave room           |    |
| GET    | /api/v1/rooms/:id/messages | Get messages      |    |

### Messages
| Method | Endpoint              | Description    | Auth |
|--------|-----------------------|----------------|------|
| PATCH  | /api/v1/messages/:id  | Edit message   |    |
| DELETE | /api/v1/messages/:id  | Delete message |    |

---

## WebSocket Protocol

### Connect
```
ws://localhost:8080/api/v1/ws
Headers: Authorization: Bearer <jwt_token>
```

### Events (Client → Server)

**Join a room:**
```json
{ "type": "join_room", "payload": { "room_id": "uuid" } }
```

**Send a message:**
```json
{
  "type": "send_message",
  "payload": {
    "room_id": "uuid",
    "content": "Hello world!",
    "reply_to_id": "uuid (optional)"
  }
}
```

**Typing indicator:**
```json
{ "type": "typing", "payload": { "room_id": "uuid" } }
{ "type": "stop_typing", "payload": { "room_id": "uuid" } }
```

**Leave room:**
```json
{ "type": "leave_room", "payload": { "room_id": "uuid" } }
```

**Ping/heartbeat:**
```json
{ "type": "ping" }
```

### Events (Server → Client)

| Event Type      | Description                        |
|-----------------|------------------------------------|
| `new_message`   | New message in room                |
| `edit_message`  | Message was edited                 |
| `delete_message`| Message was deleted                |
| `user_joined`   | User joined the room               |
| `user_left`     | User left the room                 |
| `user_online`   | User came online                   |
| `user_offline`  | User went offline                  |
| `typing`        | User is typing                     |
| `stop_typing`   | User stopped typing                |
| `error`         | Error occurred                     |
| `pong`          | Heartbeat response                 |

---

## Example API Usage

### Register
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"budi","email":"budi@example.com","password":"secret123"}'
```

### Login
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"budi@example.com","password":"secret123"}'
```

### Create Room
```bash
curl -X POST http://localhost:8080/api/v1/rooms \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"General","description":"Main chat room","type":"public"}'
```

### Get Messages
```bash
curl http://localhost:8080/api/v1/rooms/<room_id>/messages?page=1&limit=50 \
  -H "Authorization: Bearer <token>"
```

---

## Key Features

- **Real-time messaging** via WebSocket with gorilla/websocket
- **Room-based chat** public, private, direct message rooms
- **Direct Messages (DM)** 1-on-1 conversations, auto-created room per pair
- **JWT Authentication** stateless auth, 7-day token expiry
- **Online presence** track who's online in real-time
- **Typing indicators** broadcast typing status per room
- **Message history** paginated REST API for past messages
- **Edit & soft-delete** messages deleted content preserved in DB
- **Reply to messages** threaded conversation support
- **Rate limiting** token-bucket per IP, stricter on auth endpoints
- **Structured logging** zerolog, pretty console (dev) / JSON (prod)
- **Graceful shutdown** SIGINT/SIGTERM handling with 10s drain
- **Unit tests** service layer fully covered with mocked repositories
- **WebSocket test client** interactive HTML client at `/static/index.html`
- **Clean Architecture** domain → repository → service → handler layers
- **Docker ready** full docker-compose setup included
- **CI pipeline** GitHub Actions: vet, test, build, format check

---

## Testing

```bash
make test          # run all unit tests
go test ./... -v -cover   # with coverage
```

Tests live in `test/service/*_test.go` and use `testify/mock` to mock
repository interfaces no database needed to run them.

---

## WebSocket Test Client

A ready-to-use browser client is included at:

```
http://localhost:8080/static/index.html
```

It lets you:
1. Register / login to get a JWT
2. Connect via WebSocket (token passed as `?token=` query param)
3. Join a room, send messages, see typing indicators & presence events live

---

## Rate Limiting

| Endpoint group | Limit          |
|-----------------|----------------|
| `/api/v1/auth/*` | 10 requests/min per IP |
| All other `/api/v1/*` | 120 requests/min per IP |

Exceeding the limit returns `429 Too Many Requests`.

---

## Direct Messages

| Method | Endpoint    | Description                          | Auth |
|--------|-------------|---------------------------------------|------|
| GET    | /api/v1/dm  | List my DM conversations              |    |
| POST   | /api/v1/dm  | Get or create DM room with a user `{"recipient_id":"uuid"}` |    |

Once you have the DM room's ID, use it like any other room for
WebSocket messaging and message history (`/api/v1/rooms/:id/messages`).
