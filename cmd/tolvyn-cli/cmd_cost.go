package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	costFrom  string
	costTo    string
	costTeam  string
	costModel string
	costBy    string
)

var cmdCost = &cobra.Command{
	Use:   "cost",
	Short: "Show spend summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}

		if costBy == "user" {
			return runCostByUser()
		}
		if costBy == "customer" {
			return runCostByCustomer()
		}

		q := buildQueryParams(costFrom, costTo, costTeam, costModel)
		path := "/v1/usage/summary" + q

		if flagJSON {
			raw, _, err := doRequestRaw("GET", path, nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var summary struct {
			TotalCostUSD    string `json:"total_cost_usd"`
			TotalRequests   int64  `json:"total_requests"`
			TotalTokensIn   int64  `json:"total_tokens_input"`
			TotalTokensOut  int64  `json:"total_tokens_output"`
			TopModels       []struct {
				ModelID  string `json:"model_id"`
				Requests int64  `json:"requests"`
				CostUSD  string `json:"cost_usd"`
				CostMicro int64 `json:"cost_microdollars"`
			} `json:"top_models"`
			TopTeams []struct {
				TeamID   string `json:"team_id"`
				Requests int64  `json:"requests"`
				CostUSD  string `json:"cost_usd"`
				CostMicro int64 `json:"cost_microdollars"`
			} `json:"top_teams"`
		}

		// Also fetch overall total cost for percentages.
		if err := doRequest("GET", path, nil, &summary); err != nil {
			return err
		}

		period := "all time"
		if costFrom != "" || costTo != "" {
			from := costFrom
			if from == "" {
				from = "—"
			}
			to := costTo
			if to == "" {
				to = "now"
			}
			period = from + " to " + to
		}

		avgCost := "$0.0000"
		if summary.TotalRequests > 0 {
			// Parse total cost
			var totalMicro int64
			for _, m := range summary.TopModels {
				totalMicro += m.CostMicro
			}
			// The summary has total_cost_usd but we need microdollars for avg.
			// Use a simple parse.
		}
		_ = avgCost

		fmt.Printf("%-20s %s\n", "Period:", period)
		fmt.Printf("%-20s %s\n", "Total Spend:", green(summary.TotalCostUSD))
		fmt.Printf("%-20s %s\n", "Total Requests:", commaInt(summary.TotalRequests))

		if summary.TotalRequests > 0 {
			// Parse cost and compute avg.
			var totalF float64
			fmt.Sscanf(strings.TrimPrefix(summary.TotalCostUSD, "$"), "%f", &totalF)
			avg := totalF / float64(summary.TotalRequests)
			fmt.Printf("%-20s %s\n", "Avg Cost/Req:", green(fmt.Sprintf("$%.4f", avg)))
		}

		if len(summary.TopModels) > 0 {
			fmt.Println()
			fmt.Println(cyan("Top Models:"))
			// Compute total for percentage.
			var total float64
			fmt.Sscanf(strings.TrimPrefix(summary.TotalCostUSD, "$"), "%f", &total)

			for _, m := range summary.TopModels {
				var cost float64
				fmt.Sscanf(strings.TrimPrefix(m.CostUSD, "$"), "%f", &cost)
				pct := 0.0
				if total > 0 {
					pct = cost / total * 100
				}
				fmt.Printf("  %-20s %s  %5.1f%%  %s reqs\n",
					m.ModelID,
					green(fmt.Sprintf("%-10s", m.CostUSD)),
					pct,
					commaInt(m.Requests),
				)
			}
		}

		if len(summary.TopTeams) > 0 {
			fmt.Println()
			fmt.Println(cyan("Top Teams:"))
			var total float64
			fmt.Sscanf(strings.TrimPrefix(summary.TotalCostUSD, "$"), "%f", &total)
			for _, t := range summary.TopTeams {
				var cost float64
				fmt.Sscanf(strings.TrimPrefix(t.CostUSD, "$"), "%f", &cost)
				pct := 0.0
				if total > 0 {
					pct = cost / total * 100
				}
				fmt.Printf("  %-20s %s  %5.1f%%\n",
					t.TeamID,
					green(fmt.Sprintf("%-12s", t.CostUSD)),
					pct,
				)
			}
		}
		return nil
	},
}

func init() {
	cmdCost.Flags().StringVar(&costFrom, "from", "", "Start date (YYYY-MM-DD)")
	cmdCost.Flags().StringVar(&costTo, "to", "", "End date (YYYY-MM-DD)")
	cmdCost.Flags().StringVar(&costTeam, "team", "", "Filter by team ID")
	cmdCost.Flags().StringVar(&costModel, "model", "", "Filter by model")
	cmdCost.Flags().StringVar(&costBy, "by", "", "Group by dimension: user, customer")
}

func runCostByCustomer() error {
	q := buildQueryParams(costFrom, costTo, "", "")
	path := "/v1/usage/by-end-customer" + q

	if flagJSON {
		raw, _, err := doRequestRaw("GET", path, nil)
		if err != nil {
			return err
		}
		printJSON(raw)
		return nil
	}

	var resp struct {
		Customers []struct {
			Customer     string `json:"customer"`
			RequestCount int64  `json:"request_count"`
			TotalCostUSD string `json:"total_cost_usd"`
			AvgCostUSD   string `json:"avg_cost_usd"`
			TopModel     string `json:"top_model"`
			LastSeen     string `json:"last_seen"`
		} `json:"customers"`
		TotalCostUSD string `json:"total_cost_usd"`
	}
	if err := doRequest("GET", path, nil, &resp); err != nil {
		return err
	}

	if len(resp.Customers) == 0 {
		fmt.Println("No end-customer data. Add X-Tolvyn-End-Customer headers to your API calls.")
		return nil
	}

	fmt.Printf("%-30s  %-10s  %-12s  %-10s  %-20s  %s\n",
		"Customer", "Requests", "Total Cost", "Avg Cost", "Top Model", "Last Active")
	fmt.Println(strings.Repeat("─", 100))

	limit := 20
	for i, c := range resp.Customers {
		if i >= limit {
			break
		}
		lastActive := "—"
		if c.LastSeen != "" {
			lastActive = daysAgoLabel(c.LastSeen)
		}
		fmt.Printf("%-30s  %-10s  %-12s  %-10s  %-20s  %s\n",
			c.Customer,
			commaInt(c.RequestCount),
			green(fmt.Sprintf("%-12s", c.TotalCostUSD)),
			c.AvgCostUSD,
			c.TopModel,
			lastActive,
		)
	}

	fmt.Println()
	fmt.Printf("%-20s %s\n", "Total AI Spend:", green(resp.TotalCostUSD))
	return nil
}

func daysAgoLabel(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return "—"
	}
	days := int(time.Since(t).Hours() / 24)
	if days == 0 {
		return "today"
	}
	return fmt.Sprintf("%dd ago", days)
}

