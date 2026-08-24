# api-go — CLI & API quick reference

This folder contains the small Go API server and a tiny CLI helper to send Wallbox (`/api/wallbox/set`) commands.

Quick purpose
- `cmd/wallbox-set` — small CLI to send `{ key, value }` to the API.
- `rest` package — helper `PostJSON` used by the CLI.

Backend components (folder map)

- `main.go`
	- Entrypoint for the Go API service.
	- Wires MQTT manager, REST handlers and long-running services.

- `mqtt/`
	- MQTT client setup and broker connectivity.
	- Loads broker settings from `mqtt/broker_config.json`.
	- Central source for incoming topic payloads used by status endpoints.

- `chickenDoor/`
	- Huhnerklappe domain logic (manual control, schedule, wake/sleep flow).
	- Persists UI/schedule state in bbolt (including timestamps, actions, auto-stop, history).
	- Provides endpoints used by the Huehnerklappe UI (status, ui-state, schedule updates, commands).

- `wallbox/`
	- Wallbox domain logic and REST/MQTT mapping.
	- Handles `status` aggregation and publishes `set` commands.

- `rest/`
	- Shared REST helper code and JSON POST utilities.
	- Used by internal packages and CLI helpers.

- `cmd/wallbox-set/`
	- Standalone CLI for `PUT /api/wallbox/set`.
	- Useful for smoke tests and automation scripts.

- `cmd/rest-service/`
	- Alternate command/service entrypoint with local configs in `cmd/rest-service/*.json`.

- `cmd/rest-cli/`
	- Small command-line helper for REST endpoint testing.

High-level API areas

- Wallbox: `/api/wallbox/status`, `/api/wallbox/set`
- ChickenDoor (Huhnerklappe): status/ui-state/schedule/set endpoints under `/api/huehnerklappe/*`
- MQTT debug/status: `/api/mqtt/status`

Build & run (local dev)

1. Start API (requires reachable MQTT broker configured in `mqtt/broker_config.json`):

```bash
cd /home/dominik/Repository/webgui/api-go
go run ./
# or run the CLI directly:
go run ./cmd/wallbox-set --key amp --value 8
```

CLI examples

- Send amp = 8

```bash
go run ./cmd/wallbox-set --key amp --value 8
```

- Send JSON payload (time window)

```bash
go run ./cmd/wallbox-set --key dwo --value '{"from":"08:00","to":"17:00"}'
```

Notes about values
- The `--value` string is first attempted to be parsed as JSON (so numbers, booleans, objects, arrays work). If parsing fails, it is sent as a plain string.

API endpoints (used by UI)
- `GET /api/wallbox/status` — returns current wallbox state stored from MQTT messages.
- `PUT /api/wallbox/set` — accepts JSON `{key,value}` and publishes to MQTT.
- `GET /api/mqtt/status` — debug endpoint that returns received MQTT messages.

Docker / Compose
- The repo includes `docker-compose.webui.yml` which builds `webui` and `api` services. Ensure your MQTT broker is reachable from the `api` container and `mqtt/broker_config.json` points to it.
- To rebuild the API image without cache (pick up code changes):

```bash
docker compose -f docker-compose.webui.yml build --no-cache api
docker compose -f docker-compose.webui.yml up -d
```

Debugging tips
- If the API reports "MQTT client is not connected" or receives no messages, check container logs:

```bash
docker compose -f docker-compose.webui.yml logs -f api
```

- Verify that the `api` container can reach the broker (replace container name accordingly):

```bash
API=$(docker ps --filter name=api -q)
docker exec -it $API cat /app/mqtt/broker_config.json
docker exec -it $API sh -c 'nc -vz 192.168.188.97 1883 || echo unreachable'
```

Contact / next steps
- If you want the CLI compiled into the Docker image or a Makefile target, say so and I can add it.
