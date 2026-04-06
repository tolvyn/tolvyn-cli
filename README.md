# TOLVYN CLI

Financial control plane for AI infrastructure.

## Install

```bash
curl -fsSL https://releases.tolvyn.io/install.sh | sh
```

Or via Go:

```bash
go install github.com/tolvyn/tolvyn-cli@latest
```

## Usage

```bash
tolvyn login
tolvyn tail              # live cost stream
tolvyn cost              # spend summary
tolvyn budgets list      # budget utilization
```

Kill a runaway team immediately:

```bash
tolvyn kill --team marketing
```

## What you get

- `tolvyn tail` — stream every AI request live: team, service, model, tokens, cost, latency
- `tolvyn cost` — spend breakdown by team, model, or date range
- `tolvyn budgets` — set hard limits that block requests before they hit your provider
- `tolvyn kill` — emergency stop for any team

Full docs: [docs.tolvyn.io](https://docs.tolvyn.io)
Free trial: [tolvyn.io](https://tolvyn.io)

---

© 2026 TOLVYN. All rights reserved.
