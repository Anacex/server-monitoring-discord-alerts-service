# server-monitor

Lightweight Go service that checks SSL certificate expiry and disk space usage,
sending alerts to a Discord webhook when thresholds are crossed.

## Files
- `main.go` — config loading + orchestration loop
- `ssl.go` — TLS-based certificate expiry check (no shelling out to openssl)
- `disk.go` — disk usage check via /proc/mounts + syscall.Statfs (no shelling out to df)
- `notifier.go` — Discord webhook sender with per-alert cooldown/dedup
- `Dockerfile` — multi-stage build, final image based on `scratch` (~10MB)

## Build locally (test before containerizing)

```bash
go build -o server-monitor .
DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/..." \
SSL_DOMAINS="example.com,yourdomain.com" \
SSL_THRESHOLD_DAYS=14 \
DISK_THRESHOLD_PERCENT=80 \
CHECK_INTERVAL_MINUTES=1 \
STATE_DIR=/tmp/server-monitor-state \
./server-monitor
```

It runs an infinite loop (check, sleep, repeat) — Ctrl+C to stop during testing.
Set `CHECK_INTERVAL_MINUTES=1` while testing so you don't wait hours between runs;
set it back to something like 360 (6 hours) for real deployment.

## Build the Docker image

```bash
docker build -t server-monitor:latest .
```

## Push to your private registry

```bash
docker tag server-monitor:latest your-registry.company.com/devops/server-monitor:latest
docker push your-registry.company.com/devops/server-monitor:latest
```

## Run on a server — IMPORTANT: host disk visibility

By default, a container only sees its own internal filesystem. To get REAL
host disk usage (not the container's), bind-mount the host root filesystem
(read-only) into the container and point the disk check at it via
`DISK_CHECK_PATH=/hostfs`.

```bash
docker run -d \
  --name server-monitor \
  --restart unless-stopped \
  -e DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/your/webhook/url" \
  -e SSL_DOMAINS="example.com,yourdomain.com" \
  -e SSL_THRESHOLD_DAYS=14 \
  -e DISK_THRESHOLD_PERCENT=80 \
  -e CHECK_INTERVAL_MINUTES=360 \
  -e ALERT_COOLDOWN_HOURS=24 \
  -e DISK_CHECK_PATH=/hostfs \
  -v /:/hostfs:ro \
  -v server-monitor-state:/var/lib/server-monitor \
  your-registry.company.com/devops/server-monitor:latest
```

This mounts the host's entire root filesystem read-only at `/hostfs` inside
the container, with no need for `--pid host` or exposing the host's `/proc` —
keeps the container properly isolated while still getting real host stats.

The state volume (`server-monitor-state`) ensures cooldown/dedup state
survives container restarts — without it, every restart would re-send alerts
since the "already alerted" memory would reset.

## Verify it's running

```bash
docker logs -f server-monitor
```

You should see periodic log lines for each SSL/disk check, and Discord
messages appearing in your alerts channel when thresholds are crossed.