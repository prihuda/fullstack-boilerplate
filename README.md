# Fullstack Boilerplate

Full-stack Go + React boilerplate with PostgreSQL, Redis/KeyDB, Docker Hardened Images (DHI) — ready to extend.

## Stack

| Layer | Technology |
|-------|------------|
| Backend | Go 1.26, chi v5, pgx v5, Bun ORM, PostgreSQL 18, KeyDB/Redis |
| Frontend | React 19, TypeScript 6.0, Vite 8, TanStack (Router + Query + Form), Tailwind v4, shadcn/ui, Vitest, ESLint 10 |
| Deployment | Docker Hardened Images (DHI), Docker Compose, Nginx, Cloudflare (edge) |

## Quick Start

### Prerequisites

- Go 1.26+
- Node 24+ (LTS)
- Docker + Docker Compose v2.22+ (for PostgreSQL + Redis)

### 1. Clone and start infrastructure

```bash
docker compose up -d postgres keydb
```

### 2. Backend

```bash
cd backend
export JWT_SECRET=your-secret-key-change-me
export DATABASE_URL=postgres://app_user:devpassword@localhost:5432/boilerplate?sslmode=disable
go mod tidy
go run ./cmd/server
```

Server starts on `:8080`. Health check: `curl http://localhost:8080/api/v1/health`

View auto-generated API docs: `curl http://localhost:8080/docs/json`

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

Creates admin user: `admin@boilerplate.com` / `password123`

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
    logger.go                     — structured JSON request logging via slog (with request_id)
    ratelimit.go                  — Redis sliding window rate limiter (atomic Lua script)
    recover.go                    — custom JSON panic handler (INTERNAL_ERROR)
    security.go                   — OWASP security headers (HSTS 2y, X-Frame-DENY, etc.)
    validate.go                   — generic JSON request validation (returns *T, toSnakeCase)
  model/
    user.go                       — User, RefreshToken structs
    response.go                   — APIResponse, ErrorResponse, TokenResponse, LoginRequest
  repository/
    user_repo.go                  — bun.IDB-based user CRUD
    refresh_token_repo.go         — token rotation + cleanup in PostgreSQL transaction
  service/
    auth_service.go               — Login (bcrypt + JWT), Refresh (rotation with reuse detection), Logout, GetUser
pkg/database/
  postgres.go                     — pgx pool with Bun ORM wrapper
migrations/                       — SQL migration files (users + refresh_tokens)
scripts/
  seed.sh                         — seed admin user
```

### API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | /api/v1/auth/login | No | Login — sets cookies + returns tokens |
| POST | /api/v1/auth/refresh | Token* | Rotate refresh token (cookie or body) |
| POST | /api/v1/auth/logout | Token* | Revoke refresh token (cookie or body) |
| GET | /api/v1/auth/me | JWT | Get current user |
| GET | /api/v1/health | No | DB + Redis health check |
| GET | /docs/json | No | Auto-generated route documentation (JSON) |

_* Token can be sent as HttpOnly cookie (web) or `refresh_token` in JSON body (API clients)._

**Login** returns tokens in both HttpOnly cookies (browsers) and JSON body (API clients).
**Refresh** uses rotating tokens with reuse detection — if a stolen token is reused, all sessions are revoked.

### Configuration

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| SERVER_PORT | 8080 | No | HTTP listen port |
| DATABASE_URL | — | Yes | PostgreSQL connection string |
| JWT_SECRET | — | **Yes** | JWT signing key (min 32 chars) |
| CORS_ALLOWED_ORIGINS | http://localhost:3000 | No | Comma-separated allowed origins |
| LOG_LEVEL | info | No | debug, info, warn, error |
| COOKIE_SECURE | true | No | Set Secure flag on cookies (false for local HTTP dev) |
| TOKEN_TYPE | Bearer | No | Token type field in login/refresh response body |
| KEYDB_ADDR | localhost:6379 | No | Redis/KeyDB address |

### Key Backend Features

- **Token rotation with reuse detection** — if a refresh token is used twice (stolen + legitimate), all sessions are invalidated
- **Periodic cleanup** — goroutine purges expired/revoked tokens every 24 hours
- **Rate limiting** — Redis sliding window with atomic Lua script (300 req/min global, 30 req/min on auth routes)
- **Fail-open on Redis error** — rate limit bypassed if Redis is down
- **IP detection** — respects `CF-Connecting-IP` (Cloudflare), `X-Real-IP` (nginx), falls back to `RemoteAddr`
- **Custom JSON panic recovery** — returns consistent `INTERNAL_ERROR` JSON instead of plain text
- **Structured JSON logging** — every log entry includes `request_id`, `status`, `duration_ms`
- **Stripped binary** — Docker build uses `-ldflags="-s -w"` for ~30% smaller images
- **Connection pooling** — pgx with MaxConns=20, MinConns=5
- **Timing-safe auth** — dummy bcrypt hash prevents email enumeration via response timing
- **Auto-generated API docs** — `GET /docs/json` returns full route tree via chi docgen

### Running Tests

```bash
cd backend
go test ./...                    # 86 tests
go test -v ./internal/service/   # verbose, single package
```

---

## Frontend

### Architecture

```
src/
  App.tsx                        — QueryClient, AuthProvider, Router, Toast provider
  main.tsx                       — React 19 StrictMode mount
  index.css                      — Tailwind v4 @theme tokens (shadcn/ui colors)
  routeTree.gen.ts               — auto-generated by TanStack Router plugin
  types/
    auth.ts                      — User, TokenResponse, LoginRequest, ApiError
  lib/
    api.ts                       — typed fetch client with GET dedup, auto-refresh, mutation timeout
    constants.ts                 — API_BASE_URL from env
    utils.ts                     — cn() helper (clsx + tailwind-merge)
  contexts/
    auth-context.tsx             — AuthContext, AuthProvider (passive cache subscriber via useQuery)
    toast-context.tsx            — ToastContext, ToastProvider (auto-dismiss notifications)
  hooks/
    use-auth.tsx                 — useAuth hook
    use-toast.tsx                — useToast hook
    use-idle-timeout.ts          — 30-min auto-logout on inactivity
  components/
    layout/
      query-error-boundary.tsx   — Error boundary with retry
    pages/
      login-page.tsx             — Email + password form with TanStack Form
      dashboard-page.tsx         — Dashboard with user info and logout
    ui/
      button.tsx                 — shadcn/ui CVA button (6 variants + loading state)
      card.tsx                   — Card layout components
      input.tsx                  — Styled input
      label.tsx                  — Form label
    toaster.tsx                  — Toast notification display (auto-positioned)
  routes/
    __root.tsx                   — Root layout, HeadContent, Outlet (no auth guard)
    index.tsx                    — Dashboard route with beforeLoad auth guard
    login.tsx                    — Login route with cache-only beforeLoad check
