# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/embed.go ./web/embed.go
COPY --from=frontend /src/web/dist ./web/dist
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/trove ./cmd/trove

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 10001 trove \
    && adduser -D -H -u 10001 -G trove trove \
    && mkdir -p /data /media \
    && chown trove:trove /data /media
COPY --from=backend /out/trove /usr/local/bin/trove
LABEL org.opencontainers.image.title="Trove" \
      org.opencontainers.image.description="Self-hosted media library browser and downloads" \
      org.opencontainers.image.source="https://github.com/ncecere/trove"
ENV TROVE_ADDR=:8080 TROVE_DATA=/data
WORKDIR /data
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/api/v1/setup/status || exit 1
ENTRYPOINT ["/usr/local/bin/trove"]
