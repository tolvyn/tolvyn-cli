# tolvyn CLI

Command-line interface for TOLVYN — the control and evidence layer for AI spend.

[![GitHub release](https://img.shields.io/github/v/release/tolvyn/tolvyn-cli)](https://github.com/tolvyn/tolvyn-cli/releases)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

**10,000 free requests a month. No credit card.**

## Install

```bash
curl -fsSL https://releases.tolvyn.io/install.sh | sh
```

The script checks the download against the release's `SHA256SUMS` and stops if they differ. With
[cosign](https://github.com/sigstore/cosign) installed it also checks that the release was built by this repository's
release workflow. `TOLVYN_VERSION=v1.3.2` installs a given release. To check a download by hand, fetch the binary,
`SHA256SUMS` and `SHA256SUMS.sigstore.json` from the release, then run:

```bash
sha256sum -c --ignore-missing SHA256SUMS
cosign verify-blob SHA256SUMS --bundle SHA256SUMS.sigstore.json \
  --certificate-identity "https://github.com/tolvyn/tolvyn-cli/.github/workflows/release.yml@refs/tags/v1.3.2" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Or via Go:

```bash
go install github.com/tolvyn/tolvyn-cli@latest
```

Or grab a platform binary directly from [github.com/tolvyn/tolvyn-cli/releases](https://github.com/tolvyn/tolvyn-cli/releases) (Linux x86_64/arm64, macOS Intel/Apple Silicon, Windows x86_64).

## Quick start

```bash
tolvyn init          # interactive: API URL + email + password
tolvyn status        # verify connection + auth
tolvyn tail          # live request stream
tolvyn cost          # spend summary
```

## Commands

58 cobra commands across 17 top-level groups.

### Authentication

| Command | Description |
|---|---|
| `tolvyn init` | Interactive setup — prompts for API URL, email, password |
| `tolvyn login` | Authenticate with an existing account |
| `tolvyn logout` | Clear stored credentials |
| `tolvyn status` | Show API + DB health and current auth state |

### Usage & cost

| Command | Description |
|---|---|
| `tolvyn cost [--by model\|team\|service\|user\|customer]` | Spend summary, optionally grouped by a dimension |
| `tolvyn requests` | List recent requests (`--team --service --model --from --to --limit`) |
| `tolvyn requests show <id>` | Full detail for one request including ledger linkage |
| `tolvyn tail` | Live SSE stream (`--team --service --model --min-cost --no-alerts`) |
| `tolvyn usage export` | Export request log as CSV (`-o file`) |

### Keys & providers

| Command | Description |
|---|---|
| `tolvyn keys list \| create \| revoke` | Manage TOLVYN API keys (`--env production\|test`) |
| `tolvyn providers list \| add \| delete` | Manage stored provider keys (OpenAI / Anthropic / Google) |

### Teams

| Command | Description |
|---|---|
| `tolvyn teams list \| create` | Manage teams (`--name`, `--cost-center`) |

### Budgets

| Command | Aliases | Description |
|---|---|---|
| `tolvyn budgets list` | `ls` | Show all budgets with utilization |
| `tolvyn budgets create` | `set` | Create a budget (`--scope --amount --period --mode`) |
| `tolvyn budgets delete` | `rm`, `remove` | Delete a budget |

### Alerts

| Command | Aliases | Description |
|---|---|---|
| `tolvyn alerts list` | `ls` | List alerts (`--unread --type --severity`) |
| `tolvyn alerts ack <id>` | `acknowledge` | Mark an alert acknowledged |
| `tolvyn alerts test` | — | Fire a synthetic test alert |

### Webhooks

| Command | Aliases | Description |
|---|---|---|
| `tolvyn webhooks list` | `ls` | List webhook endpoints |
| `tolvyn webhooks create` | — | Create endpoint (`--url --events`); signing secret shown once |
| `tolvyn webhooks delete <id>` | `rm` | Delete an endpoint |
| `tolvyn webhooks test <id>` | — | Trigger a synthetic delivery |
| `tolvyn webhooks deliveries <id>` | — | Recent 50 delivery attempts |

Top-level alias: `tolvyn wh`.

### Ledger

| Command | Description |
|---|---|
| `tolvyn ledger list` | Paginated hash-chained ledger records (`--limit --offset --from --to`) |
| `tolvyn ledger verify` | Walk the chain, validate every hash + HMAC. **Exits non-zero if broken — useful for CI/audit scripts** (`--from-seq --to-seq`) |
| `tolvyn ledger export` | Stream CSV including `record_hash`, `previous_hash`, `hmac_signature` for offline verification (`-o file`) |

### Models

| Command | Description |
|---|---|
| `tolvyn models` | List available models with current pricing (`--provider`) |
| `tolvyn models diff` | Historical pricing-change audit log (`--last 30d --provider`) |
| `tolvyn models changes` | Currently active pricing-change notifications for models you use |

### Savings

| Command | Description |
|---|---|
| `tolvyn savings list` | Active savings findings from the nightly analyzer (`--min-savings`) |
| `tolvyn savings dismiss <id>` | One-way dismiss (does not regenerate on future runs) |

### Governance

| Command | Aliases | Description |
|---|---|---|
| `tolvyn kill` | — | Emergency block (`--scope team\|service\|agent\|api_key\|all --target`) |
| `tolvyn kill list` | `ls` | List active kill switches |
| `tolvyn kill undo <id>` | `deactivate`, `rm` | Lift a kill switch |
| `tolvyn audit` | — | Audit log (`--from --to --last --format table\|csv --limit`) |
| `tolvyn reconcile upload <file.csv>` | — | Reconcile provider invoice CSV against TOLVYN ledger |
| `tolvyn reconcile list` | `ls` | Past runs |
| `tolvyn reconcile delete <run-id>` | `rm` | Delete a past run |

### Other

| Command | Description |
|---|---|
| `tolvyn version` | Print CLI version |

## Global flags

| Flag | Default | Description |
|---|---|---|
| `--json` | `false` | Output raw JSON instead of formatted tables |
| `--no-color` | `false` | Disable ANSI colors (auto-disabled when stdout is not a TTY) |
| `--api-url` | from config | Override API endpoint (default `https://api.tolvyn.io`). It must start with `https://` (`http://` only for `localhost`). Your sign-in is sent only to the server you signed in to; to use another server, sign in there: `tolvyn login --api-url https://...` |

## Configuration

Config file lives at `~/.tolvyn/config.json` (created on first `tolvyn init` / `tolvyn login`). Permissions: file `0600`, directory `0700`, set again on every save.

```json
{
  "api_url": "https://api.tolvyn.io",
  "token": "eyJhbGciOiJIUzI1NiIsInR...",
  "token_api_url": "https://api.tolvyn.io",
  "default_environment": "production"
}
```

`token_api_url` is the server that issued the token; the token is sent to no other.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Generic error (network, auth, validation, server 4xx/5xx) |

`tolvyn ledger verify` exits with code `1` if the chain integrity check fails — useful for CI/audit scripts:

```bash
tolvyn ledger verify || { echo "Ledger broken — investigate"; exit 1; }
```

## Changelog

[github.com/tolvyn/tolvyn-cli/releases](https://github.com/tolvyn/tolvyn-cli/releases)

## Links

- CLI reference: [docs.tolvyn.io/reference/cli](https://docs.tolvyn.io/reference/cli)
- API reference: [docs.tolvyn.io/reference/api](https://docs.tolvyn.io/reference/api)
- Dashboard: [app.tolvyn.io](https://app.tolvyn.io)
- Issues: [github.com/tolvyn/tolvyn-cli/issues](https://github.com/tolvyn/tolvyn-cli/issues)

## Community & Feedback

- Discussions: [github.com/tolvyn/tolvyn-cli/discussions](https://github.com/tolvyn/tolvyn-cli/discussions)
- Email: [founder@tolvyn.io](mailto:founder@tolvyn.io)
- Issues: [github.com/tolvyn/tolvyn-cli/issues](https://github.com/tolvyn/tolvyn-cli/issues)

---

© 2026 TOLVYN. Apache 2.0 licensed.
