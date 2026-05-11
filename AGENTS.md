# Fullstack Boilerplate — Agent Guide

> **Last updated:** 2026-05-11 — beforeLoad auth, TS6, code splitting, tests, CSP

## Commands

```bash
# Backend
cd backend && go run ./cmd/server                   # start API on :8080
cd backend && go run ./cmd/server -routes           # print API routes (Markdown) and exit
cd backend && go test ./...                          # 86 tests (service, handler, middleware, config)
cd backend && bash scripts/seed.sh                   # seed admin@boilerplate.com / password123

# Frontend
cd frontend && npm run dev                           # dev server on :5173
cd frontend && npm run build                         # tsc --noEmit + vite build
cd frontend && npm test                              # 40 tests (contexts, hooks, components, routes)
cd frontend && npm run lint                          # ESLint

# Docker
docker compose up -d postgres keydb                  # infra only (run backend locally)
docker compose up -d                                 # full stack
```

## Architecture

**Backend** — chi router, pgx pool (20 max / 5 min conns), Bun ORM, go-playground/validator, KeyDB/Redis.

ID generation uses `crypto/rand` directly. Bcrypt cost 12; missing-user responses use a dummy hash to prevent email enumeration via timing.

Periodic cleanup goroutine in `main.go` purges expired/revoked refresh tokens every 24 hours.

**Frontend** — Vite 8, TypeScript 6.0, TanStack Router (file-based routes with route-level code splitting via `autoCodeSplitting: true`), TanStack Query + Form, Tailwind v4, shadcn/ui, ESLint 10, Vitest.

`@/` path alias maps to `src/`. React-refresh rule allows `Route`, `AuthContext`, `ToastContext`, `LoginPage`, `DashboardPage` exports — update `allowExportNames` in `eslint.config.js` if adding new named exports.

Route pages live in `src/components/pages/` (not in route files) to support code splitting. Route files only export the `Route` object.

## Auth

Auth uses TanStack Router's `beforeLoad` + `throw redirect()` pattern (official TanStack Router recommendation).

- **Dashboard route** (`routes/index.tsx`): `beforeLoad` calls `ensureQueryData(['auth', 'me'])`, redirects to `/login` on 401
- **Login route** (`routes/login.tsx`): `beforeLoad` checks cache only (`getQueryData`) — zero network calls on login page
- **AuthProvider** (`auth-context.tsx`): `useQuery({ enabled: false })` passively reads from cache — never triggers network requests
- **Login flow**: `post('/auth/login')` → `invalidateQueries` → `router.navigate({ to: '/' })` → dashboard's `beforeLoad` verifies auth
- **Logout flow**: `post('/auth/logout')` → `removeQueries` → `router.navigate({ to: '/login' })` → login renders instantly

Dual delivery: same JWT works via **HttpOnly cookie** (web browsers) or **Authorization: Bearer** header (mobile/CLI/third-party). Cookie has priority.

Refresh uses rotating tokens with theft detection: if a refresh token is reused, ALL sessions for that user are revoked.

Rate limiting: Redis sliding window via atomic Lua script — 300 req/min global, 30 req/min on `/auth/*`. Trusts `CF-Connecting-IP` > `X-Real-IP` > `RemoteAddr`. Fail-open on Redis error.

## API

All responses wrapped in envelope: `{"success": true, "data": ...}` or `{"success": false, "error": {"code": "...", "message": "..."}}`.

Auto-generated route docs at `GET /docs/json` (via chi docgen). Only available when server is running.

## Frontend API Client

- `credentials: 'include'` — cookies sent on every request
- Auto-refresh on 401: silently calls `/auth/refresh`, retries once
- GET deduplication: concurrent identical GETs share one network call
- Mutation timeout: POST/PUT/DELETE abort after 30s
- Uses `AbortSignal.any()` with proper type guard

## State & Context

Auth and toast state live in `src/contexts/` (`auth-context.tsx`, `toast-context.tsx`). Hooks in `src/hooks/` (`use-auth.tsx`, `use-toast.tsx`) are thin context consumers. Auth state is populated by route `beforeLoad` guards — AuthProvider never auto-fetches.

## Routes

Route protection via `beforeLoad` guards (not component-level guards):

- **`/` (dashboard)**: protected — `beforeLoad` redirects unauthenticated users to `/login`
- **`/login`**: public — `beforeLoad` redirects authenticated users to dashboard, cache-only check
- **`/login?redirect=/...`**: search param for post-login redirect

