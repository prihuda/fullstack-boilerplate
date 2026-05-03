FROM node:24-trixie AS build

# Placeholder — replaced at runtime by entrypoint.sh via env_file
ARG VITE_API_URL=__VITE_API_URL__
ENV VITE_API_URL=${VITE_API_URL}

WORKDIR /app

COPY package.json package-lock.json* ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --prefer-offline

COPY --link . .
RUN npm run build

FROM debian:trixie-slim

RUN apt-get update && apt-get install -y --no-install-recommends nginx curl && rm -rf /var/lib/apt/lists/*

COPY --link --from=build /app/dist /usr/share/nginx/html
COPY --link nginx.conf /etc/nginx/conf.d/default.conf
COPY --link entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

EXPOSE 80

HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
  CMD curl -f http://localhost || exit 1

ENTRYPOINT ["/entrypoint.sh"]
