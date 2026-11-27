# Changelog

## 1.3.1 — unreleased

### Security

- **The API URL must start with `https://`** (`http://` only for `localhost`, `127.0.0.1` and `[::1]`), from the config,
  `--api-url` and `tolvyn login`.
- **A sign-in is sent only to the server that issued it.** `--api-url` naming another server says so and sends nothing;
  sign in there instead. A config written before 1.3.1 counts as issued by the `api_url` it has when 1.3.1 first
  reads it, and records that.
- **Redirects are not followed.**
- **Query values and ids are escaped** in every request.
- **Exports are created readable only by you** and never overwrite a file unless you pass `--force`, which replaces the
  file (or a symlink at that path) rather than writing through it; the config file and its directory are made private
  again on every save; `--force` replaces only a file or a symlink. `usage export` and `ledger export` stream to a
  temporary file renamed into place at the end, so an export that fails part-way with an error leaves nothing behind
  and an old file untouched. An export stopped with Ctrl-C leaves a `.tolvyn-export-*` file beside the output and,
  without `--force`, an empty file at the output path; delete both. Exports stop after 60 seconds without data from
  the API, not after 30 seconds in total.
- **`tail` stops after a minute without data** from the API.
- **`install.sh` checks the release's checksums** and, with cosign 3.1.3 or later installed, its signature (signed with
  cosign 3, in Sigstore's bundle format; 3.1.3 fixed the verification bypass GHSA-fx35-mq7g-6g98). Without cosign,
  or with an older or unreadable one, it says so and checks the checksum only. `TOLVYN_REQUIRE_SIGNATURE=1` installs only when the signature was verified. It installs the
  binary `0755`, owned by root when it uses sudo. `TOLVYN_VERSION` must be a release version (`v1.3.1`);
  `TOLVYN_INSTALL_DIR` installs elsewhere than `/usr/local/bin`.


### Documentation

- README: providers and wording updated (13 Nov), and the LICENSE file (Apache License 2.0, as the README's badge
  says) added. That change was numbered 1.3.1 but never released; it ships here.

## 1.3.0 — 2026-05-19

See the release notes on GitHub for v1.3.0.
