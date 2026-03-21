package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var cmdTeams = &cobra.Command{
	Use:   "teams",
	Short: "Manage teams (list, create)",
}

var cmdTeamsList = &cobra.Command{
	Use:   "list",
	Short: "List teams",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if flagJSON {
			raw, _, err := doRequestRaw("GET", "/v1/teams", nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var teams []struct {
			ID         string     `json:"id"`
			Name       string     `json:"name"`
			CostCenter *string    `json:"cost_center"`
			CreatedAt  time.Time  `json:"created_at"`
		}
		if err := doRequest("GET", "/v1/teams", nil, &teams); err != nil {
			return err
		}

		fmt.Printf("%-20s %-14s %-12s\n", cyan("NAME"), cyan("COST CENTER"), cyan("CREATED"))
		for _, t := range teams {
			cc := "—"
			if t.CostCenter != nil && *t.CostCenter != "" {
				cc = *t.CostCenter
			}
			fmt.Printf("%-20s %-14s %-12s\n",
				t.Name, cc, t.CreatedAt.Format("2006-01-02"))
		}
		return nil
	},
}

var (
	teamCreateName       string
	teamCreateCostCenter string
)

var cmdTeamsCreate = &cobra.Command{
	Use:   "create",
	Short: "Create a team",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if teamCreateName == "" {
			return fmt.Errorf("--name is required")
		}

		body := map[string]any{"name": teamCreateName}
		if teamCreateCostCenter != "" {
			body["cost_center"] = teamCreateCostCenter
		}

		if flagJSON {
			raw, _, err := doRequestRaw("POST", "/v1/teams", body)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var resp struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := doRequest("POST", "/v1/teams", body, &resp); err != nil {
			return err
		}
		fmt.Printf("Team created: %s (id: %s)\n", resp.Name, resp.ID)
		return nil
	},
}

func init() {
	cmdTeamsCreate.Flags().StringVar(&teamCreateName, "name", "", "Team name (required)")
	cmdTeamsCreate.Flags().StringVar(&teamCreateCostCenter, "cost-center", "", "Cost center code")

	cmdTeams.AddCommand(cmdTeamsList, cmdTeamsCreate)
}
