// The SK round's review fixes (26 Nov), loopback only.
//   - BE-656: a config written before 1.3.1 is bound to its api_url on first load, so a later change of api_url does
//     not take the token with it; the issuer is compared by scheme, host and port; tail refuses a redirect.
//   - BE-659: --force never writes through a symlink at the output path; usage export streams past the in-memory cap;
//     reconcile upload's response is capped.
//   - The built binary: the URL rule and the token binding hold for the commands that build their own requests, and
//     a shell-completion request works whatever the config says.
//   - BE-339, BE-501: install.sh end to end, with a stand-in curl: a good install, no SHA256SUMS, a mismatch, and a
//     TOLVYN_VERSION that is not a version.
package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestALegacyConfigIsBoundOnFirstLoad(t *testing.T) {
	home(t)
	os.MkdirAll(filepath.Dir(configPath()), 0o700)                                                 //nolint:errcheck
	os.WriteFile(configPath(), []byte(`{"api_url":"http://127.0.0.1:7001","token":"tok"}`), 0o600) //nolint:errcheck
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(configPath()); !strings.Contains(string(b), `"token_api_url": "http://127.0.0.1:7001"`) {
		t.Errorf("the first load did not record the issuer: %s", b)
	}
	cfg.APIURL = "http://127.0.0.1:7002" // what an edit of api_url without a sign-in does
	save(t, *cfg)
	if tokenFor("http://127.0.0.1:7002") != "" || tokenFor("http://127.0.0.1:7001") != "tok" {
		t.Errorf("after api_url changed, the token follows it")
	}
}

func TestTheIssuerIsComparedBySchemeHostAndPort(t *testing.T) {
	home(t)
	save(t, Config{APIURL: "http://127.0.0.1:7001", Token: "tok", TokenAPIURL: "http://127.0.0.1:7001"})
	for _, other := range []string{"http://127.0.0.1:7002", "https://127.0.0.1:7001", "http://localhost:7001"} {
		if tokenFor(other) != "" {
			t.Errorf("the token goes to %s", other)
		}
	}
	if tokenFor("http://127.0.0.1:7001/v1") != "tok" {
		t.Errorf("the issuer itself gets no token")
	}
}

func TestTailRefusesARedirect(t *testing.T) {
	_, otherURL, other := serve(t, "localhost", jsonOK)
	_, aURL, _ := serve(t, "127.0.0.1", func(w http.ResponseWriter, q *http.Request) {
		http.Redirect(w, q, otherURL+q.URL.Path, http.StatusTemporaryRedirect)
	})
	home(t)
	save(t, Config{APIURL: aURL, Token: "tok", TokenAPIURL: aURL})
	if err := streamTail("/v1/stream"); err == nil {
		t.Errorf("tail followed a redirect without an error")
	}
	if n := len(other.all()); n != 0 {
		t.Errorf("the redirect target got %d request(s)", n)
	}
}

