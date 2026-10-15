# One image with the Go server and the built frontend.
#   docker build -t x-project .
# Run with MySQL 8: see compose.yaml and docs/dev.md.

FROM node:24-alpine AS web
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm check && pnpm build

FROM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/xproject ./cmd/xproject

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 xproject
COPY --from=server /out/xproject /usr/local/bin/xproject
COPY --from=web /src/web/dist /srv/web
ENV XP_LISTEN=:8080 \
    XP_WEB_DIR=/srv/web
USER xproject
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=120s CMD wget -qO- http://127.0.0.1:8080/api/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/xproject"]
