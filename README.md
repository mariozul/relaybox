# Relaybox

Multi-tenant webhook relay service written in Go.

Relaybox ingests events durably into an outbox, persists them in PostgreSQL, and dispatches them to registered subscriber endpoints with exponential backoff, dead-lettering, and tenant isolation.

## Architecture

- **Language**: Go 1.25
- **Pattern**: 7-layer Clean Architecture (`domain` -> `repository` -> `service` -> `transport/http` -> `adapter` -> `config` -> `telemetry`)
- **Persistence**: PostgreSQL (transactional outbox)
- **Specification**: Defined in `docs/trd/trd.md`

## Local development

```bash
docker compose up -d      # Postgres
make test                 # go test -race -timeout 90s ./...
make lint                 # golangci-lint run ./...
make build
