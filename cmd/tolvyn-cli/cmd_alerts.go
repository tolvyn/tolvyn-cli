package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

var cmdAlerts = &cobra.Command{
	Use:     "alerts",
	Aliases: []string{"al"},
	Short:   "Manage alerts (list, acknowledge, test)",
}

var (
	alertsUnread   bool
	alertsType     string
	alertsSeverity string
	alertsLimit    int
)

var cmdAlertsList = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List alerts",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		params := map[string]string{}
		if alertsUnread {
			params["acknowledged"] = "false"
		}
		if alertsSeverity != "" {
			params["severity"] = alertsSeverity
		}
		params["limit"] = fmt.Sprintf("%d", alertsLimit)
		path := "/v1/alerts?" + buildQueryString(params)

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
				ID           string    `json:"id"`
				AlertType    string    `json:"alert_type"`
				Severity     string    `json:"severity"`
				Title        string    `json:"title"`
				Acknowledged bool      `json:"acknowledged"`
				CreatedAt    time.Time `json:"created_at"`
			} `json:"data"`
			Total int `json:"total"`
		}
		if err := doRequest("GET", path, nil, &result); err != nil {
			return err
		}

		rows := result.Data
		// Client-side type filter (server doesn't support alert_type query yet).
		if alertsType != "" {
			filtered := rows[:0]
			for _, a := range rows {
				if a.AlertType == alertsType {
					filtered = append(filtered, a)
				}
			}
			rows = filtered
		}

		if len(rows) == 0 {
			fmt.Println("No alerts.")
			return nil
		}

		fmt.Printf("%-38s  %-22s  %-10s  %-50s  %-16s  %s\n",
			cyan("ID"), cyan("TYPE"), cyan("SEVERITY"), cyan("TITLE"), cyan("CREATED"), cyan("STATUS"))
		for _, a := range rows {
			sevColor := green
			switch a.Severity {
			case "critical":
				sevColor = red
			case "warning":
				sevColor = yellow
			}
			status := "unread"
			if a.Acknowledged {
				status = green("acked")
			} else {
				status = yellow(status)
			}
			fmt.Printf("%-38s  %-22s  %-19s  %-50s  %-16s  %s\n",
				a.ID, truncate(a.AlertType, 22),
				sevColor(fmt.Sprintf("%-10s", a.Severity)),
				truncate(a.Title, 50),
				a.CreatedAt.Local().Format("2006-01-02 15:04"),
				status,
			)
		}
		fmt.Printf("\n%d alert(s) shown (total %d)\n", len(rows), result.Total)
		return nil
	},
}

var cmdAlertsAck = &cobra.Command{
	Use:     "ack <id>",
	Aliases: []string{"acknowledge"},
	Short:   "Acknowledge an alert",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		id := args[0]
		if err := doRequest("PUT", "/v1/alerts/"+url.PathEscape(id)+"/acknowledge", nil, nil); err != nil {
			return err
		}
		fmt.Printf("%s Alert %s acknowledged.\n", green("✓"), id)
		return nil
	},
}

var cmdAlertsTest = &cobra.Command{
	Use:   "test",
	Short: "Create a synthetic test alert (verifies dashboard + delivery)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		var resp struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}
		if err := doRequest("POST", "/v1/alerts/test", nil, &resp); err != nil {
			return err
		}
		fmt.Printf("%s Test alert created (id: %s)\n", green("✓"), resp.ID)
		fmt.Printf("  %s\n", resp.Title)
		return nil
	},
}

func init() {
	cmdAlertsList.Flags().BoolVar(&alertsUnread, "unread", false, "Show only unacknowledged alerts")
	cmdAlertsList.Flags().StringVar(&alertsType, "type", "", "Filter by alert type (budget_threshold, cost_anomaly, model_change, pricing_change)")
	cmdAlertsList.Flags().StringVar(&alertsSeverity, "severity", "", "Filter by severity (info, warning, critical)")
	cmdAlertsList.Flags().IntVar(&alertsLimit, "limit", 50, "Page size")

	cmdAlerts.AddCommand(cmdAlertsList, cmdAlertsAck, cmdAlertsTest)
}
