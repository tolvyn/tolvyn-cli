package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// noRedirect refuses every redirect: the API never answers one, and Go keeps the Authorization header on a hop to the
// same host on another port or over http (BE-656).
func noRedirect(req *http.Request, _ []*http.Request) error {
	return fmt.Errorf("the API answered a redirect to %s; it was not followed, so your sign-in was not sent there", req.URL.Host)
}

var httpClient = &http.Client{Timeout: 30 * time.Second, CheckRedirect: noRedirect}

// maxResponse caps a response read into memory; exports stream instead (doRequestStream).
var maxResponse int64 = 16 << 20

// apiURL returns the effective API URL (flag override or config).
var apiURLOverride string

func getAPIURL() string {
	if apiURLOverride != "" {
		return apiURLOverride
	}
	cfg, err := loadConfig()
	if err != nil || cfg.APIURL == "" {
		return defaultAPIURL
	}
	return cfg.APIURL
}

// validateAPIURL refuses an API URL that is not https, except http to this machine (localhost, 127.0.0.1, [::1]):
// anywhere else the token, a password at sign-in and provider keys would cross the network readable (BE-612).
func validateAPIURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("the API URL is not a valid URL: %s", raw)
	}
	h := u.Hostname()
	if u.Scheme == "https" || (u.Scheme == "http" && (h == "localhost" || h == "127.0.0.1" || h == "::1")) {
		return nil
	}
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	return fmt.Errorf("the API URL must start with https:// (http:// is accepted only for localhost): %s", u.String())
}

// origin is scheme://host[:port], or "" when the URL does not parse.
func origin(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// tokenFor returns the stored token if it was issued by base's server, else "": a token goes only to the server that
// issued it (loadConfig binds a config written before 1.3.1 to its api_url).
func tokenFor(base string) string {
	cfg, err := loadConfig()
	if err != nil || cfg.Token == "" {
		return ""
	}
	if o := origin(base); o != "" && o == origin(cfg.TokenAPIURL) {
		return cfg.Token
	}
	return ""
}

func getToken() string { return tokenFor(getAPIURL()) }

// tokenElsewhere says so when a sign-in exists for another server than the one a command would call.
func tokenElsewhere(base string) error {
	cfg, err := loadConfig()
	if err != nil || cfg.Token == "" || tokenFor(base) != "" {
		return nil
	}
	return fmt.Errorf("this sign-in is for %s. To use %s, sign in there: tolvyn login --api-url %s",
		origin(cfg.TokenAPIURL), origin(base), origin(base))
}

// checkBeforeRequest runs before every command that calls the API: the URL must pass the rule, and a sign-in for
// another server is never sent (BE-612, BE-656).
func checkBeforeRequest(cmd *cobra.Command) error {
	switch cmd.Name() {
	case "version", "login", "init", "logout", "help", "tolvyn", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
		return nil
	}
	base := getAPIURL()
	if err := validateAPIURL(base); err != nil {
		return err
	}
	return tokenElsewhere(base)
}

// readCapped reads a response body up to maxResponse bytes.
func readCapped(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxResponse+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxResponse {
		return nil, fmt.Errorf("the API's response is too large (over %d bytes)", maxResponse)
	}
	return b, nil
}

// doRequest performs an authenticated HTTP request and decodes the response into v.
func doRequest(method, path string, body any, v any) error {
	return doRequestAt(getAPIURL(), method, path, body, v)
}

func doRequestAt(baseURL, method, path string, body any, v any) error {
	if err := validateAPIURL(baseURL); err != nil {
		return err
	}
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("could not connect to API at %s. Run 'tolvyn status' to diagnose", baseURL)
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := tokenFor(baseURL); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not connect to API at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	respBytes, err := readCapped(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == 401 {
		return fmt.Errorf("not authenticated. Run 'tolvyn login'")
	}
	if resp.StatusCode == 404 {
		var e struct {
			Message string `json:"message"`
		}
		json.Unmarshal(respBytes, &e) //nolint:errcheck
		if e.Message != "" {
			return fmt.Errorf("not found — %s", e.Message)
		}
		return fmt.Errorf("not found — resource does not exist")
	}
	if resp.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		json.Unmarshal(respBytes, &e) //nolint:errcheck
		if e.Message != "" {
			return fmt.Errorf("%s", e.Message)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if v != nil {
		if err := json.Unmarshal(respBytes, v); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// exportIdle stops an export after this long without data. An export has no total time limit: httpClient's 30 s
// would cut a large one off part-way.
var exportIdle = 60 * time.Second

var exportClient = &http.Client{CheckRedirect: noRedirect}

// doRequestStream returns the response to a GET for the caller to stream (an export): not read into memory, so not
// capped, and stopped only after exportIdle without data. A status other than 200 is an error, with the start of its
// body.
func doRequestStream(path string) (io.ReadCloser, error) {
	base := getAPIURL()
	if err := validateAPIURL(base); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	idle := time.AfterFunc(exportIdle, cancel)
	stop := func() { idle.Stop(); cancel() }
	req, err := http.NewRequestWithContext(ctx, "GET", base+path, nil)
	if err != nil {
		stop()
		return nil, fmt.Errorf("could not connect to API at %s. Run 'tolvyn status' to diagnose", base)
	}
	if tok := tokenFor(base); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := exportClient.Do(req)
	if err != nil {
		stop()
		return nil, fmt.Errorf("could not connect to API at %s: %w", base, err)
	}
	if resp.StatusCode != 200 {
		defer stop()
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, b)
	}
	return &idleBody{ReadCloser: resp.Body, idle: idle, ctx: ctx, stop: stop}, nil
}

// idleBody runs the export's idle timer only while a read waits on the API, so a slow reader of the output does not
// count as a quiet API.
type idleBody struct {
	io.ReadCloser
	idle *time.Timer
	ctx  context.Context
	stop func()
}

func (b *idleBody) Read(p []byte) (int, error) {
	b.idle.Reset(exportIdle)
	n, err := b.ReadCloser.Read(p)
	b.idle.Stop()
	if err != nil && err != io.EOF && b.ctx.Err() != nil {
		err = fmt.Errorf("the export stopped: no data from the API for %s", exportIdle)
	}
	return n, err
}

func (b *idleBody) Close() error {
	b.stop()
	return b.ReadCloser.Close()
}

// doRequestRaw returns the raw response bytes (for --json flag).
func doRequestRaw(method, path string, body any) ([]byte, int, error) {
	base := getAPIURL()
	if err := validateAPIURL(base); err != nil {
		return nil, 0, err
	}
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, base+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("could not connect to API at %s. Run 'tolvyn status' to diagnose", base)
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := tokenFor(base); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("could not connect to API at %s: %w", base, err)
	}
	defer resp.Body.Close()
	data, err := readCapped(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return data, resp.StatusCode, nil
}

// printJSON pretty-prints raw JSON to stdout.
func printJSON(data []byte) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		os.Stdout.Write(data) //nolint:errcheck
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v) //nolint:errcheck
}

// fatalf prints to stderr and exits 1.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
