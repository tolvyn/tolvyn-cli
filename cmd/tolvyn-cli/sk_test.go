// The SK round's tests for tolvyn-cli, the CLI's first. Loopback only (run in a no-network sandbox): 127.0.0.1 is the
// API, localhost another host. HOME is a temporary directory in every test.
//   - BE-612: an API URL that is not https is refused, except to localhost, 127.0.0.1 and [::1].
//   - BE-656: the token goes only to the server that issued it; a config written before 1.3.2 is bound to its api_url;
//     no redirect is followed.
//   - BE-657: query values and ids are escaped.
//   - BE-659: exports are created 0600 and never overwrite without --force; responses are capped; tail stops after a
//     time without data.
//   - BE-667: the config file and its directory are private after every save.
//   - BE-658: install.sh refuses a download whose checksum does not match.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

type seenReq struct{ method, path, auth string }

type rec struct {
	mu   sync.Mutex
	reqs []seenReq
}

func (r *rec) all() []seenReq {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]seenReq(nil), r.reqs...)
}

func serve(t *testing.T, host string, answer http.HandlerFunc) (*httptest.Server, string, *rec) {
	t.Helper()
	r := &rec{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		io.Copy(io.Discard, q.Body) //nolint:errcheck
		r.mu.Lock()
		r.reqs = append(r.reqs, seenReq{q.Method, q.URL.EscapedPath() + "?" + q.URL.RawQuery, q.Header.Get("Authorization")})
		r.mu.Unlock()
		answer(w, q)
	}))
	t.Cleanup(s.Close)
	return s, strings.Replace(s.URL, "127.0.0.1", host, 1), r
}

