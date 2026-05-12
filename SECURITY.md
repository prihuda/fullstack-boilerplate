# Security

Security features, decisions, and production considerations for the Fullstack Boilerplate.

---

## Authentication

### Password Storage

- **Bcrypt cost 12** (~400ms per comparison) — high enough to resist brute-force, low enough for acceptable latency
- **Timing-safe missing-user detection** — when an email is not found, a dummy bcrypt hash is still compared, preventing email enumeration via response-time side-channel (cost is matched to real hashes)

### JWT

- **Typed claims** — `accessTokenClaims` struct with compile-time safety (not `MapClaims`)
- **Dual delivery** — JWT sent as **HttpOnly cookie** (browsers) AND in response body (mobile/CLI clients)
- **Cookie has priority** — checked before `Authorization: Bearer` header
- **Missing `iss`/`aud`** — intentionally omitted; add via `RegisteredClaims.Issuer/Audience` if running multiple services

### Refresh Token Rotation

- **Rotating tokens** — every refresh call issues a new refresh token and invalidates the old one
- **Theft detection** — if a stolen refresh token is reused after legitimate rotation, ALL sessions for that user are revoked
- **Database-backed** — tokens stored in `refresh_tokens` table with `token_hash`, `replaced_by`, `expires_at`
- **Periodic cleanup** — goroutine purges expired/revoked tokens every 24 hours (graceful shutdown via `ctx.Done()`)
- **Transactional rotation** — UPDATE + INSERT wrapped in a Bun transaction for atomicity (no session loss on crash)

### Token Validation

- **Access token** — short-lived JWT (15 minutes), validated via `jwt.ParseWithClaims` with HMAC-SHA256
- **Refresh token** — opaque hash stored in DB, validated by lookup + expiry check
- **Auto-refresh** — frontend API client silently retries on 401, refreshing the access token via `/auth/refresh`

---

## Rate Limiting

| Scope | Limit | Implementation |
|-------|-------|----------------|
| Global | 300 req/min | Redis sorted set via atomic Lua script |
| Auth routes | 30 req/min | Separate Redis key prefix (`ratelimit-auth:`) |

- **Atomic Lua script** — check-and-add is atomic (no TOCTOU race under concurrency)
- **Fail-open** — if Redis is unreachable, request passes through (logged as error)
- **IP detection** — respects `CF-Connecting-IP` (Cloudflare) > `X-Real-IP` (nginx) > `RemoteAddr`
- **Trusted CIDRs** — private/Docker ranges (127.0.0.1/8, 10.0.0.0/8, 172.16.0.0/12, etc.) bypass rate limiting
- **No rate limit headers** — `X-RateLimit-*` headers are not sent to clients (was exposing internal metrics)

---

## HTTP Security Headers

### Backend (Go middleware)

