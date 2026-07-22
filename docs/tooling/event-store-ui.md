# Event Store UI

A web console for MongoDB and PostgreSQL event stores. Point it at the databases your application writes to and explore what happened — events, aggregates, and payloads — across any number of stores, from one place.

- **Overview** — store health, event and aggregate counts, most recent activity
- **Events** — the full event timeline, filterable by type, aggregate, ID, and time window
- **Streams** — each aggregate's event history, version by version
- **Payloads** — event data decoded and shown as formatted, syntax-highlighted JSON
- **Authentication** — optional username/password login

The console ships as a single Docker image that serves both the web app and its API.

## Quick Start

```bash
docker run --rm -p 8080:8080 \
  -e GOES_UI_STORES='[{"id":"orders","name":"Orders","driver":"postgres","url":"postgres://goes_ui:secret@host:5432/orders"}]' \
  ghcr.io/modernice/goes-ui:latest
```

Open `http://localhost:8080`. With no credentials configured, the console opens right away and shows a persistent banner that authentication is disabled.

`latest` is the most recent release (`goes-ui-v*` tags); `edge` tracks the `main` branch.

::: warning Before production
Anyone who can reach an unauthenticated instance can read your event data. For anything beyond local development, configure `GOES_UI_AUTH_USERNAME`, `GOES_UI_AUTH_PASSWORD`, and `GOES_UI_SESSION_SECRET`, terminate TLS at a reverse proxy, and set `GOES_UI_SECURE_COOKIES=true`.
:::

## Store Configuration

`GOES_UI_STORES` is a JSON array — one entry per store. All configured stores appear in the switcher in the console's header:

```json
[
  {
    "id": "orders",
    "name": "Orders",
    "driver": "postgres",
    "url": "postgres://goes_ui:secret@postgres:5432/orders?sslmode=disable",
    "table": "events"
  },
  {
    "id": "billing",
    "name": "Billing",
    "driver": "mongo",
    "url": "mongodb://goes_ui:secret@mongo:27017/billing",
    "collection": "events"
  }
]
```

| Field | Required | Description |
| --- | --- | --- |
| `id` | yes | Stable, URL-safe identifier, unique per deployment |
| `name` | no | Display name; defaults to `id` |
| `driver` | yes | `postgres` or `mongo` |
| `url` | yes | Connection URL, including credentials if required |
| `database` | no | Overrides the database selected by the URL |
| `table` | PostgreSQL only | Events table; defaults to `events`, may be schema-qualified |
| `collection` | MongoDB only | Events collection; defaults to `events` |

::: warning Database access
The console needs to read events **and create indexes**: on startup it automatically sets up the indexes it uses to browse large stores, and it refuses to start if it can't. Give it dedicated database credentials and restrict network access to the console itself.
:::

## Environment Variables

| Variable | Default | Description |
| --- | --- | --- |
| `GOES_UI_STORES` | — | Store list as JSON (required) |
| `GOES_UI_AUTH_USERNAME` | — | Login username |
| `GOES_UI_AUTH_PASSWORD` | — | Login password |
| `GOES_UI_SESSION_SECRET` | — | Session signing key, at least 32 characters |
| `GOES_UI_SESSION_TTL` | `12h` | How long a login session stays valid |
| `GOES_UI_STREAM_POLL_INTERVAL` | `10s` | How quickly newly created streams appear in the console |
| `GOES_UI_SECURE_COOKIES` | `false` | Marks the session cookie `Secure`; enable behind TLS |
| `GOES_UI_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `GOES_UI_ASSETS_DIR` | set by the image | Location of the built frontend |

Authentication is optional: set username and password to enable it (the session secret is then required too), or leave both empty to disable it. Configuring only one of the two is an error.

Secrets don't have to live in environment variables. The store list, username, password, and session secret each accept a `_FILE` variant (`GOES_UI_STORES_FILE`, …) that reads the value from a mounted file — this is how Docker and Swarm secrets are consumed. When both are set, the file wins.

The server answers health checks on `GET /healthz`; the published image already declares a matching `HEALTHCHECK`.

## Docker Swarm

[`deploy/docker-stack.example.yml`](https://github.com/modernice/goes/blob/main/deploy/docker-stack.example.yml) is a complete stack file that reads the store list, password, and session secret from Swarm secrets:

```yaml
services:
  goes-ui:
    image: ghcr.io/modernice/goes-ui:latest
    environment:
      GOES_UI_AUTH_USERNAME: developer
      GOES_UI_STORES_FILE: /run/secrets/goes_ui_stores
      GOES_UI_AUTH_PASSWORD_FILE: /run/secrets/goes_ui_password
      GOES_UI_SESSION_SECRET_FILE: /run/secrets/goes_ui_session_secret
    secrets:
      - goes_ui_stores
      - goes_ui_password
      - goes_ui_session_secret
```

[`deploy/stores.example.json`](https://github.com/modernice/goes/blob/main/deploy/stores.example.json) shows the matching store-list secret.

## Custom Builds

The stock image fits most setups. There are two reasons to build your own binary instead: your events need your application's codec to decode, or you want to serve the console from a Go server you already run. Both go through the public `eventstoreui` package.

### Decoding Custom Event Types

By default, payloads are decoded as plain JSON. If your application registers events with custom `codec` behavior, reuse the same registry in a small entrypoint of your own:

```go
package main

import (
	"log"

	"github.com/modernice/goes/event"
	"github.com/modernice/goes/eventstoreui"
	"my.company/orders/events"
)

func main() {
	registry := event.NewRegistry()
	events.Register(registry)

	if err := eventstoreui.Main(eventstoreui.WithEncoding(registry)); err != nil {
		log.Fatal(err)
	}
}
```

An event that isn't registered, or fails to decode, is flagged individually — the rest of the page still renders.

To ship the custom binary, start from the published image and swap in your executable. The frontend and all configuration carry over:

```dockerfile
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/goes-ui ./cmd/goes-ui

FROM ghcr.io/modernice/goes-ui:latest
COPY --from=build /out/goes-ui /usr/local/bin/goes-ui
```

### Embedding in an Existing Server

`eventstoreui.New` returns an app that serves the API and frontend from a single `http.Handler`, so the console can be mounted into a server you already run:

```go
cfg, _ := eventstoreui.LoadConfig()
app, err := eventstoreui.New(cfg, eventstoreui.WithEncoding(registry))
if err != nil {
	log.Fatal(err)
}
defer app.Close()

http.ListenAndServe(":8080", app.Handler())
```

## Local Development

From the repository root, `make goes-ui` builds the frontend once and runs the Go server on `:8080`. For frontend hot reload, keep that server running and start the Nuxt dev server in `ui/`:

```bash
cd ui && pnpm dev
```

Nuxt proxies `/api` to `http://127.0.0.1:8080`; override the target with `GOES_UI_API_PROXY` if the API runs elsewhere.
