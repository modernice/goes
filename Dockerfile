# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM node:22.19-alpine AS frontend
RUN corepack enable
WORKDIR /src/ui
COPY ui/package.json ui/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY ui/ ./
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/goes-ui ./cmd/goes-ui

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -S -g 10001 goes && adduser -S -D -H -u 10001 -G goes goes
COPY --from=backend /out/goes-ui /usr/local/bin/goes-ui
COPY --from=frontend /src/ui/.output/public /opt/goes-ui/public

ENV GOES_UI_LISTEN_ADDR=:8080 \
    GOES_UI_ASSETS_DIR=/opt/goes-ui/public
EXPOSE 8080
USER 10001:10001
HEALTHCHECK --interval=20s --timeout=3s --start-period=5s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/goes-ui"]
