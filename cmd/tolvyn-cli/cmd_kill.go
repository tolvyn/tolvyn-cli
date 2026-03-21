package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var killTeam string

var cmdKill = &cobra.Command{
	Use:   "kill",
	Short: "Emergency spend kill switch",
	Long:  "Blocks all AI requests for a team by setting a $0.000001 hard budget.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if killTeam == "" {
			return fmt.Errorf("--team is required")
		}

		fmt.Printf("Block all AI requests for team %q? This takes effect immediately. [y/N]: ", killTeam)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}

		// Resolve team name → UUID (proxy matches budgets by team UUID).
		var teams []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := doRequest("GET", "/v1/teams", nil, &teams); err != nil {
			return fmt.Errorf("could not list teams: %w", err)
		}
		teamID := ""
		for _, t := range teams {
			if t.Name == killTeam {
				teamID = t.ID
				break
			}
		}
		if teamID == "" {
			return fmt.Errorf("team %q not found", killTeam)
		}

		body := map[string]any{
			"scope_type": "team",
			"scope_id":   teamID,
			"amount_usd": 0.000001, // effectively $0 — blocks all requests
			"period":     "monthly",
			"mode":       "hard",
		}

		var resp map[string]any
		if err := doRequest("POST", "/v1/budgets", body, &resp); err != nil {
			return err
		}

		budgetID := ""
		if id, ok := resp["id"].(string); ok {
			budgetID = id
		}

		fmt.Println(red(fmt.Sprintf("KILL SWITCH ACTIVATED — all requests for team %q are now blocked.", killTeam)))
		if budgetID != "" {
			fmt.Printf("Budget ID: %s\n", budgetID)
			fmt.Printf("To undo: DELETE /v1/budgets/%s\n", budgetID)
		}
		return nil
	},
}

func init() {
	cmdKill.Flags().StringVar(&killTeam, "team", "", "Team name to block (required)")
}
