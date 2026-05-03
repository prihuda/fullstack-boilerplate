# Fullstack Boilerplate

Full-stack Go + React boilerplate with PostgreSQL, Redis/KeyDB, Docker — ready to extend.

## Stack

| Layer       | Technology                                                                     |
| ----------- | ------------------------------------------------------------------------------ |
| Backend     | Go 1.26, chi v5, pgx v5, Bun ORM, PostgreSQL 18, KeyDB/Redis                  |
| Frontend    | React 19, TypeScript 5.9, Vite 8, TanStack (Router + Query + Form), Tailwind v4, ESLint 10 |
| Deployment  | Docker, Docker Compose, Nginx                                                  |

## Quick Start

### Prerequisites

- Go 1.26+
- Node 24+
- Docker + Docker Compose v2.22+ (for PostgreSQL + Redis)

### 1. Clone and start infrastructure

```bash
docker compose up -d postgres keydb
```

### 2. Backend

```bash
cd backend
# Set env vars (see Configuration section below)
export JWT_SECRET=your-secret-key-change-me
export DATABASE_URL=postgres://app_user:devpassword@localhost:5432/boilerplate?sslmode=disable
go mod tidy
go run ./cmd/server
```

Server starts on `:8080`. Health check: `curl http://localhost:8080/api/v1/health`

### 3. Frontend

```bash
cd frontend
npm install
npm run dev
```

Opens at `http://localhost:5173` with hot reload.

### 4. Seed database (optional)

```bash
cd backend
bash scripts/seed.sh
```

Creates an admin user: `admin@boilerplate.com` / `password123`

---

## Backend

### Architecture

```
cmd/server/main.go               — entry point, wiring, graceful shutdown
internal/
  config/config.go                — env-based configuration
  handler/
    auth_handler.go               — POST /login, /refresh, /logout, GET /me
    health_handler.go             — GET /health (DB + Redis ping)
  middleware/
    auth.go                       — JWT verification from cookie or Authorization header
    logger.go                     — structured request logging via slog
    ratelimit.go                  — Redis sliding window rate limiter (300 req/min)
    security.go                   — OWASP security headers (HSTS, CSP, XSS, etc.)
    validate.go                   — generic JSON request validation
  model/
    user.go                       — User, RefreshToken structs
    response.go                   — APIResponse, ErrorResponse, TokenResponse, LoginRequest
  repository/
    user_repo.go                  — bun.IDB-based user CRUD
    refresh_token_repo.go         — token rotation + cleanup in PostgreSQL transaction
  sanitize/
    sanitize.go                   — input sanitization helpers
  service/
    auth_service.go               — Login (bcrypt + JWT), Refresh (rotation with reuse detection), Logout, GetUser
pkg/database/
  postgres.go                     — pgx pool with Bun ORM wrapper
migrations/                       — SQL migration files (users + refresh_tokens)
scripts/
  seed.sh                         — seed admin user
```

### API Endpoints

| Method | Path                     | Auth     | Description                          |
| ------ | ------------------------ | -------- | ------------------------------------ |
| POST   | /api/v1/auth/login       | No       | Login — sets cookies + returns tokens|
| POST   | /api/v1/auth/refresh     | Token*   | Rotate refresh token (cookie or body)|
| POST   | /api/v1/auth/logout      | Token*   | Revoke refresh token (cookie or body)|
| GET    | /api/v1/auth/me          | JWT      | Get current user                     |
| GET    | /api/v1/health           | No       | DB + Redis health check              |

_* Token can be sent as HttpOnly cookie (web) or `refresh_token` in JSON body (API clients)._

**Login** returns tokens in both HttpOnly cookies (browsers) and JSON body (API clients).  
**Refresh** uses rotating tokens with reuse detection — if a stolen token is reused, all sessions are revoked.

### Configuration

| Variable             | Default                  | Required | Description                        |
| -------------------- | ------------------------ | -------- | ---------------------------------- |
| SERVER_PORT          | 8080                     | No       | HTTP listen port                   |
| DATABASE_URL         | —                        | Yes      | PostgreSQL connection string       |
| JWT_SECRET           | —                        | **Yes**  | JWT signing key (min 32 chars)     |
| CORS_ALLOWED_ORIGINS | http://localhost:3000    | No       | Comma-separated allowed origins   |
| LOG_LEVEL            | info                     | No       | debug, info, warn, error           |
| COOKIE_SECURE        | true                     | No       | Set Secure flag on cookies (false for local HTTP dev) |
| TOKEN_TYPE           | Bearer                   | No       | Token type field in login/refresh response body       |
| TOKEN_TYPE            | Bearer                   | No       | Token type returned in auth responses |
| KEYDB_ADDR            | localhost:6379           | No       | Redis/KeyDB address                |

### Key Backend Features

