# Event Store UI

A standalone web console for MongoDB and PostgreSQL event stores. Browse the event timeline, inspect aggregate streams, and view decoded payloads across any number of named stores from a single container.

- **Overview** — store health, event and aggregate counts, most recent activity
- **Events** — cursor-paginated timeline with type, aggregate, ID, and time-window filters
- **Streams** — an in-memory catalog built from version-1 events, with event data loaded only when a stream is opened
- **Payloads** — server-side decoding with formatted, syntax-highlighted JSON
- **Authentication** — optional static username/password with signed, HTTP-only sessions

::: warning Database access
The UI requires full access to every configured event store. On startup it
creates and verifies indexes used by the stream catalog. Use dedicated
credentials and restrict network access to the UI.
:::

## Quick Start

Images are published to GitHub Container Registry — `latest` for `goes-ui-v*` release tags, `edge` for the current `main` branch:

```bash
docker run --rm -p 8080:8080 \
  -e GOES_UI_STORES='[{"id":"orders","name":"Orders","driver":"postgres","url":"postgres://goes_ui:secret@host:5432/orders"}]' \
  ghcr.io/modernice/goes-ui:latest
```

Open `http://localhost:8080`. With no username and password configured, the
console opens directly and displays a persistent warning that authentication is
disabled.

::: warning Production safety
Only disable authentication for trusted local development. Anyone who can reach
an unsecured instance can inspect its event data. For production, configure all
three authentication variables, terminate TLS at a reverse proxy, and set
`GOES_UI_SECURE_COOKIES=true`.
:::

## Store Configuration

`GOES_UI_STORES` holds a JSON array. Each entry connects one store, and all configured stores appear in the switcher in the console's header:

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

## Database Setup

The UI automatically creates the indexes it needs to browse large event stores
efficiently. No manual database setup is required, but the configured database
user must have full access. The UI will fail to start if it cannot create a
required index.

## Runtime Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `GOES_UI_STORES` | — | Store list as JSON (required) |
| `GOES_UI_AUTH_USERNAME` | — | Login username; leave both credentials empty to disable authentication |
| `GOES_UI_AUTH_PASSWORD` | — | Login password; leave both credentials empty to disable authentication |
| `GOES_UI_SESSION_SECRET` | — | Session signing key, at least 32 characters and required when authentication is enabled |
| `GOES_UI_SESSION_TTL` | `12h` | Session lifetime, as a Go duration |
| `GOES_UI_STREAM_POLL_INTERVAL` | `10s` | Interval for discovering newly created aggregate streams |
| `GOES_UI_SECURE_COOKIES` | `false` | Adds the `Secure` flag to the session cookie |
| `GOES_UI_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `GOES_UI_ASSETS_DIR` | set by the image | Directory with the built frontend assets |

The username and password must either both be configured or both be empty. The
store list, username, password, and session secret also accept a `_FILE` variant
(e.g. `GOES_UI_STORES_FILE`) that reads the value from a mounted file — the
natural fit for Docker and Swarm secrets. When both are set, `_FILE` wins.

The server exposes `GET /healthz` for container health checks; the published image already declares a `HEALTHCHECK` against it.

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

## Custom Event Codecs

The stock binary decodes payloads as generic JSON. If your application registers events with custom `codec.Unmarshaler` behavior, build a small entrypoint that injects your own `codec.Encoding` through the public `eventstoreui` package:

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

The decoded value is normalized through `encoding/json` before it reaches the browser. An unregistered event name or a failed decode is reported on that event alone — the rest of the page still renders.

To ship the custom binary, reuse the published image and replace only the executable; the frontend assets and runtime configuration carry over:

```dockerfile
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/goes-ui ./cmd/goes-ui

FROM ghcr.io/modernice/goes-ui:latest
COPY --from=build /out/goes-ui /usr/local/bin/goes-ui
```

## Embedding

`eventstoreui.New` returns an app whose `http.Handler` serves both the API and the frontend, so the console can also be mounted inside an existing server:

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
