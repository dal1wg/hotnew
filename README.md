# hotnew

`hotnew` is a lightweight Go-based hot news aggregation pipeline designed for compliant source ingestion, normalized processing, storage, summarization, and downstream distribution.

## Features

- Go backend with small dependency surface
- Compliant source-oriented pipeline: `Source -> Ingest -> Normalize -> Summarize -> Store -> Distribute`
- Configurable storage backend: `memory`, `file` (JSONL), `sqlite`
- Blog/webhook distribution support
- Delivery tracking and retry queue
- Exponential retry backoff with archive support
- Built-in read-only dashboard at `/`

## Project Structure

```text
hotnew/
├─ AGENTS.md
├─ README.md
├─ cmd/hotnew/
├─ internal/
│  ├─ app/
│  ├─ config/
│  ├─ distribute/
│  ├─ domain/
│  ├─ ingest/
│  ├─ normalize/
│  ├─ source/
│  ├─ store/
│  └─ summarize/
├─ docs/
└─ go.mod
```

## Quick Start

### 1. Build

```bash
go build ./...
```

### 2. Run

```bash
go run ./cmd/hotnew
```

Default server address:

```text
:8080
```

Open:

- Dashboard: `http://localhost:8080/`
- Health: `http://localhost:8080/healthz`

## Core Environment Variables

### HTTP

- `HOTNEW_HTTP_ADDR`

### Storage

- `HOTNEW_STORE_BACKEND=sqlite|file|memory`
- `HOTNEW_SQLITE_DSN`
- `HOTNEW_FILE_ARTICLES_PATH`
- `HOTNEW_FILE_DELIVERIES_PATH`
- `HOTNEW_FILE_RETRIES_PATH`
- `HOTNEW_FILE_RETRIES_ARCHIVE_PATH`

### Retry

- `HOTNEW_RETRY_ENABLED`
- `HOTNEW_RETRY_INTERVAL`
- `HOTNEW_RETRY_BATCH_SIZE`
- `HOTNEW_RETRY_TIMEOUT`
- `HOTNEW_RETRY_MAX_ATTEMPTS`
- `HOTNEW_RETRY_BACKOFF`
- `HOTNEW_RETRY_MAX_BACKOFF`
- `HOTNEW_RETRY_ARCHIVE_AFTER`
- `HOTNEW_RETRY_ARCHIVE_BATCH`

### Blog Distribution

- `HOTNEW_BLOG_ENABLED`
- `HOTNEW_BLOG_ENDPOINT`
- `HOTNEW_BLOG_TIMEOUT`
- `HOTNEW_BLOG_AUTH_TOKEN`
- `HOTNEW_BLOG_SITE_NAME`
- `HOTNEW_BLOG_AUTHOR`
- `HOTNEW_BLOG_MODE`

## Key APIs

### Run Ingest

```text
POST /v1/run?limit=10
```

### List Articles

```text
GET /v1/articles?limit=20
```

### List Deliveries

```text
GET /v1/deliveries?limit=20
```

### List Retry Jobs

```text
GET /v1/retries?status=failed&channel=blog&article_id=<article_id>&limit=50
```

### Run One Retry Job

```text
POST /v1/retries/run-one?id=<retry_job_id>
```

### Reset One Retry Job

```text
POST /v1/retries/reset?id=<retry_job_id>
```

### Archive Old Succeeded Retry Jobs

```text
POST /v1/retries/archive?older_than=72h&limit=200
```

## Dashboard

The built-in dashboard is intentionally simple and read-mostly. It provides:

- article list
- delivery list
- retry queue list with filters
- manual retry run/reset actions
- runner status view

No frontend build step is required.

## Notes

- `sqlite` is the recommended backend for persistent single-node deployment.
- `file` is useful for low-complexity local deployments and debugging.
- `memory` is only suitable for local testing.
- Retry behavior uses exponential backoff capped by `HOTNEW_RETRY_MAX_BACKOFF`.

## Additional Docs

- [Retry Guide](./docs/retry.md)
- [Agent Rules](./AGENTS.md)
## Standard Config

A standard environment template is provided at:

- `configs/hotnew.env.example`

Use this file as baseline environment settings, including WeCom robot variables:

- `HOTNEW_WECOM_ENABLED`
- `HOTNEW_WECOM_WEBHOOK`
- `HOTNEW_WECOM_TIMEOUT`

## Build Scripts

- PowerShell (Windows): `scripts/build.ps1`
- Bash (Linux/macOS): `scripts/build.sh`

Examples:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build.ps1
```

```bash
bash scripts/build.sh
```
