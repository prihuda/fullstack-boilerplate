#syntax=docker/dockerfile:1

# ── Build stage ─────────────────────────────────────────────────────
FROM dhi.io/golang:1.26-debian13-dev AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY --link . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

# ── Run stage ────────────────────────────────────────────────────────
FROM dhi.io/static:20250419-glibc-debian13

COPY --from=build /server /server

EXPOSE 8080

ENTRYPOINT ["/server"]
