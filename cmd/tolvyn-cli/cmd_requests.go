package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	reqTeam  string
	reqModel string
	reqLimit int
	reqFrom  string
	reqTo    string
)

var cmdRequests = &cobra.Command{
	Use:   "requests",
	Short: "Show request history",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if reqLimit > 100 {
			reqLimit = 100
		}

		q := buildQueryParams(reqFrom, reqTo, reqTeam, reqModel)
		sep := "?"
		if q != "" {
			sep = "&"
			q += sep
		} else {
			q = "?"
		}
		q += fmt.Sprintf("limit=%d", reqLimit)
		path := "/v1/usage/requests" + q

		if flagJSON {
			raw, _, err := doRequestRaw("GET", path, nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var result struct {
			Data []struct {
				ID             string     `json:"id"`
				Provider       string     `json:"provider"`
				ModelID        string     `json:"model_id"`
				TokensInput    int        `json:"tokens_input"`
				TokensOutput   int        `json:"tokens_output"`
				CostUSD        string     `json:"cost_usd"`
				LatencyTotalMs *int       `json:"latency_total_ms"`
				TeamID         *string    `json:"team_id"`
				ServiceName    *string    `json:"service_name"`
				StatusCode     int        `json:"status_code"`
				CreatedAt      time.Time  `json:"created_at"`
			} `json:"data"`
			Total int `json:"total"`
		}
		if err := doRequest("GET", path, nil, &result); err != nil {
			return err
		}

		header := fmt.Sprintf("%-17s | %-16s | %-14s | %7s | %8s | %7s",
			"TIME", "TEAM/SERVICE", "MODEL", "TOKENS", "COST", "LATENCY")
		fmt.Println(cyan(header))

		for _, r := range result.Data {
			ts := r.CreatedAt.Format("2006-01-02 15:04")
			teamSvc := ""
			if r.TeamID != nil && *r.TeamID != "" {
				teamSvc = *r.TeamID
			}
			if r.ServiceName != nil && *r.ServiceName != "" {
				if teamSvc != "" {
					teamSvc += "/" + *r.ServiceName
				} else {
					teamSvc = *r.ServiceName
				}
			}
			if len(teamSvc) > 16 {
				teamSvc = teamSvc[:16]
			}
			model := r.ModelID
			if len(model) > 14 {
				model = model[:14]
			}
			tokens := int64(r.TokensInput + r.TokensOutput)
			latStr := "—"
			if r.LatencyTotalMs != nil && *r.LatencyTotalMs > 0 {
				latStr = fmt.Sprintf("%dms", *r.LatencyTotalMs)
			}

			if useColor {
				plain := fmt.Sprintf("%-17s | %-16s | %-14s | %7s | ", ts, teamSvc, model, commaInt(tokens))
				fmt.Printf("%s%s | %7s\n", plain, green(fmt.Sprintf("%8s", r.CostUSD)), latStr)
			} else {
				fmt.Printf("%-17s | %-16s | %-14s | %7s | %8s | %7s\n",
					ts, teamSvc, model, commaInt(tokens), r.CostUSD, latStr)
			}
		}
		return nil
	},
}

func init() {
	cmdRequests.Flags().StringVar(&reqTeam, "team", "", "Filter by team ID")
	cmdRequests.Flags().StringVar(&reqModel, "model", "", "Filter by model")
	cmdRequests.Flags().IntVar(&reqLimit, "limit", 20, "Number of rows (max 100)")
	cmdRequests.Flags().StringVar(&reqFrom, "from", "", "Start date (YYYY-MM-DD)")
	cmdRequests.Flags().StringVar(&reqTo, "to", "", "End date (YYYY-MM-DD)")
}
