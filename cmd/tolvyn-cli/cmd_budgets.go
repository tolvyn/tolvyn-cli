package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var cmdBudgets = &cobra.Command{
	Use:   "budgets",
	Short: "Manage budgets (list, create)",
}

var cmdBudgetsList = &cobra.Command{
	Use:   "list",
	Short: "List budgets",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if flagJSON {
			raw, _, err := doRequestRaw("GET", "/v1/budgets", nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var budgets []struct {
			ID             string  `json:"id"`
			ScopeType      string  `json:"scope_type"`
			ScopeID        *string `json:"scope_id"`
			AmountUSD      string  `json:"amount_usd"`
			CurrentSpendUSD string `json:"current_spend_usd"`
			UtilizationPct float64 `json:"utilization_pct"`
			Period         string  `json:"period"`
			Mode           string  `json:"mode"`
		}
		if err := doRequest("GET", "/v1/budgets", nil, &budgets); err != nil {
			return err
		}

		fmt.Printf("%-22s %-6s %-12s %-12s %-8s %-10s\n",
			cyan("SCOPE"), cyan("MODE"), cyan("LIMIT"), cyan("SPENT"), cyan("UTIL"), cyan("PERIOD"))

		for _, b := range budgets {
			scope := b.ScopeType
			if b.ScopeID != nil && *b.ScopeID != "" {
				scope += " (" + *b.ScopeID + ")"
			}
			if len(scope) > 22 {
				scope = scope[:22]
			}

			utilStr := fmt.Sprintf("%.1f%%", b.UtilizationPct)
			var utilColored string
			switch {
			case b.UtilizationPct >= 90:
				utilColored = red(utilStr)
			case b.UtilizationPct >= 75:
				utilColored = yellow(utilStr)
			default:
				utilColored = green(utilStr)
			}

			fmt.Printf("%-22s %-6s %-12s %-12s %-16s %-10s\n",
				scope, b.Mode, b.AmountUSD, b.CurrentSpendUSD, utilColored, b.Period)
		}
		return nil
	},
}

var (
	budgetScope   string
	budgetTeam    string
	budgetService string
	budgetAmount  float64
	budgetPeriod  string
	budgetMode    string
)

var cmdBudgetsCreate = &cobra.Command{
	Use:   "create",
	Short: "Create a budget",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if budgetAmount <= 0 {
			return fmt.Errorf("--amount is required and must be > 0")
		}

		body := map[string]any{
			"amount_usd": budgetAmount,
			"period":     budgetPeriod,
			"mode":       budgetMode,
		}
		switch budgetScope {
		case "team":
			body["scope_type"] = "team"
			body["scope_id"] = budgetTeam
		case "service":
			body["scope_type"] = "service"
			body["scope_id"] = budgetService
		default:
			body["scope_type"] = "organization"
		}

		if flagJSON {
			raw, _, err := doRequestRaw("POST", "/v1/budgets", body)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var resp map[string]any
		if err := doRequest("POST", "/v1/budgets", body, &resp); err != nil {
			return err
		}

		scopeLabel := "Organization"
		if budgetScope == "team" && budgetTeam != "" {
			scopeLabel = "team " + budgetTeam
		} else if budgetScope == "service" && budgetService != "" {
			scopeLabel = "service " + budgetService
		}
		fmt.Printf("Budget created: $%.2f %s %s limit on %s\n",
			budgetAmount, budgetPeriod, budgetMode, scopeLabel)
		return nil
	},
}

func init() {
	cmdBudgetsCreate.Flags().StringVar(&budgetScope, "scope", "org", "Scope: org, team, or service")
	cmdBudgetsCreate.Flags().StringVar(&budgetTeam, "team", "", "Team name (if scope=team)")
	cmdBudgetsCreate.Flags().StringVar(&budgetService, "service", "", "Service name (if scope=service)")
	cmdBudgetsCreate.Flags().Float64Var(&budgetAmount, "amount", 0, "Budget limit in USD (required)")
	cmdBudgetsCreate.Flags().StringVar(&budgetPeriod, "period", "monthly", "Period: monthly, weekly, daily")
	cmdBudgetsCreate.Flags().StringVar(&budgetMode, "mode", "soft", "Mode: soft or hard")

	cmdBudgets.AddCommand(cmdBudgetsList, cmdBudgetsCreate)
}