```

### Key Frontend Features

- **Route code splitting** — `autoCodeSplitting: true`, pages lazy-loaded per route
- **Link preloading** — `defaultPreload: 'intent'` prefetches route data on hover
- **Auth via beforeLoad** — TanStack Router `beforeLoad` + `throw redirect()` pattern
- **Zero API calls on login page** — AuthProvider uses `useQuery({ enabled: false })`, auth state populated by route guards
- **SEO via TanStack Router + static fallback** — OG tags, Twitter Card, canonical, theme-color, noscript in both `index.html` and per-route `head()`. Plus `sitemap.xml` and `robots.txt`.
- **shadcn/ui components** — button, card, input, label with CVA variants
- **Auto-refresh on 401** — API client silently rotates tokens and retries once
- **GET deduplication** — concurrent requests to the same URL share one network call
- **Mutation timeout** — POST/PUT/DELETE auto-abort after 30 seconds
- **Idle timeout** — auto-logout after 30 minutes of inactivity
- **TypeScript 6.0** — strict mode, exact types throughout

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| VITE_API_URL | http://localhost:8080/api/v1 | Backend API base URL |

### Running Tests

```bash
cd frontend
npm test                         # 40 tests (Vitest)
npm test -- --watch              # watch mode
npm test -- --coverage           # with coverage report
```

---

## Docker

### Development

```bash
# Start all services
docker compose up -d

# Or start only infrastructure (run backend/frontend locally)
docker compose up -d postgres keydb
```

### Production Build

```bash
docker compose build
docker compose up -d
```

All stages use **Docker Hardened Images (DHI)** from `dhi.io` — CIS-hardened, minimal footprint, non-root runtime.

| Service | Build image | Runtime image |
|---------|-------------|---------------|
| Backend | `dhi.io/golang:1.26-debian13-dev` | `dhi.io/static:20250419-debian13` |
| Frontend | `dhi.io/node:24-debian13-dev` | `dhi.io/nginx:1.30-debian13` |

The compose file includes:
- **PostgreSQL** with persistent volume (mount at `/var/lib/postgresql` for PG 18+ support) and health check
- **KeyDB** (multi-threaded Redis drop-in) with LRU eviction (256MB max)
- **Backend** — Go API hardened image (cache mounts, COPY --link, stripped binary, non-root)
- **Frontend** — Nginx hardened image serving SPA (VITE_API_URL as build-time ARG)

### CSP & HSTS

CSP in `nginx.conf` is relaxed for local development (`connect-src 'self' http://localhost:*`).
In production, Cloudflare handles HSTS and strict CSP at the edge — no nginx changes needed.

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
    public/              Static assets (sitemap, robots, favicon, OG image)
    Dockerfile           DHI nginx-served production build
  docker-compose.yml     Full-stack development + production
```

---

## Extending

### Adding a new page

1. Create the page component in `src/components/pages/your-page.tsx`
2. Create `src/routes/your-page.tsx` — import and use the page component, set `beforeLoad` if protected
3. Add `head()` for SEO meta tags
4. TanStack Router auto-generates the route tree; route is lazy-loaded automatically

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
