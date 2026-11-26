# Changelog

## 1.3.2 — unreleased

### Security

- **The API URL must start with `https://`** (`http://` only for `localhost`, `127.0.0.1` and `[::1]`), from the config,
  `--api-url` and `tolvyn login`.
- **A sign-in is sent only to the server that issued it.** `--api-url` naming another server says so and sends nothing;
  sign in there instead. A config written before 1.3.2 counts as issued by the `api_url` it has when 1.3.2 first
  reads it, and records that.
- **Redirects are not followed.**
- **Query values and ids are escaped** in every request.
- **Exports are created readable only by you** and never overwrite a file unless you pass `--force`, which replaces the
  file (or a symlink at that path) rather than writing through it; the config file and its directory are made private
  again on every save. `usage export` and `ledger export` stream to a temporary file renamed into place at the end,
  so an export that fails part-way leaves nothing behind and an old file untouched; they stop after 60 seconds
  without data, not after 30 seconds in total.
- **`tail` stops after a minute without data** from the API.
- **`install.sh` checks the release's checksums** and, with cosign installed, its signature, and installs the binary
  `0755`, owned by root when it uses sudo. `TOLVYN_VERSION` must be a release version (`v1.3.2`);
  `TOLVYN_INSTALL_DIR` installs elsewhere than `/usr/local/bin`.

## 1.3.1 — 2026-11-13

### Documentation

- README: providers and wording updated; no code change.
- The repository's LICENSE file (Apache License 2.0, as the README's badge says) was added after 1.3.0.
- `tolvyn version` reports 1.3.1. The version string is the only change in the source.

## 1.3.0 — 2026-05-19

See the release notes on GitHub for v1.3.0.
