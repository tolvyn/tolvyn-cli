// The SK round's review fixes (26 Nov), loopback only.
//   - BE-656: a config written before 1.3.2 is bound to its api_url on first load, so a later change of api_url does
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
	"strings"
	"sync"
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
	f, err := createExportFile(p, true)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("new") //nolint:errcheck
	f.Close()
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
	old := httpClient.Timeout
	httpClient.Timeout = 300 * time.Millisecond
	defer func() { httpClient.Timeout = old }()
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
