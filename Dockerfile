# syntax=docker/dockerfile:1
# Web image: Tailwind CSS and the vendored htmx, a static Go binary with the
# assets embedded, and a small non-root runtime. The SQLite file lives in the
# /data volume.

FROM oven/bun:1.4.4-alpine@sha256:4cf82f5686b196504fe119db3fcd9185b191f82ece82c40680e597228ec6070a AS assets
WORKDIR /src
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile
COPY . .
RUN bun run vendor && bun run css

FROM golang:1.27.4-alpine3.24@sha256:0a5f7fe796599ff487605987432e7619ae45beeb2fba4c2ce0b907879932aebf AS build
WORKDIR /src
ENV CGO_ENABLED=0
ARG VERSION=dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=assets /src/resources/public/static resources/public/static
RUN go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/kanban ./cmd/kanban

FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -H -u 10001 app \
	&& mkdir -p /data \
	&& chown app:app /data
WORKDIR /data
COPY --from=build /out/kanban /usr/local/bin/kanban
USER app
# HOST=0.0.0.0 makes the server reachable from outside the container. The
# kanban keeps no sessions, so APP_KEY is optional (a random one is generated
# per process).
ENV HOST=0.0.0.0 PORT=8080 DATABASE_URL=file:/data/kanban.db LOG_LEVEL=info
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
	CMD ["wget", "-qO-", "http://127.0.0.1:8080/healthz"]
ENTRYPOINT ["kanban"]
