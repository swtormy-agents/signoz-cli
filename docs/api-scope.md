# SigNoz API scope (signoz-cli v0.1)

This document lists REST endpoints used by **signoz-cli v0.1**. The CLI is unofficial and community-built; it is not affiliated with SigNoz.

Authentication for all calls below: header `SIGNOZ-API-KEY` (from env `SIGNOZ_API_KEY`). Base URL from env `SIGNOZ_URL` or flag `--url`.

## Covered in v0.1

| Command | Method | Path | Documentation |
|---------|--------|------|---------------|
| `signoz health`, `signoz version` | GET | `/api/v1/version` | [SigNoz API reference](https://signoz.io/api-reference/) (version endpoint); OpenAPI in [SigNoz/signoz `docs/api/openapi.yml`](https://github.com/SigNoz/signoz/blob/main/docs/api/openapi.yml) |
| `signoz logs query` | POST | `/api/v5/query_range` | [Logs API overview](https://signoz.io/docs/logs-management/logs-api/overview/) |
| `signoz traces query` | POST | `/api/v5/query_range` | [Traces API](https://signoz.io/docs/apm-and-distributed-tracing/traces-api/) |
| `signoz alerts list` | GET | `/api/v3/rules` | [Alerts management](https://signoz.io/docs/userguide/alerts-management/) (lists rules via API); OpenAPI `ListRulesV3` |
| `signoz services list` | GET | `/api/v1/services/list` | Documented in public SigNoz API reference / legacy query-service routes (service name list) |
| `signoz api` | * | `/api/...` | Escape hatch for any path under `/api` confirmed in your instance |

The v5 querier (`POST /api/v5/query_range`) is also described in the SigNoz OpenAPI spec (`QueryRangeV5`) and used by the [official SigNoz MCP server](https://github.com/SigNoz/signoz-mcp-server) for composite queries.

## Planned (not in v0.1 yet)

- `GET /api/v2/rules/{id}` — alert get
- `GET /api/v1/traces/{traceId}` — trace detail
- Metrics queries via `/api/v5/query_range` with `signal: metrics`
- Dashboard list/get endpoints from OpenAPI

## Out of scope

- SigNoz **ee/** enterprise-only routes
- Routes not present in public docs or OpenAPI without explicit confirmation
