package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

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

func getToken() string {
	cfg, err := loadConfig()
	if err != nil {
		return ""
	}
	return cfg.Token
}

// doRequest performs an authenticated HTTP request and decodes the response into v.
func doRequest(method, path string, body any, v any) error {
	return doRequestAt(getAPIURL(), method, path, body, v)
}

func doRequestAt(baseURL, method, path string, body any, v any) error {
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
	if tok := getToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not connect to API at %s. Run 'tolvyn status' to diagnose", baseURL)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		return fmt.Errorf("not authenticated. Run 'tolvyn login'")
	}
	if resp.StatusCode == 404 {
		var e struct{ Message string `json:"message"` }
		json.Unmarshal(respBytes, &e) //nolint:errcheck
		if e.Message != "" {
			return fmt.Errorf("not found — %s", e.Message)
		}
		return fmt.Errorf("not found — resource does not exist")
	}
	if resp.StatusCode >= 300 {
		var e struct{ Message string `json:"message"` }
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

// doRequestRaw returns the raw response bytes (for --json flag).
func doRequestRaw(method, path string, body any) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, getAPIURL()+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("could not connect to API at %s. Run 'tolvyn status' to diagnose", getAPIURL())
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := getToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("could not connect to API at %s. Run 'tolvyn status' to diagnose", getAPIURL())
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
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
