# Deployment

## Mac Mini

Install Docker Desktop, copy `.env.example` to `.env`, configure only needed credentials, and run:

```sh
docker compose up --build -d
docker compose logs -f fraud-service
```

Persisted data is in `./data`. Back it up while the stack is stopped, or take a SQLite-safe backup through the Go service's host volume. To use a Google service account, place it at `./secrets/gcp-service-account.json` (that directory is read-only inside the service). Use Tailscale for private remote access rather than exposing the dashboard directly.

## VPS

Use a host with Docker Compose, a domain, and no other services on ports 80/443. Configure `CADDY_EMAIL`, `GRAFANA_HOST`, and `DASHBOARD_HOST` in `.env`, set a non-default `GRAFANA_PASSWORD`, then run:

```sh
docker compose --profile vps up --build -d
```

Before public exposure, keep Grafana behind authentication, restrict provider credentials to their minimum scopes, verify the `.env` file is not tracked, and test a clean deployment. Caddy is deliberately a profile so local development has no accidental public listener.

## Operations

- Start with a five-minute Flipify cadence and record live allowance/usage.
- Check `last_flipify_poll_success` and Prometheus's last-poll metric after deployment.
- Verify one test listing appears once in the dashboard, one urgent test sends one Telegram notification, and one normal qualified candidate appears in the next Resend digest.
- Review the independent Vision unit counts monthly before changing the primary-image policy.