| Header | Value | Notes |
|--------|-------|-------|
| `X-Content-Type-Options` | `nosniff` | Prevents MIME type sniffing |
| `X-Frame-Options` | `DENY` | Precludes clickjacking |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Controls referrer leakage |
| `Permissions-Policy` | `geolocation=(), microphone=(), camera=()` | Restricts browser API access |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains; preload` | 2-year HSTS with preload |
| `Cache-Control` | `no-store, no-cache, must-revalidate` | Disables caching for API responses |

### Frontend (nginx — local dev only)

| Header | Value | Notes |
|--------|-------|-------|
| `Content-Security-Policy` | `default-src 'self'; script-src 'self' 'unsafe-inline'; ...` | Relaxed for local dev; see CSP section |
| All backend headers above | Same values | Applied by nginx for SPA resources |

### Production

- **HSTS** — handled by Cloudflare at the edge (not duplicated in nginx)
- **CSP** — strict CSP enforced by Cloudflare; local nginx CSP is intentionally permissive
- **TLS** — terminated at Cloudflare; backend never handles raw TLS

---

## Content Security Policy (CSP)

### Local Development

```
default-src 'self';
script-src  'self' 'unsafe-inline';
style-src   'self' 'unsafe-inline';
img-src     'self' data:;
font-src    'self';
connect-src 'self' http://localhost:* http://127.0.0.1:*;
```

- `'unsafe-inline'` in `script-src` is required by TanStack Router's route code-splitting (lazy-loaded route chunks are injected as inline scripts)
- `connect-src` allows all localhost ports — add new services without editing nginx.conf
- **This is not production-grade.** Cloudflare should enforce a strict CSP in production.

### Production (Cloudflare)

Recommended CSP for production:

```
default-src 'none';
script-src 'self';
style-src 'self' 'unsafe-inline';
img-src 'self' data:;
font-src 'self';
connect-src 'self' https://api.yourdomain.com;
frame-ancestors 'none';
base-uri 'none';
form-action 'self';
```

Set via Cloudflare Transform Rules or Workers. See [Cloudflare CSP docs](https://developers.cloudflare.com/rules/transform/response-header-modification/).

---

## Docker Hardening

- **DHI images** — Docker Hardened Images from `dhi.io`: CIS-hardened, minimal footprint
- **No shell** — `dhi.io/static` runtime has no shell, no package manager, no utilities (not even `wget`/`curl`)
- **Non-root** — final containers run as `nonroot` user (no privilege escalation path)
- **Stripped binary** — Go binary compiled with `-ldflags="-s -w"` to remove symbol tables
- **COPY --link** — all COPY instructions use `--link` for better layer cache reuse
- **No entrypoint.sh** — DHI nginx has no shell; `VITE_API_URL` is a build-time ARG, not runtime-injected

---

## Input Validation

- **go-playground/validator** — struct tags for required fields, email format, length constraints
- **toSnakeCase** — validation error field names are converted to snake_case (e.g., `account_id`)
- **MaxBytesReader** — request body limited to 4KB for auth endpoints (prevents memory exhaustion)
- **Parameterized queries** — Bun ORM uses parameterized SQL throughout (no SQL injection vector)
- **Bcrypt truncation** — passwords over 72 bytes are silently truncated by bcrypt; add SHA-256 pre-hashing if longer passwords are needed

---

## CORS

- **Credentials allowed** — `Access-Control-Allow-Credentials: true` for HttpOnly cookie auth
- **Methods** — GET, POST, PUT, DELETE, OPTIONS
- **Headers** — Content-Type, Authorization
- **Max age** — 300s (5 min) preflight cache
- **Always registered** — CORS middleware is unconditional (not guarded by `if origins > 0`)
- **Default origins** — `http://localhost:3000,http://localhost:5173` (set via docker-compose)

---

## Dependency Management

- **Go module** — direct deps audited at update time; `go.sum` provides integrity verification
- **npm** — lockfile (`package-lock.json`) committed; `npm ci` used in Docker builds (deterministic installs)
- **No known CVEs** — pgx/v5 latest version addresses GO-2024-2567, GO-2024-2606, GO-2026-4771, GO-2026-4772
- **Minimal dependencies** — no ORM wrappers beyond Bun, no HTTP client libraries beyond chi, no utility libraries beyond what's needed

---

## Assumptions & Trade-offs

| Decision | Rationale | When to re-evaluate |
|----------|-----------|---------------------|
| CSP `'unsafe-inline'` for scripts | TanStack Router lazy-loading requires inline scripts | Use nonce-based CSP if security requirements tighten |
| HSTS delegated to Cloudflare | Cloudflare handles TLS edge, nginx doesn't need it | If deploying without Cloudflare, enable HSTS in nginx.conf |
| Rate limit headers removed | Exposed internal metrics to browsers | Add back if client-side rate limit awareness is needed |
| Missing `iss`/`aud` JWT claims | Not needed for single-service deployment | Add when running multiple services with shared JWT |
| Bcrypt cost 12 | Balance of security and latency (~400ms) | Increase cost as hardware improves |
| No server-side CSP nonce generation | Static nginx serving prebuilt SPA | Use a backend that generates nonces if CSP strictness is critical |

---

## npm Supply Chain

### 2026-05-11: @tanstack/* Compromise

On 2026-05-11, 42 `@tanstack/*` packages had 84 malicious versions published via a GitHub Actions cache poisoning attack combined with OIDC token extraction. Malware harvested credentials and exfiltrated them over an encrypted messenger network.

**Our status:** Not affected. `package-lock.json` resolved to versions predating the malicious range. See [GHSA-g7cv-rxg3-hmpx](https://github.com/TanStack/router/security/advisories/GHSA-g7cv-rxg3-hmpx).

### Mitigation: `min-release-age`

The `.npmrc` file configures:

```ini
min-release-age = 7d
```

This prevents npm from installing any package version published less than 7 days ago. Combined with `package-lock.json` (which pins exact versions), this provides defense-in-depth against "publish and immediately poison" supply chain attacks.

If you need to install a fresh package (e.g., a security patch published today), override temporarily:

```bash
npm install --min-release-age=0 <package>
```

---

## Reporting Issues

For security vulnerabilities, open a GitHub Issue or contact the repository maintainer directly. Do not disclose vulnerabilities publicly until they are resolved.