func TestForceNeverWritesThroughASymlink(t *testing.T) {
	d := t.TempDir()
	target := filepath.Join(d, "someone-elses-file")
	os.WriteFile(target, []byte("keep"), 0o644) //nolint:errcheck
	p := filepath.Join(d, "export.csv")
	os.Symlink(target, p) //nolint:errcheck
	if _, err := writeExport(p, true, strings.NewReader("new")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(target); string(b) != "keep" {
		t.Errorf("the symlink's target was written: %q", b)
	}
	if st, _ := os.Lstat(p); !st.Mode().IsRegular() || st.Mode().Perm() != 0o600 {
		t.Errorf("the export is %v", st.Mode())
	}
}

var (
	builtOnce sync.Once
	builtBin  string
	builtErr  error
)

// tolvyn builds the CLI once per test run and runs it with HOME at home.
func tolvyn(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	builtOnce.Do(func() {
		dir, err := os.MkdirTemp("", "tolvyn-cli-bin")
		if err != nil {
			builtErr = err
			return
		}
		builtBin = filepath.Join(dir, "tolvyn")
		if out, err := exec.Command("go", "build", "-o", builtBin, ".").CombinedOutput(); err != nil {
			builtErr = err
			t.Logf("%s", out)
		}
	})
	if builtErr != nil {
		t.Fatalf("go build: %v", builtErr)
	}
	cmd := exec.Command(builtBin, args...)
	cmd.Env = append(os.Environ(), "HOME="+home, "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestTheBuiltBinary(t *testing.T) {
	big := strings.Repeat("a", int(maxResponse)+1024)
	_, aURL, a := serve(t, "127.0.0.1", func(w http.ResponseWriter, q *http.Request) {
		switch q.URL.Path {
		case "/v1/usage/requests":
			w.Header().Set("Content-Type", "text/csv")
			w.Write([]byte(big)) //nolint:errcheck
		case "/v1/reconciliation/upload":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"x":"` + big + `"}`)) //nolint:errcheck
		default:
			jsonOK(w, q)
		}
	})
	_, bURL, b := serve(t, "localhost", jsonOK)
	h := home(t)
	save(t, Config{APIURL: aURL, Token: "tok", TokenAPIURL: aURL})
	csv := filepath.Join(h, "statement.csv")
	os.WriteFile(csv, []byte("date,amount\n"), 0o600) //nolint:errcheck

	own := [][]string{{"audit", "--format", "csv"}, {"ledger", "export"}, {"reconcile", "upload", csv, "--month", "2026-10", "--provider", "openai"}, {"usage", "export"}}
	for _, args := range own {
		out, err := tolvyn(t, h, append(args, "--api-url", bURL)...)
		if err == nil || !strings.Contains(out, "sign in there") {
			t.Errorf("%v --api-url another server: err=%v out=%q", args, err, out)
		}
		out, err = tolvyn(t, h, append(args, "--api-url", "http://example.com")...)
		if err == nil || !strings.Contains(out, "must start with https://") {
			t.Errorf("%v --api-url http://example.com: err=%v out=%q", args, err, out)
		}
	}
	if n := len(b.all()); n != 0 {
		t.Errorf("another server got %d request(s)", n)
	}

	out := filepath.Join(h, "usage.csv")
	if o, err := tolvyn(t, h, "usage", "export", "-o", out); err != nil {
		t.Errorf("usage export over the in-memory cap: %v %q", err, o)
	} else if st, _ := os.Stat(out); st == nil || st.Size() != int64(len(big)) {
		t.Errorf("usage export wrote %v bytes, want %d", st, len(big))
	}
	if o, err := tolvyn(t, h, "reconcile", "upload", csv, "--month", "2026-10", "--provider", "openai"); err == nil || !strings.Contains(o, "too large") {
		t.Errorf("reconcile upload read a response over the cap: err=%v", err)
	}
	if len(a.all()) == 0 {
		t.Errorf("the issuer got nothing")
	}

	// The default completion command is off (DisableDefaultCmd); cobra's hidden __complete request still exists.
	save(t, Config{APIURL: "http://example.com"})
	for _, args := range [][]string{{"__complete", "ledger", ""}, {"__completeNoDesc", "ledger", ""}} {
		if o, err := tolvyn(t, h, args...); err != nil {
			t.Errorf("%v with a config the URL rule refuses: %v %q", args, err, o)
		}
	}
}

// installEnv is a directory of the tools install.sh uses, with curl and uname replaced: curl serves a release from
// rel (by the URL's last segment, exit 22 when missing) and logs each URL; nothing else is on PATH (no cosign, no sudo).
func installEnv(t *testing.T, rel, log string) []string {
	t.Helper()
	bin := t.TempDir()
	for _, tool := range []string{"mktemp", "rm", "awk", "sha256sum", "cut", "install", "grep", "sed", "cp", "cat", "chmod"} {
		p, err := exec.LookPath(tool)
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		os.Symlink(p, filepath.Join(bin, tool)) //nolint:errcheck
	}
	os.WriteFile(filepath.Join(bin, "uname"), []byte("#!/bin/sh\ncase \"$1\" in -s) echo Linux ;; -m) echo x86_64 ;; esac\n"), 0o755) //nolint:errcheck
	os.WriteFile(filepath.Join(bin, "curl"), []byte(`#!/bin/sh
out=""; url=""
while [ $# -gt 0 ]; do
  case "$1" in -o) out="$2"; shift 2 ;; -*) shift ;; *) url="$1"; shift ;; esac
done
echo "$url" >> "$FAKE_LOG"
f="$FAKE_RELEASE/${url##*/}"
[ -f "$f" ] || exit 22
if [ -n "$out" ]; then cp "$f" "$out"; else cat "$f"; fi
`), 0o755) //nolint:errcheck
	return []string{"PATH=" + bin, "FAKE_RELEASE=" + rel, "FAKE_LOG=" + log, "HOME=" + t.TempDir()}
}

func TestInstallScriptEndToEnd(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	script, _ := filepath.Abs(filepath.Join("..", "..", "install.sh"))
	const asset = "tolvyn-linux-amd64"
	cases := []struct {
		name, version, sums, want string
		ok                        bool
	}{
		{"a good install", "v9.9.9", "good", "installed", true},
		{"no SHA256SUMS", "v9.9.9", "", "no SHA256SUMS", false},
		{"a mismatch", "v9.9.9", "bad", "Checksum mismatch", false},
		{"a TOLVYN_VERSION that is not a version", "v9.9.9/../../../other/repo", "good", "TOLVYN_VERSION", false},
		{"a TOLVYN_VERSION with a second line", "v9.9.9\n/../../other/repo", "good", "TOLVYN_VERSION", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rel, dest := t.TempDir(), t.TempDir()
			log := filepath.Join(t.TempDir(), "curl.log")
			os.WriteFile(filepath.Join(rel, asset), []byte("the binary"), 0o644) //nolint:errcheck
			switch c.sums {
			case "good":
				sum, _ := exec.Command("sha256sum", filepath.Join(rel, asset)).Output()
				os.WriteFile(filepath.Join(rel, "SHA256SUMS"), []byte(strings.Fields(string(sum))[0]+"  "+asset+"\n"), 0o644) //nolint:errcheck
			case "bad":
				os.WriteFile(filepath.Join(rel, "SHA256SUMS"), []byte(strings.Repeat("0", 64)+"  "+asset+"\n"), 0o644) //nolint:errcheck
			}
			cmd := exec.Command(sh, script)
			cmd.Env = append(installEnv(t, rel, log), "TOLVYN_VERSION="+c.version, "TOLVYN_INSTALL_DIR="+dest)
			out, err := cmd.CombinedOutput()
			if (err == nil) != c.ok || !strings.Contains(string(out), c.want) {
				t.Errorf("err=%v, output:\n%s", err, out)
			}
			st, statErr := os.Stat(filepath.Join(dest, "tolvyn"))
			if c.ok && (statErr != nil || st.Mode().Perm() != 0o755) {
				t.Errorf("the binary: %v %v", statErr, st)
			}
			if !c.ok && statErr == nil {
				t.Errorf("a binary was installed")
			}
			if logged, _ := os.ReadFile(log); c.want == "TOLVYN_VERSION" && len(logged) != 0 {
				t.Errorf("curl was called: %s", logged)
			}
		})
	}
}

// An export that fails part-way leaves nothing behind: no partial file, and with --force the old file untouched.
func TestAFailedExportLeavesNothingBehind(t *testing.T) {
	_, aURL, _ := serve(t, "127.0.0.1", func(w http.ResponseWriter, q *http.Request) {
		conn, buf, _ := w.(http.Hijacker).Hijack()
		buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: text/csv\r\nContent-Length: 100000\r\n\r\npartial,row\n") //nolint:errcheck
		buf.Flush()                                                                                                 //nolint:errcheck
		conn.Close()
	})
	h := home(t)
	save(t, Config{APIURL: aURL, Token: "tok", TokenAPIURL: aURL})
	old := filepath.Join(h, "old.csv")
	os.WriteFile(old, []byte("old"), 0o600) //nolint:errcheck
	if _, err := tolvyn(t, h, "usage", "export", "-o", old, "--force"); err == nil {
		t.Errorf("a cut-off export succeeded")
	}
	if b, _ := os.ReadFile(old); string(b) != "old" {
		t.Errorf("--force: the old file is now %q", b)
	}
	fresh := filepath.Join(h, "fresh.csv")
	tolvyn(t, h, "usage", "export", "-o", fresh) //nolint:errcheck
	if _, err := os.Lstat(fresh); err == nil {
		t.Errorf("a cut-off export left a file behind")
	}
	ledgerOld := filepath.Join(h, "ledger-old.csv")
	os.WriteFile(ledgerOld, []byte("old"), 0o600) //nolint:errcheck
	if _, err := tolvyn(t, h, "ledger", "export", "-o", ledgerOld, "--force"); err == nil {
		t.Errorf("a cut-off ledger export succeeded")
	}
	if b, _ := os.ReadFile(ledgerOld); string(b) != "old" {
		t.Errorf("ledger --force: the old file is now %q", b)
	}
	ledgerFresh := filepath.Join(h, "ledger-fresh.csv")
	tolvyn(t, h, "ledger", "export", "-o", ledgerFresh) //nolint:errcheck
	if _, err := os.Lstat(ledgerFresh); err == nil {
		t.Errorf("a cut-off ledger export left a file behind")
	}
	if left, _ := filepath.Glob(filepath.Join(h, ".tolvyn-export-*")); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

// A steady export is not cut off by the API client's total timeout (30 s): only by time without data.
func TestASteadyExportIsNotCutOffByTheClientTimeout(t *testing.T) {
	_, aURL, _ := serve(t, "127.0.0.1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		for i := 0; i < 8; i++ {
			w.Write([]byte("row\n")) //nolint:errcheck
			w.(http.Flusher).Flush()
			time.Sleep(100 * time.Millisecond)
		}
	})
	home(t)
	save(t, Config{APIURL: aURL, Token: "tok", TokenAPIURL: aURL})
	old, oldIdle := httpClient.Timeout, exportIdle
	httpClient.Timeout, exportIdle = 300*time.Millisecond, 300*time.Millisecond
	defer func() { httpClient.Timeout, exportIdle = old, oldIdle }()
	body, err := doRequestStream("/v1/usage/requests")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	b, err := io.ReadAll(body)
	if err != nil || len(b) != 32 {
		t.Errorf("read %d bytes, err=%v", len(b), err)
	}
}

// An export that stops sending data is stopped after exportIdle, with a message saying so.
func TestAStalledExportStops(t *testing.T) {
	_, aURL, _ := serve(t, "127.0.0.1", func(w http.ResponseWriter, q *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte("row\n")) //nolint:errcheck
		w.(http.Flusher).Flush()
		select {
		case <-q.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	home(t)
	save(t, Config{APIURL: aURL, Token: "tok", TokenAPIURL: aURL})
	old := exportIdle
	exportIdle = 300 * time.Millisecond
	defer func() { exportIdle = old }()
	start := time.Now()
	body, err := doRequestStream("/v1/usage/requests")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	_, err = io.ReadAll(body)
	if err == nil || !strings.Contains(err.Error(), "no data") || time.Since(start) > 3*time.Second {
		t.Errorf("a stalled export: err=%v after %v", err, time.Since(start))
	}
}

// F6 (Gate 2): the release builds exactly the asset names v1.3.0 shipped, and install.sh asks for exactly the four
// that are not Windows. Users and install.sh fetch these names; a rename breaks them.
func TestReleaseAssetsAreTheNamesV130Shipped(t *testing.T) {
	v130 := []string{"tolvyn-darwin-amd64", "tolvyn-darwin-arm64", "tolvyn-linux-amd64", "tolvyn-linux-arm64", "tolvyn-windows-amd64.exe"}
	wf, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`target: \[([^\]]*)\]`).FindSubmatch(wf)
	if m == nil || !strings.Contains(string(wf), `-o "dist/tolvyn-${GOOS}-${GOARCH}${ext}"`) ||
		!strings.Contains(string(wf), `ext=""; [ "$GOOS" = windows ] && ext=".exe"`) {
		t.Fatalf("the workflow's build matrix or output name changed shape")
	}
	var built []string
	for _, target := range strings.Split(string(m[1]), ",") {
		goos, goarch, _ := strings.Cut(strings.TrimSpace(target), "/")
		name := "tolvyn-" + goos + "-" + goarch
		if goos == "windows" {
			name += ".exe"
		}
		built = append(built, name)
	}
	sort.Strings(built)
	if strings.Join(built, " ") != strings.Join(v130, " ") {
		t.Errorf("the workflow builds %v, v1.3.0 shipped %v", built, v130)
	}
	sh, _ := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	var asked []string
	for _, a := range regexp.MustCompile(`ASSET="([^"]+)"`).FindAllSubmatch(sh, -1) {
		asked = append(asked, string(a[1]))
	}
	sort.Strings(asked)
	if strings.Join(asked, " ") != strings.Join(v130[:4], " ") {
		t.Errorf("install.sh asks for %v, want %v", asked, v130[:4])
	}
}

