# ChatApp

A real-time chat application built with Go. Supports WebSocket messaging, JWT authentication, room management, direct messages, and PostgreSQL persistence.

---

## Screenshots

The following screenshots highlight the main features of **ChatApp**, including authentication, real-time messaging, direct messaging, and user profile management.

### Login

![Login](docs/screenshots/login.jpg)

The login page provides a secure authentication system using JWT. Users can sign in using their registered email and password. After successful authentication, a JWT access token is generated and used to authorize subsequent REST API and WebSocket requests. This page also serves as the entry point for new users through the registration feature.

---

### Dashboard

![Dashboard](docs/screenshots/dashboard.png)

The dashboard is the primary interface for public and private room conversations. Users can browse available chat rooms, join discussions, send and receive messages in real time via WebSocket, view message history, monitor online user presence, and see typing indicators. The interface updates instantly without requiring page refreshes.

---

### Direct Message Dashboard

![Direct Messages](docs/screenshots/dashboarddm.png)

The Direct Message (DM) dashboard enables private one-on-one conversations between users. A dedicated DM room is automatically created for each unique user pair, ensuring messages remain private. Users can exchange messages in real time, view conversation history, reply to previous messages, and receive typing status updates similar to public chat rooms.

---

### Settings

![Settings](docs/screenshots/settings.png)

The settings page allows users to manage their account preferences and personal information. From this page, users can update their profile details, modify account settings, review authentication information, and configure application preferences. This centralized management interface improves usability while keeping account-related operations organized and secure.
---

## Project Structure

```
chatapp/
├── cmd/
│   └── main.go                    # Entry point, dependency injection, graceful shutdown
├── internal/
│   ├── domain/
│   │   ├── models.go              # Entities: User, Room, Message, DTOs
│   │   └── repository.go          # Repository interfaces
│   ├── database/
│   │   └── postgres.go            # DB connection and auto-migration
│   ├── repository/
│   │   ├── user_repository.go     # GORM implementations
│   │   ├── room_repository.go     # Includes DM room queries
│   │   └── message_repository.go
│   ├── service/
│   │   ├── auth_service.go        # Auth business logic
│   │   ├── room_service.go        # Room business logic
│   │   ├── message_service.go     # Message business logic
│   │   └── dm_service.go          # Direct message logic
│   ├── handler/
│   │   ├── handlers.go            # HTTP handlers (REST API)
│   │   └── dm_handler.go          # DM endpoints
│   ├── middleware/
│   │   ├── auth.go                # JWT middleware (header + query token)
│   │   ├── rate_limiter.go        # Token-bucket rate limiting
│   │   ├── logger.go              # Request ID, request logger, recovery
│   │   └── validation.go          # Input validation helpers
│   ├── logger/
│   │   └── logger.go              # zerolog setup
│   ├── websocket/
│   │   ├── hub.go                 # Connection manager and broadcaster
│   │   └── handler.go             # WebSocket upgrade and event routing
│   └── router/
│       └── router.go              # Route definitions
├── static/
│   └── index.html                 # Browser WebSocket client
├── test/
│   ├── mock/                      # testify mocks for repositories
│   └── service/                   # Unit tests: auth, room, message, DM
├── docker-compose.yml
├── Dockerfile
└── Makefile
```

---

## Tech Stack

| Layer       | Technology              |
|-------------|-------------------------|
| Framework   | Gin                     |
| WebSocket   | gorilla/websocket       |
| ORM         | GORM                    |
| Database    | PostgreSQL              |
| Auth        | JWT (golang-jwt/jwt v5) |
| Password    | bcrypt                  |
| UUID        | google/uuid             |
| Logging     | zerolog                 |

---

## Quick Start

**1. Clone and configure environment**
```bash
cp .env.example .env
# Fill in your DB credentials and JWT secret in .env
go mod tidy
```

**2. Start PostgreSQL**
```bash
make docker-up
```

**3. Run the server**
```bash
make run
```

Server runs at `http://localhost:8080`.

---

## REST API

### Auth

| Method | Endpoint                | Description        | Auth |
|--------|-------------------------|--------------------|------|
| POST   | /api/v1/auth/register   | Register new user  | No   |
| POST   | /api/v1/auth/login      | Login              | No   |
| GET    | /api/v1/auth/me         | Get current user   | Yes  |
| POST   | /api/v1/auth/logout     | Logout (revoke token) | Yes  |

### Rooms

| Method | Endpoint                       | Description          | Auth |
|--------|--------------------------------|----------------------|------|
| GET    | /api/v1/rooms                  | List public rooms    | Yes  |
| POST   | /api/v1/rooms                  | Create room          | Yes  |
| GET    | /api/v1/rooms/me               | My joined rooms      | Yes  |
| GET    | /api/v1/rooms/:id              | Get room detail      | Yes  |
| DELETE | /api/v1/rooms/:id              | Delete room          | Yes  |
| POST   | /api/v1/rooms/:id/join         | Join room            | Yes  |
| POST   | /api/v1/rooms/:id/leave        | Leave room           | Yes  |
| GET    | /api/v1/rooms/:id/messages     | Get message history  | Yes  |

### Messages

