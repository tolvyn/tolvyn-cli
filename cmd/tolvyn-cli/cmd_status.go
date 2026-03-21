package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var cmdStatus = &cobra.Command{
	Use:   "status",
	Short: "Check API connectivity and auth status",
	RunE: func(cmd *cobra.Command, args []string) error {
		apiURL := getAPIURL()

		if flagJSON {
			raw, _, err := doRequestRaw("GET", "/health", nil)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: "+err.Error())
				os.Exit(1)
			}
			printJSON(raw)
			return nil
		}

		start := time.Now()
		req, _ := http.NewRequest("GET", apiURL+"/health", nil)
		resp, err := httpClient.Do(req)
		latency := time.Since(start)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%-10s%s  %s\n", "API:", apiURL, red("[ERROR]"))
			fmt.Fprintf(os.Stderr, "Error: could not connect to API at %s\n", apiURL)
			os.Exit(1)
		}
		defer resp.Body.Close()

		var health struct {
			Status  string `json:"status"`
			DB      string `json:"db"`
			Version string `json:"version"`
		}
		json.NewDecoder(resp.Body).Decode(&health) //nolint:errcheck

		statusLabel := green("[OK]")
		if health.Status != "ok" || resp.StatusCode >= 400 {
			statusLabel = red("[ERROR]")
		}
		dbLabel := health.DB
		switch dbLabel {
		case "ok":
			dbLabel = green("ok")
		case "error":
			dbLabel = red("error")
		}

		fmt.Printf("%-10s%s  %s\n", "API:", apiURL, statusLabel)
		if dbLabel != "" {
			fmt.Printf("%-10s%s\n", "Database:", dbLabel)
		}
		if health.Version != "" {
			fmt.Printf("%-10s%s\n", "Version:", health.Version)
		}
		fmt.Printf("%-10s%dms\n", "Latency:", latency.Milliseconds())

		if getToken() != "" {
			var acct struct {
				Email string `json:"email"`
			}
			if err := doRequest("GET", "/v1/account", nil, &acct); err == nil {
				fmt.Printf("%-10s%s\n", "Auth:", green("Authenticated as: "+acct.Email))
			} else {
				fmt.Printf("%-10s%s\n", "Auth:", yellow("token present but invalid — run 'tolvyn login'"))
			}
		} else {
			fmt.Printf("%-10s%s\n", "Auth:", yellow("not authenticated — run 'tolvyn login'"))
		}

		if health.Status != "ok" || resp.StatusCode >= 400 {
			os.Exit(1)
		}
		return nil
	},
}

func errOut() *os.File { return os.Stderr }
