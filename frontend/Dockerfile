#syntax=docker/dockerfile:1

# ── Build stage ─────────────────────────────────────────────────────
FROM dhi.io/node:24-debian13-dev AS build

ARG VITE_API_URL=http://localhost:8080/api/v1
ENV VITE_API_URL=${VITE_API_URL}

WORKDIR /app

COPY package.json package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --prefer-offline

COPY --link . .
RUN npm run build

# ── Run stage ────────────────────────────────────────────────────────
FROM dhi.io/nginx:1.30-debian13

COPY --link --from=build /app/dist /usr/share/nginx/html
COPY --link nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