- **Token rotation with reuse detection** — if a refresh token is used twice (stolen + legitimate), all sessions are invalidated
- **Periodic cleanup** — goroutine purges expired/revoked tokens every 24 hours
- **Rate limiting** — Redis sliding window (300 req/min global, stricter per-auth-route)
- **IP detection** — respects `CF-Connecting-IP` (Cloudflare), `X-Real-IP` (nginx), falls back to `RemoteAddr`
- **Stripped binary** — Docker build uses `-ldflags="-s -w"` for ~30% smaller images
- **Connection pooling** — pgx with MaxConns=20, MinConns=5

### Running Tests

```bash
cd backend
go test ./...                    # all tests
go test ./internal/service/...   # single package
```

---

## Frontend

### Architecture

```
src/
  App.tsx                        — QueryClient, Router, Toast provider
  main.tsx                       — React 19 StrictMode mount
  index.css                      — Tailwind v4 @theme tokens
  routeTree.gen.ts               — auto-generated by TanStack Router plugin
  types/
    auth.ts                      — User, TokenResponse, LoginRequest, ApiError
  lib/
    api.ts                       — typed fetch client with GET dedup, auto-refresh, mutation timeout
    constants.ts                 — API_BASE_URL from env
    utils.ts                     — cn() helper (clsx + tailwind-merge)
  contexts/
    auth-context.tsx             — AuthContext, AuthProvider (login/logout/checkAuth)
    toast-context.tsx            — ToastContext, ToastProvider (auto-dismiss notifications)
  hooks/
    use-auth.tsx                 — useAuth hook
    use-toast.tsx                — useToast hook
    use-idle-timeout.ts          — 30-min auto-logout on inactivity
  components/
    layout/
      query-error-boundary.tsx   — Error boundary with retry
    ui/
      button.tsx                 — CVA button (6 variants + loading state)
      card.tsx                   — Card layout components
      input.tsx                  — Styled input with error state
      label.tsx                  — Form label
    toaster.tsx                  — Toast notification display (auto-positioned)
  routes/
    __root.tsx                   — Auth guard, AuthProvider wrapper, idle timeout, SEO head
    login.tsx                    — Email + password form with TanStack Form
    index.tsx                    — Dashboard with user info and logout
```

### Key Frontend Features

- **SEO via TanStack Router** — meta tags (title, description) set per-route via `head()` API instead of `index.html`
- **Auto-refresh on 401** — API client silently rotates tokens and retries once
- **GET deduplication** — concurrent requests to the same URL share one network call
- **Mutation timeout** — POST/PUT/DELETE auto-abort after 30 seconds
- **Idle timeout** — auto-logout after 30 minutes of inactivity
- **Indonesian locale** — default `lang="id"` with localized descriptions

### Configuration

| Variable       | Default                    | Description                   |
| -------------- | -------------------------- | ----------------------------- |
| VITE_API_URL   | http://localhost:8080/api/v1 | Backend API base URL        |

---

## Docker

### Development

```bash
# Start all services
docker compose up -d

# Or start only infrastructure (run backend/frontend locally)
docker compose up -d postgres keydb
```

### Production

```bash
# Build and run everything
docker compose up -d
```

The compose file includes:
- **PostgreSQL** with persistent volume and health check
- **KeyDB** (multi-threaded Redis drop-in) with LRU eviction (256MB max)
- **Backend** — Go API with optimized Dockerfile (cache mounts, COPY --link, stripped binary)
- **Frontend** — Nginx serving SPA with multi-stage build (npm cache, COPY --link)

---

## Pre-commit Checks

A Git pre-commit hook via Husky + lint-staged runs automatically on every `git commit`:

| Step | Check | What it catches |
|------|-------|-----------------|
| 1 | `go vet ./...` | Go code quality issues |
| 2 | `go build ./cmd/server` | Compilation errors |
| 3 | `eslint --fix` on staged `.ts/.tsx` | Lint errors |
| 4 | `tsc --noEmit` | TypeScript type errors |

Hook fails the commit if any step fails. Installed automatically via `npm install` at the repo root.

---

## Project Structure

```
root/
  backend/               Go API server
    cmd/server/main.go   Entry point
    internal/            App logic (handlers, middleware, services, repos)
    migrations/          SQL migrations
    pkg/database/        DB connection helpers
    scripts/             Utility scripts
  frontend/              React SPA
    src/                 App source
    Dockerfile           Nginx-served production build
  docker-compose.yml     Full-stack development + production
```

---

## Extending

### Adding a new page

1. Create `src/routes/items.tsx` with `createFileRoute('/items')`
2. Add `head()` for SEO meta tags
3. TanStack Router auto-generates the route tree via `routeTree.gen.ts`
4. Export the route component for Fast Refresh support

### Adding a new API endpoint

1. Add handler in `backend/internal/handler/`
2. Register route in `cmd/server/main.go`
3. Add repository method if needed
4. Add frontend API call via `get()` / `post()` / `put()` / `del()` from `src/lib/api.ts`

### Adding a new database table

1. Create migration in `backend/migrations/`
2. Add model struct in `backend/internal/model/`
3. Add repository in `backend/internal/repository/`

---

## License

MIT
