package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var cmdSavings = &cobra.Command{
	Use:   "savings",
	Short: "View and dismiss savings findings",
}

var (
	savingsMinUSD float64
)

var cmdSavingsList = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List active savings findings",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if flagJSON {
			raw, _, err := doRequestRaw("GET", "/v1/savings", nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var result struct {
			Findings []struct {
				ID                 string    `json:"id"`
				FindingType        string    `json:"finding_type"`
				ModelID            string    `json:"model_id"`
				ServiceName        string    `json:"service_name"`
				CurrentCostUSD     string    `json:"current_cost_usd"`
				OptimizedCostUSD   string    `json:"optimized_cost_usd"`
				SavingsUSD         string    `json:"savings_usd"`
				RecommendationText string    `json:"recommendation_text"`
				CreatedAt          time.Time `json:"created_at"`
			} `json:"findings"`
			TotalSavingsUSD string `json:"total_savings_usd"`
		}
		if err := doRequest("GET", "/v1/savings", nil, &result); err != nil {
			return err
		}

		// Client-side min-savings filter.
		rows := result.Findings
		if savingsMinUSD > 0 {
			filtered := rows[:0]
			for _, r := range rows {
				if savAsFloat(r.SavingsUSD) >= savingsMinUSD {
					filtered = append(filtered, r)
				}
			}
			rows = filtered
		}

		if len(rows) == 0 {
			fmt.Println("No savings findings.")
			return nil
		}

		fmt.Printf("%-38s  %-26s  %-22s  %-18s  %-12s  %s\n",
			cyan("ID"), cyan("TYPE"), cyan("MODEL"), cyan("SERVICE"), cyan("SAVINGS/MO"), cyan("RECOMMENDATION"))
		for _, r := range rows {
			model := r.ModelID
			if model == "" {
				model = "—"
			}
			svc := r.ServiceName
			if svc == "" {
				svc = "—"
			}
			rec := r.RecommendationText
			if len(rec) > 60 {
				rec = rec[:60] + "…"
			}
			fmt.Printf("%-38s  %-26s  %-22s  %-18s  %-12s  %s\n",
				r.ID, truncate(r.FindingType, 26),
				truncate(model, 22), truncate(svc, 18),
				green(r.SavingsUSD), rec,
			)
		}
		fmt.Printf("\nTotal potential savings: %s\n", green(result.TotalSavingsUSD))
		return nil
	},
}

var cmdSavingsDismiss = &cobra.Command{
	Use:   "dismiss <id>",
	Short: "Dismiss a savings finding (one-way)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		id := args[0]
		if err := doRequest("POST", "/v1/savings/"+id+"/dismiss", nil, nil); err != nil {
			return err
		}
		fmt.Printf("%s Finding %s dismissed.\n", green("✓"), id)
		return nil
	},
}

// savAsFloat parses "$1.2345" or "1.2345" into a float64 (0 on parse failure).
func savAsFloat(s string) float64 {
	s = strings.TrimSpace(strings.TrimPrefix(s, "$"))
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func init() {
	cmdSavingsList.Flags().Float64Var(&savingsMinUSD, "min-savings", 0, "Hide findings below this monthly USD threshold")

	cmdSavings.AddCommand(cmdSavingsList, cmdSavingsDismiss)
}
