# SlingBoard (sling)

SlingBoard is a lightweight digital bulletin board solution where you can "sling" content and have it displayed on a common screen.

## Architecture

SlingBoard runs as a NATS-only responder service. The `h8sd` daemon is deployed separately to bridge HTTP/WebSocket traffic into NATS. SlingBoard listens on NATS subjects derived from the HTTP request mapping conventions and publishes user content to `slingboard.global`.

## Local development

The all-in-one compose setup runs NATS, SlingBoard, and h8sd (Go 1.25 toolchain):

```
cd examples/all-in-one-local
docker compose up --build
```

Open the UI via h8sd at `http://localhost:8080/`.

## HTTP to NATS mapping

These are the current subject mappings used by h8sd for `localhost`:

- `GET /` → `h8s.http.GET.localhost`
- `POST /api/commands` → `h8s.http.POST.localhost.api.commands`
- `GET /static/style.css` → `h8s.http.GET.localhost.static.style%2Ecss`
- `WS /slings` → `h8s.ws.ws.localhost.slings`

All HTTP responses are sent back on the NATS reply subject with `Status-Code`, `Content-Type`, and `Cache-Control: no-cache` headers.

## CLI usage

The CLI sends commands over HTTP (via h8sd):

```
./sling --api-url http://localhost:8080 message "hello"
./sling --api-url http://localhost:8080 url https://example.com
./sling --api-url http://localhost:8080 file ./path/to/file.png
```
