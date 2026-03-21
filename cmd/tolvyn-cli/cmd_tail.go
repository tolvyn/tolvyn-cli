package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	tailTeam     string
	tailService  string
	tailModel    string
	tailMinCost  float64
	tailNoAlerts bool
)

var cmdTail = &cobra.Command{
	Use:   "tail",
	Short: "Stream live AI requests in real-time",
	Long:  "Connect to the SSE live-tail endpoint and print requests as they happen.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}

		// Build SSE query params (server filters by team/service/model).
		q := url.Values{}
		if tailTeam != "" {
			q.Set("team", tailTeam)
		}
		if tailService != "" {
			q.Set("service", tailService)
		}
		if tailModel != "" {
			q.Set("model", tailModel)
		}
		// minCost: pass as microdollars so server can store it (even if not filtered server-side).
		if tailMinCost > 0 {
			q.Set("min_cost", fmt.Sprintf("%d", int64(tailMinCost*1_000_000)))
		}
		path := "/v1/stream/tail"
		if len(q) > 0 {
			path += "?" + q.Encode()
		}

		// Print header.
		printTailHeader()

		// Ctrl+C handler.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Println()
			fmt.Println("Stream disconnected.")
			os.Exit(0)
		}()

		// Retry loop.
		const maxRetries = 3
		retries := 0
		for {
			err := streamTail(path)
			if err != nil {
				retries++
				if retries >= maxRetries {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Fprintf(os.Stderr, "Connection lost: %v — retrying in 5s (%d/%d)...\n",
					err, retries, maxRetries)
				time.Sleep(5 * time.Second)
			}
		}
	},
}

func init() {
	cmdTail.Flags().StringVar(&tailTeam, "team", "", "Filter by team name")
	cmdTail.Flags().StringVar(&tailService, "service", "", "Filter by service name")
	cmdTail.Flags().StringVar(&tailModel, "model", "", "Filter by model (substring match)")
	cmdTail.Flags().Float64Var(&tailMinCost, "min-cost", 0, "Minimum cost in USD to display")
	cmdTail.Flags().BoolVar(&tailNoAlerts, "no-alerts", false, "Suppress alert events")
}

// printTailHeader prints the fixed-width column header.
func printTailHeader() {
	cols := fmt.Sprintf("%-8s | %-22s | %-16s | %8s | %8s | %8s",
		"TIME", "TEAM/SERVICE", "MODEL", "TOKENS", "COST", "LATENCY")
	fmt.Println(cyan(cols))
	sep := strings.Repeat("─", 9) + "┼" +
		strings.Repeat("─", 24) + "┼" +
		strings.Repeat("─", 18) + "┼" +
		strings.Repeat("─", 10) + "┼" +
		strings.Repeat("─", 10) + "┼" +
		strings.Repeat("─", 9)
	fmt.Println(cyan(sep))
}

// streamTail opens the SSE connection and reads events until error or EOF.
func streamTail(path string) error {
	req, err := http.NewRequest("GET", getAPIURL()+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+getToken())
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Use a client without a timeout for long-lived streaming connections.
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not connect to API at %s. Run 'tolvyn status' to diagnose", getAPIURL())
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("not authenticated. Run 'tolvyn login'")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		var event struct {
			Type         string `json:"type"`
			Timestamp    string `json:"timestamp"`
			Team         string `json:"team"`
			Service      string `json:"service"`
			Model        string `json:"model"`
			TokensInput  int64  `json:"tokens_input"`
			TokensOutput int64  `json:"tokens_output"`
			CostUSD      string `json:"cost_usd"`
			LatencyMs    int    `json:"latency_ms"`
			Severity     string `json:"severity"`
			Message      string `json:"message"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "heartbeat":
			// silently ignored
			continue

		case "alert":
			if !tailNoAlerts {
				scope := event.Message
				if scope == "" {
					scope = "(no details)"
				}
				fmt.Println(red("[ALERT] " + scope))
			}
			continue
		}

		// Regular request event — apply client-side filters.
		if tailModel != "" && !strings.Contains(event.Model, tailModel) {
			continue
		}
		if tailMinCost > 0 {
			var cost float64
			trimmed := strings.TrimPrefix(event.CostUSD, "$")
			fmt.Sscanf(trimmed, "%f", &cost)
			if cost < tailMinCost {
				continue
			}
		}

		printTailEvent(
			event.Timestamp, event.Team, event.Service, event.Model,
			event.TokensInput+event.TokensOutput,
			event.CostUSD, event.LatencyMs,
		)
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return err
	}
	return fmt.Errorf("stream ended")
}

// printTailEvent prints one request event in fixed-width columns.
func printTailEvent(ts, team, service, model string, tokens int64, costUSD string, latencyMs int) {
	teamSvc := team
	if service != "" {
		if teamSvc != "" {
			teamSvc = team + "/" + service
		} else {
			teamSvc = service
		}
	}
	if len(teamSvc) > 22 {
		teamSvc = teamSvc[:22]
	}
	if len(model) > 16 {
		model = model[:16]
	}

	tokStr := commaInt(tokens)
	latStr := "—"
	if latencyMs > 0 {
		latStr = fmt.Sprintf("%dms", latencyMs)
	}

	if useColor {
		// Print cost column in green, everything else normal.
		pre := fmt.Sprintf("%-8s | %-22s | %-16s | %8s | ", ts, teamSvc, model, tokStr)
		post := fmt.Sprintf(" | %8s", latStr)
		fmt.Printf("%s%s%s\n", pre, green(fmt.Sprintf("%8s", costUSD)), post)
	} else {
		fmt.Printf("%-8s | %-22s | %-16s | %8s | %8s | %8s\n",
			ts, teamSvc, model, tokStr, costUSD, latStr)
	}
}

// commaInt formats an int64 with thousands separators.
func commaInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}