func jsonOK(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"token":"tok-new"}`) //nolint:errcheck
}

func home(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	t.Setenv("HOME", d)
	apiURLOverride = ""
	t.Cleanup(func() { apiURLOverride = "" })
	return d
}

func save(t *testing.T, c Config) {
	t.Helper()
	if err := saveConfig(&c); err != nil {
		t.Fatal(err)
	}
}

func TestAnAPIURLThatIsNotHttpsIsRefused(t *testing.T) {
	for _, u := range []string{"http://example.com", "http://api.tolvyn.io", "ftp://127.0.0.1", "not a url"} {
		if err := validateAPIURL(u); err == nil || !strings.Contains(err.Error(), "https://") && !strings.Contains(err.Error(), "valid URL") {
			t.Errorf("%s accepted (err=%v)", u, err)
		}
	}
	for _, u := range []string{"https://api.tolvyn.io", "http://localhost:8080", "http://127.0.0.1:1", "http://[::1]:1"} {
		if err := validateAPIURL(u); err != nil {
			t.Errorf("%s refused: %v", u, err)
		}
	}
	home(t)
	save(t, Config{APIURL: "http://example.com", Token: "tok", TokenAPIURL: "http://example.com"})
	if err := checkBeforeRequest(&cobra.Command{Use: "list"}); err == nil || !strings.Contains(err.Error(), "https://") {
		t.Errorf("a command ran with api_url http://example.com (err=%v)", err)
	}
	apiURLOverride = "http://example.com"
	if err := checkBeforeRequest(&cobra.Command{Use: "list"}); err == nil {
		t.Errorf("a command ran with --api-url http://example.com")
	}
	if err := doLogin("http://example.com", "e@example.invalid", "pw"); err == nil || !strings.Contains(err.Error(), "https://") {
		t.Errorf("login to http://example.com was attempted (err=%v)", err)
	}
}

func TestTheTokenGoesOnlyToTheServerThatIssuedIt(t *testing.T) {
	_, aURL, a := serve(t, "127.0.0.1", jsonOK)
	_, bURL, b := serve(t, "localhost", jsonOK)
	home(t)
	save(t, Config{APIURL: aURL, Token: "tok", TokenAPIURL: aURL})
	if err := doRequest("GET", "/v1/api-keys", nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := a.all(); len(got) != 1 || got[0].auth != "Bearer tok" {
		t.Errorf("the issuer got %+v", got)
	}
	apiURLOverride = bURL
	err := checkBeforeRequest(&cobra.Command{Use: "list"})
	if err == nil || !strings.Contains(err.Error(), "sign in there") {
		t.Errorf("--api-url to another server: err=%v", err)
	}
	_ = doRequest("GET", "/v1/api-keys", nil, nil)
	for _, r := range b.all() {
		if r.auth != "" {
			t.Errorf("another server got %q", r.auth)
		}
	}
}

func TestAConfigWrittenBefore132IsBoundToItsAPIURL(t *testing.T) {
	_, aURL, a := serve(t, "127.0.0.1", jsonOK)
	_, bURL, b := serve(t, "localhost", jsonOK)
	d := home(t)
	os.MkdirAll(filepath.Join(d, ".tolvyn"), 0o700)                                              //nolint:errcheck
	os.WriteFile(configPath(), []byte(fmt.Sprintf(`{"api_url":%q,"token":"tok"}`, aURL)), 0o600) //nolint:errcheck
	_ = doRequest("GET", "/v1/api-keys", nil, nil)
	apiURLOverride = bURL
	_ = doRequest("GET", "/v1/api-keys", nil, nil)
	if got := a.all(); len(got) != 1 || got[0].auth != "Bearer tok" {
		t.Errorf("the configured server got %+v", got)
	}
	for _, r := range b.all() {
		if r.auth != "" {
			t.Errorf("another server got %q", r.auth)
		}
	}
}

func TestLoginSendsNoOldTokenAndRecordsItsServer(t *testing.T) {
	_, aURL, _ := serve(t, "127.0.0.1", jsonOK)
	_, bURL, b := serve(t, "localhost", jsonOK)
	home(t)
	save(t, Config{APIURL: aURL, Token: "tok-old", TokenAPIURL: aURL})
	if err := doLogin(bURL, "e@example.invalid", "pw"); err != nil {
		t.Fatal(err)
	}
	if got := b.all(); len(got) != 1 || got[0].auth != "" {
		t.Errorf("login sent %+v", got)
	}
	cfg, _ := loadConfig()
	if cfg.Token != "tok-new" || cfg.TokenAPIURL != bURL {
		t.Errorf("after login: token %q, issued by %q", cfg.Token, cfg.TokenAPIURL)
	}
}

func TestNoRedirectIsFollowed(t *testing.T) {
	_, otherURL, other := serve(t, "localhost", jsonOK)
	otherSame := strings.Replace(otherURL, "localhost", "127.0.0.1", 1)
	for name, to := range map[string]string{"another host": otherURL, "the same host, another port": otherSame} {
		_, aURL, _ := serve(t, "127.0.0.1", func(w http.ResponseWriter, q *http.Request) {
			http.Redirect(w, q, to+q.URL.Path, http.StatusTemporaryRedirect)
		})
		home(t)
		save(t, Config{APIURL: aURL, Token: "tok", TokenAPIURL: aURL})
		before := len(other.all())
		if err := doRequest("GET", "/v1/api-keys", nil, nil); err == nil {
			t.Errorf("%s: a redirect was followed without an error", name)
		}
		if n := len(other.all()) - before; n != 0 {
			t.Errorf("%s: the redirect target got %d request(s)", name, n)
		}
	}
}

func TestQueryValuesAreEscaped(t *testing.T) {
	u, _ := url.Parse("/x" + buildQueryParams("2026-01-01", "", "a&admin=true", "m#x"))
	if q := u.Query(); len(q) != 3 || q.Get("team_id") != "a&admin=true" || q.Get("model") != "m#x" || q.Get("from") != "2026-01-01" {
		t.Errorf("buildQueryParams: %v", q)
	}
	q, _ := url.ParseQuery(buildQueryString(map[string]string{"from": "2026-01-01&admin=true", "limit": "1", "empty": ""}))
	if len(q) != 2 || q.Get("from") != "2026-01-01&admin=true" {
		t.Errorf("buildQueryString: %v", q)
	}
}

// Every id put into a path goes through url.PathEscape: a source check, so a new command cannot forget it.
func TestNoIDGoesIntoAPathUnescaped(t *testing.T) {
	re := regexp.MustCompile(`"(/v1/[^"]*/)"\s*\+\s*([A-Za-z_.]+)`)
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			if !strings.HasPrefix(m[2], "url.PathEscape") {
				t.Errorf("%s: %q + %s", f, m[1], m[2])
			}
		}
	}
}

// No query value is joined into a URL by hand ("name="+value): every query goes through url.Values (BE-657, and the
// two sites the report did not list, usage export and models --provider).
func TestNoQueryValueIsJoinedByHand(t *testing.T) {
	re := regexp.MustCompile(`"[?&]?[a-z_]+="\s*\+`)
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, m := range re.FindAllString(string(b), -1) {
			t.Errorf("%s: %s", f, m)
		}
	}
}

func TestExportsArePrivateAndNeverOverwrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.csv")
	if _, err := writeExport(p, false, strings.NewReader("first")); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Errorf("a new export is %o", st.Mode().Perm())
	}
	if _, err := writeExport(p, false, strings.NewReader("second")); err == nil {
		t.Errorf("an existing file was overwritten without --force")
	}
	if b, _ := os.ReadFile(p); string(b) != "first" {
		t.Errorf("the existing file changed: %q", b)
	}
	os.Chmod(p, 0o644) //nolint:errcheck
	if _, err := writeExport(p, true, strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 || st.Size() != 0 {
		t.Errorf("with --force: mode %o, size %d", st.Mode().Perm(), st.Size())
	}
}

