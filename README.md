# SlingBoard (sling)

SlingBoard is a lightweight digital bulletin board solution where you can "sling" content and have it displayed on a common screen.

## Architecture

SlingBoard runs as a NATS-only responder service. The `h8sd` daemon is deployed separately to bridge HTTP/WebSocket traffic into NATS. SlingBoard listens on NATS subjects derived from the HTTP request mapping conventions and publishes user content to `slingboard.{board}`. Each board is backed by a JetStream stream (`sb_{board}`) using interest retention (messages expire after 24h). The frontend uses Datastar to apply server-rendered HTML fragments received over WebSockets.

```mermaid
graph TD
    CLI[CLI] -->|HTTP /api/commands| H8SD[h8sd]
    Browser[Browser] -->|HTTP /, /board/{name}/| H8SD
    Browser -->|WebSocket /board/{name}/| H8SD
    H8SD -->|NATS req/reply| SlingBoard[SlingBoard service]
    SlingBoard -->|JetStream publish| Stream[JetStream sb_{board}]
    Stream -->|Pull consumer| SlingBoard
    SlingBoard -->|HTML fragments| H8SD
```

## Local development

The all-in-one compose setup runs NATS with JetStream enabled, SlingBoard, and h8sd (Go 1.25 toolchain):

```
cd examples/all-in-one-local
docker compose up --build
```

Open the UI via h8sd at `http://localhost:8080/` for the boards list, then navigate to `/board/{name}/` to view a board.

## HTTP to NATS mapping

These are the current subject mappings used by h8sd for `localhost` (Datastar consumes board WebSockets for live updates):

- `GET /` → `h8s.http.get.localhost`
- `GET /board/{name}/` → `h8s.http.get.localhost.board.{name}`
- `POST /api/commands` → `h8s.http.post.localhost.api.commands`
- `GET /static/style.css` → `h8s.http.get.localhost.static.style%2Ecss`
- `WS /board/{name}/` → `h8s.ws.ws.localhost.board.{name}`

If you run `serve` with `--fqdn veggen.mattilsynet.io`, the host segment is reversed so subjects become `h8s.http.get.io.mattilsynet.veggen...`.

All HTTP responses are sent back on the NATS reply subject with `Status-Code`, `Content-Type`, and `Cache-Control: no-cache` headers. WebSocket messages are delivered via JetStream pull consumers and carry raw HTML fragments, which Datastar prepends into `#slings`.

## Markdown

Markdown files (`.md`, `.markdown`) are detected server-side and rendered to HTML before being sent to the browser.

## CLI usage

The CLI sends commands over HTTP (via h8sd):

```
./sling --api-url http://localhost:8080 --board team-a message "hello"
./sling --api-url http://localhost:8080 --board team-a url https://example.com
./sling --api-url http://localhost:8080 --board team-a file ./path/to/file.png
```

Board management commands:

```
./sling --api-url http://localhost:8080 board list
./sling --api-url http://localhost:8080 board create team-a
```

Serve with explicit NATS connection settings and a custom fqdn:

```
./sling serve --nats-url nats://localhost:4222 --nats-creds /path/to/creds --fqdn veggen.mattilsynet.io
```

## Tests

Run the full test suite before committing changes:

```
go test ./...
```
