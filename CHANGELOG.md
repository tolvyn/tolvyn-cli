# Changelog

## 1.3.2 — unreleased

### Security

- **The API URL must start with `https://`** (`http://` only for `localhost`, `127.0.0.1` and `[::1]`), from the config,
  `--api-url` and `tolvyn login`.
- **A sign-in is sent only to the server that issued it.** `--api-url` naming another server says so and sends nothing;
  sign in there instead. A config written before 1.3.2 counts as issued by its `api_url`.
- **Redirects are not followed.**
- **Query values and ids are escaped** in every request.
- **Exports are created readable only by you** and never overwrite a file unless you pass `--force`; the config file and
  its directory are made private again on every save.
- **`tail` stops after a minute without data** from the API.
- **`install.sh` checks the release's checksums** and, with cosign installed, its signature, and installs the binary
  `0755`, owned by root when it uses sudo.

## 1.3.1 — 2026-11-13

### Documentation

- README: providers and wording updated; no code change.
- The repository's LICENSE file (Apache License 2.0, as the README's badge says) was added after 1.3.0.
- `tolvyn version` reports 1.3.1. The version string is the only change in the source.

## 1.3.0 — 2026-05-19

See the release notes on GitHub for v1.3.0.