func TestResponsesAreCapped(t *testing.T) {
	_, aURL, _ := serve(t, "127.0.0.1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"x":"`+strings.Repeat("a", 4096)+`"}`) //nolint:errcheck
	})
	home(t)
	save(t, Config{APIURL: aURL})
	old := maxResponse
	maxResponse = 1024
	defer func() { maxResponse = old }()
	var v map[string]any
	if err := doRequest("GET", "/x", nil, &v); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("a response over the cap was read (err=%v)", err)
	}
}

func TestTailStopsWhenNoDataArrives(t *testing.T) {
	_, aURL, _ := serve(t, "127.0.0.1", func(w http.ResponseWriter, q *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		select {
		case <-q.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	home(t)
	save(t, Config{APIURL: aURL, Token: "tok", TokenAPIURL: aURL})
	old := tailIdle
	tailIdle = 300 * time.Millisecond
	defer func() { tailIdle = old }()
	start := time.Now()
	err := streamTail("/v1/stream")
	if err == nil || !strings.Contains(err.Error(), "no data") || time.Since(start) > 3*time.Second {
		t.Errorf("tail: err=%v after %v", err, time.Since(start))
	}
}

func TestTheConfigIsPrivateAfterEverySave(t *testing.T) {
	d := home(t)
	os.MkdirAll(filepath.Join(d, ".tolvyn"), 0o755)                                  //nolint:errcheck
	os.WriteFile(configPath(), []byte(`{"api_url":"https://api.tolvyn.io"}`), 0o644) //nolint:errcheck
	os.Chmod(filepath.Join(d, ".tolvyn"), 0o755)                                     //nolint:errcheck
	os.Chmod(configPath(), 0o644)                                                    //nolint:errcheck
	save(t, Config{APIURL: "https://api.tolvyn.io", Token: "tok"})
	if st, _ := os.Stat(configPath()); st.Mode().Perm() != 0o600 {
		t.Errorf("config.json is %o", st.Mode().Perm())
	}
	if st, _ := os.Stat(filepath.Join(d, ".tolvyn")); st.Mode().Perm() != 0o700 {
		t.Errorf("~/.tolvyn is %o", st.Mode().Perm())
	}
}

func TestInstallScriptRefusesAMismatchedChecksum(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "tolvyn-linux-amd64")
	os.WriteFile(bin, []byte("the binary"), 0o644)                            //nolint:errcheck
	sum := "4a5f3f3a2d7a4b2e8d3e0c9b5f2c1a7e6d5c4b3a29181716151413121110f0e0" // not its sum
	good := strings.TrimSpace(run(t, dir, "sha256sum tolvyn-linux-amd64 | cut -d' ' -f1"))
	verify := func(sums string) error {
		os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sums), 0o644) //nolint:errcheck
		cmd := exec.Command("sh", "-c", `TOLVYN_INSTALL_LIB=1 . "$1" && verify_checksum "$2" "$3" tolvyn-linux-amd64`, "sh",
			filepath.Join("..", "..", "install.sh"), bin, filepath.Join(dir, "SHA256SUMS"))
		return cmd.Run()
	}
	if err := verify(good + "  tolvyn-linux-amd64\n"); err != nil {
		t.Errorf("a matching sum was refused: %v", err)
	}
	if err := verify(sum + "  tolvyn-linux-amd64\n"); err == nil {
		t.Errorf("a wrong sum was accepted")
	}
	if err := verify(good + "  tolvyn-darwin-arm64\n"); err == nil {
		t.Errorf("a sums file without the asset was accepted")
	}
}

// BE-501: the installed binary is 0755 (root-owned when sudo is used), not the 0711 of mktemp's file left owned by
// the installing user.
func TestInstallScriptInstallsTheBinary0755(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "downloaded")
	os.WriteFile(src, []byte("the binary"), 0o600) //nolint:errcheck
	dest := filepath.Join(dir, "bin")
	os.MkdirAll(dest, 0o755) //nolint:errcheck
	cmd := exec.Command("sh", "-c", `TOLVYN_INSTALL_LIB=1 . "$1" && install_binary "$2" "$3" tolvyn`, "sh",
		filepath.Join("..", "..", "install.sh"), src, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install_binary: %v: %s", err, out)
	}
	if st, err := os.Stat(filepath.Join(dest, "tolvyn")); err != nil || st.Mode().Perm() != 0o755 {
		t.Errorf("the installed binary: %v, mode %v", err, st)
	}
}

func run(t *testing.T, dir, script string) string {
	t.Helper()
	cmd := exec.Command("sh", "-c", script)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
