# tolvyn CLI

The terminal interface for TOLVYN — watch live AI spend, manage keys, set budgets, and kill runaway services.

---

## Installation

**Binary download (recommended):**

```bash
# Linux / macOS
curl -sSL https://github.com/tolvyn/tolvyn-cli/releases/latest/download/tolvyn-$(uname -s)-$(uname -m) \
    -o /usr/local/bin/tolvyn && chmod +x /usr/local/bin/tolvyn
```

**go install:**

```bash
go install github.com/tolvyn/tolvyn-cli@latest
```

---

## Getting started

```bash
tolvyn init      # interactive setup — saves API URL, logs in, stores token
tolvyn status    # verify connectivity
tolvyn tail      # stream live requests
```

---

## Commands

| Command                 | Description                                                   | Key Flags                                                     |
|-------------------------|---------------------------------------------------------------|---------------------------------------------------------------|
| `tolvyn init`           | Interactive first-time setup: API URL, email, password        |                           —                                   |
| `tolvyn login`          | Authenticate with email and password                          |                           —                                   |
| `tolvyn logout`         | Clear stored credentials                                      |                           —                                   |
| `tolvyn status`         | Check API connectivity, database health, auth status, version |                           —                                   |
| `tolvyn tail`           | Stream live AI requests via SSE                               | `--team`, `--service`, `--model`, `--min-cost`, `--no-alerts` |
| `tolvyn cost`           | Show spend summary for a date range                           | `--from`, `--to`, `--team`, `--model`                         |
| `tolvyn requests`       | Show paginated request log                                    | `--team`, `--model`, `--from`, `--to`, `--limit`              |
| `tolvyn keys list`      | List all API keys (prefix, env, last used)                    |                           —                                   |
| `tolvyn keys create`    | Generate a new TOLVYN API key                                 | `--name` (required), `--env`, `--team`                        |
| `tolvyn keys revoke`    | Revoke an API key by ID                                       |                           —                                   |
| `tolvyn providers list` | List stored provider keys                                     |                           —                                   |
| `tolvyn providers add`  | Add or rotate a provider key (openai, anthropic, google)      |                           —                                   |
| `tolvyn teams list`     | List all teams                                                |                           —                                   |
| `tolvyn teams create`   | Create a new team                                             | `--name` (required), `--cost-center`                          |
| `tolvyn budgets list`   | List all budgets with utilization                             |                           —                                   |
| `tolvyn budgets create` | Create a budget                                               | `--scope`, `--team`, `--amount`, `--period`, `--mode`         |
| `tolvyn kill`           | Block a team immediately (creates $0.000001 hard budget)      | `--team`                                                      |

**Global flags (available on all commands):**

| Flag         | Description                               |
|--------------|-------------------------------------------|
| `--json`     | Output raw JSON instead of formatted text |
| `--no-color` | Disable colored output                    |
| `--api-url`  | Override the API URL from config          |

---

## tolvyn tail

`tolvyn tail` is the flagship command. It connects to the TOLVYN server's SSE endpoint and streams each AI request to your terminal as it completes.

```bash
tolvyn tail
```

**Output format:**

```
TIME     | TEAM/SERVICE          | MODEL            |   TOKENS |     COST | LATENCY
─────────┼───────────────────────┼──────────────────┼──────────┼──────────┼─────────
14:22:01 | backend/summariser    | gpt-4o           |    1,847 |  $0.0185 |    923ms
14:22:03 | ml-team/classifier    | claude-sonnet-4-6|      412 |  $0.0031 |    341ms
14:22:07 | search/reranker       | gpt-4o-mini      |    3,201 |  $0.0005 |    187ms
[ALERT] Budget: ml-team monthly budget reached 90% ($450.23 / $500.00)
```

Columns:
- **TIME** — HH:MM:SS of the completed request
- **TEAM/SERVICE** — `team/service` from the API key tags (up to 22 chars)
- **MODEL** — Model ID (up to 16 chars)
- **TOKENS** — Combined input + output tokens, formatted with thousands separators
- **COST** — Total cost in USD (green in color terminals)
- **LATENCY** — Total response time in ms (or `—` if not available)

**Filtering:**

```bash
tolvyn tail --team backend           # only requests from the backend team
tolvyn tail --service summariser     # only requests from a specific service
tolvyn tail --model gpt-4o           # substring match on model name
tolvyn tail --min-cost 0.10          # only requests costing more than $0.10
tolvyn tail --no-alerts              # suppress [ALERT] lines
```

**Reconnection:** The CLI automatically reconnects up to 3 times (5-second backoff) if the stream is interrupted.

---

## tolvyn kill

Emergency stop for a team. Creates a `$0.000001` hard-mode budget, effectively blocking all further proxy requests from that team.

```bash
tolvyn kill --team ml-team
# Budget created: budget_id=abc123
# To undo: tolvyn budgets delete abc123
```

The command prints the budget ID so it can be deleted to restore access.

---

## tolvyn budgets create

```bash
# Soft monthly budget: warns at 50/75/90/100%, does not block
tolvyn budgets create \
    --scope team \
    --team backend \
    --amount 500 \
    --period monthly \
    --mode soft

# Hard daily budget: blocks requests at limit
tolvyn budgets create \
    --scope service \
    --amount 10 \
    --period daily \
    --mode hard
```

**Scope values:** `org`, `team`, `service`
**Period values:** `monthly`, `weekly`, `daily`
**Mode values:** `soft` (alert only), `hard` (block requests)

---

## Configuration

The CLI stores its configuration at `~/.tolvyn/config.json`.

```json
{
    "api_url": "https://api.tolvyn.io",
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "default_environment": "production"
}
```

| Field                 | Description                                                          |
|-----------------------|----------------------------------------------------------------------|
| `api_url`             | TOLVYN API base URL                                                  |
| `token`               | JWT bearer token (stored after `tolvyn login`)                       |
| `default_environment` | Default environment for new API keys (`production` or `development`) |

The config file is written with `0600` permissions (owner read/write only).

To use a different API URL (e.g. local dev server):

```bash
tolvyn --api-url http://localhost:8081 status
```