// The idle timer runs only while the CLI waits on the API: a reader that pauses (an export piped into less) does not
// stop an export the API is still sending.
func TestASlowReaderKeepsTheExportAlive(t *testing.T) {
	_, aURL, _ := serve(t, "127.0.0.1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte("a\n")) //nolint:errcheck
		w.(http.Flusher).Flush()
		time.Sleep(400 * time.Millisecond) // while the reader is away, longer than the idle limit
		w.Write([]byte("b\nc\n"))          //nolint:errcheck
	})
	home(t)
	save(t, Config{APIURL: aURL, Token: "tok", TokenAPIURL: aURL})
	old := exportIdle
	exportIdle = 300 * time.Millisecond
	defer func() { exportIdle = old }()
	body, err := doRequestStream("/v1/usage/requests")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	first := make([]byte, 2)
	if _, err := io.ReadFull(body, first); err != nil {
		t.Fatal(err)
	}
	time.Sleep(700 * time.Millisecond) // the reader is busy elsewhere
	rest, err := io.ReadAll(body)
	if err != nil || string(first)+string(rest) != "a\nb\nc\n" {
		t.Errorf("after a pause: %q, err=%v", string(first)+string(rest), err)
	}
}

// --force replaces a file or a symlink, nothing else: a named pipe (or a device) at the path is refused and kept.
func TestForceRefusesASpecialFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Skip("no named pipes here:", err)
	}
	if _, err := writeExport(p, true, strings.NewReader("x")); err == nil {
		t.Errorf("--force replaced a named pipe")
	}
	if st, err := os.Lstat(p); err != nil || st.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("the named pipe is now %v (err %v)", st, err)
	}
}

