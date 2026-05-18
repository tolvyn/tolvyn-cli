package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	auditFrom   string
	auditTo     string
	auditLast   string
	auditFormat string
	auditLimit  int
)

var cmdAudit = &cobra.Command{
	Use:   "audit",
	Short: "View and export audit log",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}

		params := map[string]string{}

		if auditLast != "" {
			dur, err := parseDuration(auditLast)
			if err != nil {
				return fmt.Errorf("invalid --last value %q: use format like 24h, 7d, 30d", auditLast)
			}
			params["from"] = time.Now().Add(-dur).Format(time.RFC3339)
		}
		if auditFrom != "" {
			params["from"] = auditFrom
		}
		if auditTo != "" {
			params["to"] = auditTo
		}
		params["limit"] = strconv.Itoa(auditLimit)

		path := "/v1/audit?" + buildQueryString(params)

		if auditFormat == "csv" {
			return doAuditCSV(path)
		}

		if flagJSON {
			raw, _, err := doRequestRaw("GET", path, nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var result struct {
			Logs []struct {
				CreatedAt    string `json:"created_at"`
				ActorType    string `json:"actor_type"`
				ActorID      string `json:"actor_id"`
				Action       string `json:"action"`
				ResourceType string `json:"resource_type"`
				ResourceID   string `json:"resource_id"`
				IPAddress    string `json:"ip_address"`
			} `json:"logs"`
			Total int `json:"total"`
		}

		if err := doRequest("GET", path, nil, &result); err != nil {
			return err
		}

		if len(result.Logs) == 0 {
			fmt.Println("No audit log entries found.")
			return nil
		}

		fmt.Printf("%-24s %-8s %-36s %-32s %s\n",
			cyan("TIME"), cyan("ACTOR"), cyan("ACTOR ID"), cyan("ACTION"), cyan("IP"))
		fmt.Println(strings.Repeat("─", 120))
		for _, log := range result.Logs {
			t, _ := time.Parse(time.RFC3339, log.CreatedAt)
			fmt.Printf("%-24s %-8s %-36s %-32s %s\n",
				t.Local().Format("2006-01-02 15:04:05"),
				log.ActorType,
				truncate(log.ActorID, 36),
				log.Action,
				log.IPAddress,
			)
		}
		fmt.Printf("\n%d entries\n", result.Total)
		return nil
	},
}

// parseDuration parses "24h", "7d", "30d" into time.Duration.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// buildQueryString joins a map of string params into a URL query string.
func buildQueryString(params map[string]string) string {
	var parts []string
	for k, v := range params {
		if v != "" {
			parts = append(parts, k+"="+v)
		}
	}
	return strings.Join(parts, "&")
}

// truncate shortens s to max runes, appending … if trimmed.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// doAuditCSV fetches the audit endpoint with Accept: text/csv and streams to stdout.
func doAuditCSV(path string) error {
	req, err := http.NewRequest("GET", getAPIURL()+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+getToken())
	req.Header.Set("Accept", "text/csv")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	_, err = io.Copy(os.Stdout, resp.Body)
	return err
}

func init() {
	cmdAudit.Flags().StringVar(&auditFrom, "from", "", "Start date (RFC3339 or YYYY-MM-DD)")
	cmdAudit.Flags().StringVar(&auditTo, "to", "", "End date (RFC3339 or YYYY-MM-DD)")
	cmdAudit.Flags().StringVar(&auditLast, "last", "", "Show last N hours/days (e.g. 24h, 7d, 30d)")
	cmdAudit.Flags().StringVar(&auditFormat, "format", "table", "Output format: table or csv")
	cmdAudit.Flags().IntVar(&auditLimit, "limit", 100, "Max entries to return")
}
