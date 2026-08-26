# API

The Go service listens on port 8080. The Flask UI proxies the same public JSON routes.

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/healthz` | Service health. |
| `GET` | `/api/listings?page=1&per_page=25` | Paginated, qualified listings only. |
| `GET` | `/api/listings/{id}` | Listing detail, including fraud flags. |
| `POST` | `/api/listings/{id}/mark` | Set `status` and `notes`. Valid statuses are `new`, `interested`, `contacted`, `toured`, `applied`, and `passed`. |
| `GET` | `/api/stats` | Today's queue and cost summary. |
| `POST` | `/api/model-evaluations` | Sidecar-only model metrics persistence endpoint. |
| `POST` | `/api/costs` | Append a controlled external-service cost ledger entry. |
| `GET` | `:9090/metrics` | Prometheus metric exposition. |

Example review update:

```sh
curl -X POST http://localhost:8080/api/listings/1/mark \
  -H 'content-type: application/json' \
  -d '{"status":"contacted","notes":"Asked about lease date"}'
```

Provider API calls stay server-side. Browser clients never receive API keys, contact data in notifications, or direct database access.
