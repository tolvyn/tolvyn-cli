package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var cmdWebhooks = &cobra.Command{
	Use:     "webhooks",
	Aliases: []string{"wh"},
	Short:   "Manage webhook endpoints (list, create, delete, test, deliveries)",
}

var cmdWebhooksList = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List webhook endpoints",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if flagJSON {
			raw, _, err := doRequestRaw("GET", "/v1/webhooks", nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var result struct {
			Webhooks []struct {
				ID             string     `json:"id"`
				URL            string     `json:"url"`
				EventTypes     []string   `json:"event_types"`
				Active         bool       `json:"active"`
				LastDeliveryAt *time.Time `json:"last_delivery_at"`
				LastStatus     string     `json:"last_status"`
			} `json:"webhooks"`
			Total int `json:"total"`
		}
		if err := doRequest("GET", "/v1/webhooks", nil, &result); err != nil {
			return err
		}

		if len(result.Webhooks) == 0 {
			fmt.Println("No webhooks configured. Use 'tolvyn webhooks create --url <url> --events <list>' to add one.")
			return nil
		}

		fmt.Printf("%-38s  %-40s  %-30s  %-9s  %s\n",
			cyan("ID"), cyan("URL"), cyan("EVENTS"), cyan("STATUS"), cyan("LAST DELIVERY"))
		for _, w := range result.Webhooks {
			events := strings.Join(w.EventTypes, ",")
			last := "never"
			if w.LastDeliveryAt != nil {
				last = w.LastDeliveryAt.Local().Format("2006-01-02 15:04")
			}
			status := w.LastStatus
			if status == "" {
				status = "—"
			}
			if status == "success" {
				status = green(status)
			} else if status == "failed" {
				status = red(status)
			}
			active := ""
			if !w.Active {
				active = " (disabled)"
			}
			fmt.Printf("%-38s  %-40s  %-30s  %-9s  %s%s\n",
				w.ID, truncate(w.URL, 40), truncate(events, 30), status, last, active)
		}
		return nil
	},
}

var (
	webhookCreateURL    string
	webhookCreateEvents string
)

var cmdWebhooksCreate = &cobra.Command{
	Use:   "create",
	Short: "Create a webhook endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if webhookCreateURL == "" {
			return fmt.Errorf("--url is required")
		}
		if !strings.HasPrefix(webhookCreateURL, "https://") {
			return fmt.Errorf("--url must start with https://")
		}
		events := strings.Split(webhookCreateEvents, ",")
		for i := range events {
			events[i] = strings.TrimSpace(events[i])
		}

		body := map[string]any{
			"url":         webhookCreateURL,
			"event_types": events,
		}
		if flagJSON {
			raw, _, err := doRequestRaw("POST", "/v1/webhooks", body)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var resp struct {
			ID         string   `json:"id"`
			URL        string   `json:"url"`
			EventTypes []string `json:"event_types"`
			Secret     string   `json:"secret"`
		}
		if err := doRequest("POST", "/v1/webhooks", body, &resp); err != nil {
			return err
		}

		fmt.Printf("%s Webhook created\n", green("✓"))
		fmt.Printf("  ID:     %s\n", resp.ID)
		fmt.Printf("  URL:    %s\n", resp.URL)
		fmt.Printf("  Events: %s\n", strings.Join(resp.EventTypes, ","))
		fmt.Println()
		fmt.Println(yellow("Signing secret (shown ONCE — save it now):"))
		fmt.Println(resp.Secret)
		return nil
	},
}

var cmdWebhooksDelete = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a webhook endpoint",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		id := args[0]
		fmt.Printf("Delete webhook %s? [y/N]: ", id)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if a := strings.TrimSpace(strings.ToLower(answer)); a != "y" && a != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
		if err := doRequest("DELETE", "/v1/webhooks/"+url.PathEscape(id), nil, nil); err != nil {
			return err
		}
		fmt.Printf("%s Webhook %s deleted.\n", green("✓"), id)
		return nil
	},
}

var cmdWebhooksTest = &cobra.Command{
	Use:   "test <id>",
	Short: "Trigger a synthetic test delivery to a webhook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		id := args[0]
		var resp struct {
			Success    bool   `json:"success"`
			StatusCode int    `json:"status_code"`
			Error      string `json:"error"`
		}
		if err := doRequest("POST", "/v1/webhooks/"+url.PathEscape(id)+"/test", nil, &resp); err != nil {
			return err
		}
		if resp.Success {
			fmt.Printf("%s Delivered (HTTP %d)\n", green("✓"), resp.StatusCode)
			return nil
		}
		fmt.Printf("%s Failed (HTTP %d) — %s\n", red("✗"), resp.StatusCode, resp.Error)
		os.Exit(1)
		return nil
	},
}

var cmdWebhooksDeliveries = &cobra.Command{
	Use:   "deliveries <id>",
	Short: "Show recent delivery attempts for a webhook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		id := args[0]
		path := "/v1/webhooks/" + url.PathEscape(id) + "/deliveries"

		if flagJSON {
			raw, _, err := doRequestRaw("GET", path, nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var result struct {
			Deliveries []struct {
				ID           string     `json:"id"`
				EventType    string     `json:"event_type"`
				StatusCode   *int       `json:"status_code"`
				AttemptCount int        `json:"attempt_count"`
				DeliveredAt  *time.Time `json:"delivered_at"`
				ErrorMessage string     `json:"error_message"`
				CreatedAt    time.Time  `json:"created_at"`
			} `json:"deliveries"`
			Total int `json:"total"`
		}
		if err := doRequest("GET", path, nil, &result); err != nil {
			return err
		}

		if len(result.Deliveries) == 0 {
			fmt.Println("No deliveries yet.")
			return nil
		}

		fmt.Printf("%-38s  %-26s  %-7s  %-8s  %-16s  %s\n",
			cyan("DELIVERY ID"), cyan("EVENT"), cyan("STATUS"), cyan("ATTEMPTS"), cyan("DELIVERED"), cyan("ERROR"))
		for _, d := range result.Deliveries {
			statusStr := "—"
			if d.StatusCode != nil {
				statusStr = fmt.Sprintf("%d", *d.StatusCode)
				if *d.StatusCode >= 200 && *d.StatusCode < 300 {
					statusStr = green(statusStr)
				} else {
					statusStr = red(statusStr)
				}
			}
			delivered := "—"
			if d.DeliveredAt != nil {
				delivered = d.DeliveredAt.Local().Format("2006-01-02 15:04")
			}
			fmt.Printf("%-38s  %-26s  %-15s  %-8d  %-16s  %s\n",
				d.ID, truncate(d.EventType, 26), statusStr,
				d.AttemptCount, delivered, truncate(d.ErrorMessage, 50))
		}
		return nil
	},
}

func init() {
	cmdWebhooksCreate.Flags().StringVar(&webhookCreateURL, "url", "", "Webhook URL (https only, required)")
	cmdWebhooksCreate.Flags().StringVar(&webhookCreateEvents, "events", "alert.all", "Comma-separated event types (alert.budget_threshold, alert.cost_anomaly, alert.model_change, alert.pricing_change, alert.all)")

	cmdWebhooks.AddCommand(cmdWebhooksList, cmdWebhooksCreate, cmdWebhooksDelete, cmdWebhooksTest, cmdWebhooksDeliveries)
}