func runCostByUser() error {
	q := buildQueryParams(costFrom, costTo, "", "")
	path := "/v1/usage/by-user" + q

	if flagJSON {
		raw, _, err := doRequestRaw("GET", path, nil)
		if err != nil {
			return err
		}
		printJSON(raw)
		return nil
	}

	var resp struct {
		Users []struct {
			User         string  `json:"user"`
			RequestCount int64   `json:"request_count"`
			TotalCostUSD string  `json:"total_cost_usd"`
			AvgCostUSD   string  `json:"avg_cost_usd"`
			TopModel     string  `json:"top_model"`
			RetryRatePct float64 `json:"retry_rate_pct"`
		} `json:"users"`
		PeriodStart string `json:"period_start"`
		PeriodEnd   string `json:"period_end"`
	}
	if err := doRequest("GET", path, nil, &resp); err != nil {
		return err
	}

	if len(resp.Users) == 0 {
		fmt.Println("No user data. Add X-Tolvyn-User headers to your API calls.")
		return nil
	}

	fmt.Printf("%-35s  %-10s  %-10s  %-10s  %-20s  %s\n",
		"User", "Requests", "Spend", "Avg Cost", "Top Model", "Retry Rate")
	fmt.Println(strings.Repeat("─", 100))

	limit := 20
	for i, u := range resp.Users {
		if i >= limit {
			break
		}
		retryColor := green
		if u.RetryRatePct > 10 {
			retryColor = red
		} else if u.RetryRatePct >= 2 {
			retryColor = yellow
		}
		fmt.Printf("%-35s  %-10s  %-10s  %-10s  %-20s  %s\n",
			u.User,
			commaInt(u.RequestCount),
			green(fmt.Sprintf("%-10s", u.TotalCostUSD)),
			u.AvgCostUSD,
			u.TopModel,
			retryColor(fmt.Sprintf("%.1f%%", u.RetryRatePct)),
		)
	}
	return nil
}

func buildQueryParams(from, to, team, model string) string {
	var parts []string
	if from != "" {
		parts = append(parts, "from="+from)
	}
	if to != "" {
		parts = append(parts, "to="+to)
	}
	if team != "" {
		parts = append(parts, "team_id="+team)
	}
	if model != "" {
		parts = append(parts, "model="+model)
	}
	if len(parts) == 0 {
		return ""
	}
	return "?" + strings.Join(parts, "&")
}