| Method | Endpoint               | Description    | Auth |
|--------|------------------------|----------------|------|
| PATCH  | /api/v1/messages/:id   | Edit message   | Yes  |
| DELETE | /api/v1/messages/:id   | Delete message | Yes  |

### Direct Messages

| Method | Endpoint    | Description                                          | Auth |
|--------|-------------|------------------------------------------------------|------|
| GET    | /api/v1/dm  | List my DM conversations                             | Yes  |
| POST   | /api/v1/dm  | Get or create a DM room `{"recipient_id":"uuid"}`   | Yes  |

Once you have the DM room ID, use it like any other room for WebSocket messaging and message history.

---

## WebSocket Protocol

**Connect:**
```
ws://localhost:8080/api/v1/ws
```

Token authentication is done **after** the upgrade, not in the URL. Passing a token as a query parameter (`?token=...`) would expose it in server logs, browser history, and Referer headers.

**Two supported auth flows:**

Option A — Authorization header (non-browser / API clients):
```
ws://localhost:8080/api/v1/ws
Header: Authorization: Bearer <jwt_token>
```

Option B — First-message auth (browser clients):
```
1. Connect without any token
2. Server immediately sends: {"type":"auth_required"}
3. Client replies:           {"type":"auth","payload":{"token":"<jwt>"}}
4. Server validates and proceeds normally
```
If the first message is not a valid auth event, the connection is closed.

**Client to server events:**

Authenticate (only needed in browser flow — see above):
```json
{ "type": "auth", "payload": { "token": "<jwt>" } }
```

Join a room:
```json
{ "type": "join_room", "payload": { "room_id": "uuid" } }
```

Send a message:
```json
{
  "type": "send_message",
  "payload": {
    "room_id": "uuid",
    "content": "Hello!",
    "reply_to_id": "uuid (optional)"
  }
}
```

Typing indicator:
```json
{ "type": "typing", "payload": { "room_id": "uuid" } }
{ "type": "stop_typing", "payload": { "room_id": "uuid" } }
```

Leave room:
```json
{ "type": "leave_room", "payload": { "room_id": "uuid" } }
```

Ping / heartbeat:
```json
{ "type": "ping" }
```

**Server to client events:**

| Event           | Description                    |
|-----------------|--------------------------------|
| new_message     | New message in room            |
| edit_message    | Message was edited             |
| delete_message  | Message was deleted            |
| user_joined     | User joined the room           |
| user_left       | User left the room             |
| user_online     | User came online               |
| user_offline    | User went offline              |
| typing          | User is typing                 |
| stop_typing     | User stopped typing            |
| messages_read   | Partner read your messages     |
| error           | Error occurred                 |
| pong            | Heartbeat response             |

---

## Edit and Delete Rules

Messages can only be edited or deleted if **both** conditions are met:

1. The message was sent less than 3 minutes ago.
2. The recipient has not yet read it (no blue double-tick).

If either condition fails, the edit and delete options are hidden from the context menu. This is enforced on the frontend. The 3-minute window applies regardless of read status.

---

## Example API Usage

Register:
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"budi","email":"budi@example.com","password":"secret123"}'
```

Login:
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"budi@example.com","password":"secret123"}'
```

Create a room:
```bash
curl -X POST http://localhost:8080/api/v1/rooms \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"General","description":"Main chat room","type":"public"}'
```

Get messages:
```bash
curl http://localhost:8080/api/v1/rooms/<room_id>/messages?page=1&limit=50 \
  -H "Authorization: Bearer <token>"
```

---

## Features

- Real-time messaging via WebSocket
- Room-based chat: public, private, and direct message rooms
- Direct Messages (DM): 1-on-1 conversations, auto-created room per pair
- JWT authentication with 7-day token expiry
- Online presence tracking in real-time
- Typing indicators broadcast per room
- Paginated message history via REST API
- Edit and soft-delete messages (rules above)
- Reply to messages for threaded conversations
- Token-bucket rate limiting per IP
- File upload validated by extension AND magic-bytes MIME type (prevents disguised uploads)
- WebSocket authentication via first-message handshake (token never exposed in URL or logs)
- JWT revocation via token blacklist — logout immediately invalidates the token
- CORS and WebSocket origins restricted to `ALLOWED_ORIGINS` env variable (no allow-all)
- Edit and delete restricted to current room members (ex-members cannot modify old messages)
- Structured logging with zerolog (pretty in dev, JSON in prod)
- Graceful shutdown on SIGINT/SIGTERM with 10s drain
- Unit tests for the service layer using mocked repositories
- Browser WebSocket client included at `/static/index.html`
- Clean architecture: domain > repository > service > handler
- Docker Compose setup included

---

## Rate Limiting

| Endpoint group          | Limit                   |
|-------------------------|-------------------------|
| /api/v1/auth/*          | 10 requests/min per IP  |
| All other /api/v1/*     | 120 requests/min per IP |

Exceeding the limit returns `429 Too Many Requests`.

---

## Testing

```bash
make test
# or with coverage
go test ./... -v -cover
```

Tests live in `test/service/` and use `testify/mock` to mock repository interfaces. No database is needed to run them.

---

## Browser Client

A ready-to-use browser client is served at:

```
http://localhost:8080/static/index.html
```

It lets you register or login, connect via WebSocket, join rooms, send messages, and see typing indicators and presence events live.
