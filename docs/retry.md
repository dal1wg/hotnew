# Retry Guide

## Store Backends

`HOTNEW_STORE_BACKEND` supports `memory`, `file`, and `sqlite`.

### File Backend Paths

- `HOTNEW_FILE_ARTICLES_PATH`
- `HOTNEW_FILE_DELIVERIES_PATH`
- `HOTNEW_FILE_RETRIES_PATH`
- `HOTNEW_FILE_RETRIES_ARCHIVE_PATH`

### SQLite Backend Path

- `HOTNEW_SQLITE_DSN`

## Retry Settings

- `HOTNEW_RETRY_ENABLED`
- `HOTNEW_RETRY_INTERVAL`
- `HOTNEW_RETRY_BATCH_SIZE`
- `HOTNEW_RETRY_TIMEOUT`
- `HOTNEW_RETRY_MAX_ATTEMPTS`
- `HOTNEW_RETRY_BACKOFF`
- `HOTNEW_RETRY_MAX_BACKOFF`
- `HOTNEW_RETRY_ARCHIVE_AFTER`
- `HOTNEW_RETRY_ARCHIVE_BATCH`

## Exponential Backoff

Retry delay starts from `HOTNEW_RETRY_BACKOFF` and doubles on each failed retry until it reaches `HOTNEW_RETRY_MAX_BACKOFF`.

Example with `5m` base and `6h` max:

- 1st retry delay: `5m`
- 2nd retry delay: `10m`
- 3rd retry delay: `20m`
- 4th retry delay: `40m`
- continues doubling until capped at `6h`

## Retry APIs

### Query Retry Jobs

`GET /v1/retries`

Query params:

- `status`
- `channel`
- `article_id`
- `limit`

Examples:

```text
GET /v1/retries?status=failed&channel=blog&limit=50
GET /v1/retries?article_id=<article_id>
```

### Run Retry Batch

`POST /v1/retries/run?limit=10`

### Run Single Retry Job

`POST /v1/retries/run-one?id=<retry_job_id>`

### Reset Single Retry Job

`POST /v1/retries/reset?id=<retry_job_id>`

Reset puts the job back into `queued`, clears `last_error`, and resets `attempts` to `0`.

### Archive Succeeded Retry Jobs

`POST /v1/retries/archive`

Query params:

- `older_than`: optional duration like `24h`, `168h`
- `limit`: optional archive batch size

Example:

```text
POST /v1/retries/archive?older_than=72h&limit=200
```

Behavior:

- `sqlite` backend moves rows from `retry_jobs` to `retry_jobs_archive`
- `file` backend moves entries from active retry JSONL to archive JSONL
- `memory` backend removes active succeeded jobs and keeps an in-memory archive slice only for the process lifetime