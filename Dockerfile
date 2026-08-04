# syntax=docker/dockerfile:1
# Frao Advisor — single binary serving either the MCP server or the standalone
# dashboard (the subcommand picks the role). This image defaults to the
# dashboard; run `frao-advisor` with no args for MCP mode.

# --- Build stage: static Go binary (no CGO) ---
FROM golang:1.26.5-alpine AS build
WORKDIR /src

# Cache module downloads (go.mod/go.sum change less often than source).
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/frao-advisor .

# --- Runtime stage: minimal alpine, non-root ---
# APP_UID must match the host user that owns ./data (the bind-mounted SQLite
# dir) so the container can write advisor.db. Set per-host via compose build arg.
FROM alpine:3.21
ARG APP_UID=1000
RUN apk add --no-cache ca-certificates \
    && addgroup -g "$APP_UID" advisor \
    && adduser  -D -u "$APP_UID" -G advisor advisor

COPY --from=build /out/frao-advisor /usr/local/bin/frao-advisor

RUN mkdir -p /data && chown advisor:advisor /data

USER advisor
EXPOSE 9753

ENTRYPOINT ["/usr/local/bin/frao-advisor"]
CMD ["dashboard"]