// Item SK, 27 Nov: the release is signed by cosign v3 (cosign-installer v4), in Sigstore's new bundle format.
//   - cosign v3 or later on PATH: the signature is verified, and a failed verify installs nothing.
//   - no cosign, a cosign older than 3, or one whose version cannot be read: the same, a message, and the SHA256
//     checksum only (an old cosign is never asked to read the new format).
//   - TOLVYN_REQUIRE_SIGNATURE=1: nothing is installed unless the signature was verified. Any other value but 0 or
//     unset is refused, so a mistyped "yes" cannot silently mean "not required".
func TestInstallScriptChecksTheSignatureWithCosignV3OrLater(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	script, _ := filepath.Abs(filepath.Join("..", "..", "install.sh"))
	const asset = "tolvyn-linux-amd64"
	const checksumOnly = "cosign 3 or later is needed to check the signature; checking the SHA256 checksum only"
	const verified = "Signature verified (cosign)"
	cases := []struct {
		name, cosign, verify string // cosign: "" means none on PATH
		bundle               bool
		require              string
		want                 string
		ok, verifyCalled     bool
	}{
		{"cosign v3, the signature verifies", "v3.0.2", "ok", true, "", verified, true, true},
		{"cosign v4, the signature verifies", "v4.1.0", "ok", true, "", verified, true, true},
		{"cosign v3, the signature does not verify", "v3.0.2", "bad", true, "", "does not verify", false, true},
		{"cosign v3, no signature bundle in the release", "v3.0.2", "ok", false, "", "does not verify", false, false},
		{"no cosign: the checksum only", "", "ok", true, "", checksumOnly, true, false},
		{"cosign v2, as if none: the checksum only", "v2.4.1", "ok", true, "", checksumOnly, true, false},
		{"a cosign whose version cannot be read, as if none", "devel", "ok", true, "", checksumOnly, true, false},
		{"required, cosign v3 verifies", "v3.0.2", "ok", true, "1", verified, true, true},
		{"required, no cosign: refused", "", "ok", true, "1", "TOLVYN_REQUIRE_SIGNATURE=1", false, false},
		{"required, cosign v2: refused", "v2.4.1", "ok", true, "1", "TOLVYN_REQUIRE_SIGNATURE=1", false, false},
		{"required, an unreadable cosign: refused", "devel", "ok", true, "1", "TOLVYN_REQUIRE_SIGNATURE=1", false, false},
		{"required, the signature does not verify: refused", "v3.0.2", "bad", true, "1", "does not verify", false, true},
		{"required set to 0: not required", "", "ok", true, "0", checksumOnly, true, false},
		{"required set to anything else: refused", "v3.0.2", "ok", true, "yes", "TOLVYN_REQUIRE_SIGNATURE must be 1", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rel, dest := t.TempDir(), t.TempDir()
			log := filepath.Join(t.TempDir(), "curl.log")
			os.WriteFile(filepath.Join(rel, asset), []byte("the binary"), 0o644) //nolint:errcheck
			sum, _ := exec.Command("sha256sum", filepath.Join(rel, asset)).Output()
			os.WriteFile(filepath.Join(rel, "SHA256SUMS"), []byte(strings.Fields(string(sum))[0]+"  "+asset+"\n"), 0o644) //nolint:errcheck
			if c.bundle {
				os.WriteFile(filepath.Join(rel, "SHA256SUMS.sigstore.json"), []byte("{}"), 0o644) //nolint:errcheck
			}
			env := installEnv(t, rel, log)
			if c.cosign != "" {
				bin := strings.TrimPrefix(env[0], "PATH=")
				os.WriteFile(filepath.Join(bin, "cosign"), []byte(`#!/bin/sh
case "$1" in
  version) printf '  ______\ncosign: A tool for Container Signing\n\nGitVersion:    %s\nGitCommit:     0000000\n' "$FAKE_COSIGN_VERSION" ;;
  verify-blob) echo "$@" >> "$FAKE_LOG.cosign"; [ "$FAKE_COSIGN_VERIFY" = ok ] ;;
  *) exit 2 ;;
esac
`), 0o755) //nolint:errcheck
			}
			cmd := exec.Command(sh, script)
			cmd.Env = append(env, "TOLVYN_VERSION=v9.9.9", "TOLVYN_INSTALL_DIR="+dest,
				"FAKE_COSIGN_VERSION="+c.cosign, "FAKE_COSIGN_VERIFY="+c.verify)
			if c.require != "" {
				cmd.Env = append(cmd.Env, "TOLVYN_REQUIRE_SIGNATURE="+c.require)
			}
			out, err := cmd.CombinedOutput()
			if (err == nil) != c.ok || !strings.Contains(string(out), c.want) {
				t.Errorf("err=%v, output:\n%s", err, out)
			}
			_, statErr := os.Stat(filepath.Join(dest, "tolvyn"))
			if c.ok != (statErr == nil) {
				t.Errorf("installed=%v, want %v", statErr == nil, c.ok)
			}
			args, _ := os.ReadFile(log + ".cosign")
			if c.verifyCalled != (len(args) > 0) {
				t.Errorf("cosign verify-blob called=%v, want %v", len(args) > 0, c.verifyCalled)
			}
			if c.verifyCalled {
				for _, want := range []string{"--bundle", "SHA256SUMS.sigstore.json",
					"--certificate-identity https://github.com/tolvyn/tolvyn-cli/.github/workflows/release.yml@refs/tags/v9.9.9",
					"--certificate-oidc-issuer https://token.actions.githubusercontent.com"} {
					if !strings.Contains(string(args), want) {
						t.Errorf("cosign verify-blob was called without %q: %s", want, args)
					}
				}
			}
		})
	}
}