SEO meta tags (title, description, Open Graph, Twitter Card) set per-route via TanStack Router `head()` API. Static fallback in `index.html`. Static files: `public/sitemap.xml`, `public/robots.txt`, `public/og-image.webp`.

## Middleware

Ordering: RequestID → RealIP → CleanPath → Recover → Logger → SecurityHeaders → CORS → RateLimiter

- **Recover** — custom JSON panic output (`INTERNAL_ERROR`) instead of chi plain-text. Includes `request_id` in error logs.
- **Logger** — structured JSON via slog with `request_id`, `status`, `duration_ms`, `bytes_written`, optional `user_id`.
- **SecurityHeaders** — factory pattern with OWASP headers (HSTS, X-Frame-Options DENY, etc.).
- **Validate** — returns `*T` instead of `(T, bool)`, uses `toSnakeCase` for field names in validation errors.
- **RateLimiter** — struct-based with `NewRateLimiter()` + `.Middleware()`. Uses atomic Lua script for Redis operations. Configurable `TrustedCIDRs`. Fail-open on Redis error.

## Database

SQL migrations in `backend/migrations/` via Bun migrator — auto-run on startup. Each migration has `.up.sql` + `.down.sql` pair, embedded via `//go:embed`. Tables: `users`, `refresh_tokens`.

PostgreSQL 18+ requires the volume mount at `/var/lib/postgresql` (not `/var/lib/postgresql/data`) for version-specific subdirectory support.

## Pre-commit

Pre-commit hook via Husky + lint-staged at repo root. Runs on every `git commit`:
1. `go vet ./...` — backend code quality
2. `go build ./cmd/server` — backend compilation check
3. `lint-staged` → `eslint --fix` on staged `.ts/.tsx` files
4. `tsc --noEmit` — frontend type safety

Install automatically via `npm install` (prepare script triggers husky). Root `.gitignore` excludes `node_modules/`.

## Docker

**All stages use Docker Hardened Images (DHI)** from `dhi.io`:

| Service  | Build stage                  | Final stage                      |
| -------- | ---------------------------- | -------------------------------- |
| Backend  | `dhi.io/golang:1.26-debian13-dev` | `dhi.io/static:20250419-debian13`  |
| Frontend | `dhi.io/node:24-debian13-dev`     | `dhi.io/nginx:1.30-debian13`       |

Final images run as `nonroot`, have no package manager, and minimal footprint. Build mounts (`--mount=type=cache`) for faster rebuilds. `COPY --link` for better cache reuse. Go binary stripped with `-ldflags="-s -w"`.

`VITE_API_URL` is a build-time ARG (default `http://localhost:8080/api/v1`). No runtime injection needed — build once per environment.

## Tests

**Backend** (86 tests):
- `config_test.go` — config loading, env overrides, JWT_SECRET panic
- `auth_service_test.go` — login, refresh, token reuse detection, logout with mock repos
- `auth_handler_test.go` — HTTP handlers via httptest
- `auth_test.go`, `validate_test.go`, `recover_test.go`, `security_test.go` — middleware tests

**Frontend** (40 tests via Vitest + Testing Library):
- `api.test.ts` — HTTP client, envelope unwrap, 401 retry, timeout, GET dedup
- `auth-context.test.tsx` — login/logout flow, auth state
- `toast-context.test.tsx` — add/remove, auto-dismiss, queue limit
- `use-idle-timeout.test.ts` — timer lifecycle, activity reset, cleanup
- `button.test.tsx`, `input.test.tsx`, `toaster.test.tsx` — component rendering
- `-login.test.tsx`, `-__root.test.tsx` — route integration tests

## Notable

- No Makefile. No `.env.example` (config in README).
- `backend/internal/sanitize/` was removed (unused — JSON API doesn't need HTML sanitization).
- JWT generation uses typed `accessTokenClaims` struct for compile-time safety.
- CSP in nginx.conf is for local dev (`connect-src 'self' http://localhost:*`). Production CSP handled by Cloudflare at edge.
- HSTS left to Cloudflare — not set in nginx.
- PostgreSQL 18+ volume mount at `/var/lib/postgresql` (parent dir, not `/data`).
- Entrypoint.sh was removed — DHI nginx has no shell for `RUN` instructions.
- Rate limit response headers (`X-RateLimit-*`) removed from backend responses — were exposing internal metrics to browsers.
