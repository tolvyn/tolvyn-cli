package main

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var modelsProvider string

var cmdModels = &cobra.Command{
	Use:   "models",
	Short: "List available AI models and pricing",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}

		path := "/v1/models"
		if modelsProvider != "" {
			path += "?" + url.Values{"provider": {modelsProvider}}.Encode()
		}

		if flagJSON {
			raw, _, err := doRequestRaw("GET", path, nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var resp struct {
			Data []struct {
				ModelID              string     `json:"model_id"`
				Provider             string     `json:"provider"`
				ModelFamily          string     `json:"model_family"`
				DisplayName          string     `json:"display_name"`
				PricingInputPerMTok  float64    `json:"pricing_input_per_mtok"`
				PricingOutputPerMTok float64    `json:"pricing_output_per_mtok"`
				PricingCachedPerMTok *float64   `json:"pricing_cached_per_mtok"`
				ContextWindow        *int       `json:"context_window"`
				DeprecatedAt         *time.Time `json:"deprecated_at"`
			} `json:"data"`
			Total int `json:"total"`
		}
		if err := doRequest("GET", path, nil, &resp); err != nil {
			return err
		}

		if len(resp.Data) == 0 {
			fmt.Println("No models found.")
			return nil
		}

		fmt.Printf("%-40s %-12s %-18s %-10s %-10s\n",
			cyan("MODEL"), cyan("PROVIDER"), cyan("FAMILY"),
			cyan("IN/MTok"), cyan("OUT/MTok"))

		for _, m := range resp.Data {
			deprecated := ""
			if m.DeprecatedAt != nil {
				deprecated = red(" [deprecated]")
			}
			fmt.Printf("%-40s %-12s %-18s $%-9.4f $%-9.4f%s\n",
				m.ModelID,
				m.Provider,
				m.ModelFamily,
				m.PricingInputPerMTok,
				m.PricingOutputPerMTok,
				deprecated,
			)
		}
		fmt.Printf("\n%d model(s)\n", resp.Total)
		return nil
	},
}

var (
	modelsDiffLast     string
	modelsDiffProvider string
)

var cmdModelsDiff = &cobra.Command{
	Use:   "diff",
	Short: "Show model version changes and pricing history",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}

		params := map[string]string{}
		if modelsDiffLast != "" {
			dur, err := parseDuration(modelsDiffLast)
			if err != nil {
				return fmt.Errorf("invalid --last value: %w", err)
			}
			params["from"] = time.Now().Add(-dur).Format(time.RFC3339)
		}
		if modelsDiffProvider != "" {
			params["provider"] = modelsDiffProvider
		}

		path := "/v1/models/changes-history?" + buildQueryString(params)

		if flagJSON {
			raw, _, err := doRequestRaw("GET", path, nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var result struct {
			Changes []struct {
				ModelID        string `json:"model_id"`
				Provider       string `json:"provider"`
				FieldChanged   string `json:"field_changed"`
				PreviousValue  string `json:"previous_value"`
				NewValue       string `json:"new_value"`
				PercentChange  string `json:"percent_change"`
				EffectiveDate  string `json:"effective_date"`
				ImpactEstimate string `json:"impact_estimate_usd"`
				ImpactDir      string `json:"impact_direction"`
			} `json:"changes"`
			Total          int    `json:"total"`
			TotalImpactUSD string `json:"total_impact_usd"`
		}

		if err := doRequest("GET", path, nil, &result); err != nil {
			return err
		}

		if len(result.Changes) == 0 {
			fmt.Println("No pricing changes found for models you use.")
			return nil
		}

		fmt.Printf("%-20s %-12s %-8s %-12s %-12s %-8s %-12s %s\n",
			cyan("MODEL"), cyan("PROVIDER"), cyan("FIELD"), cyan("BEFORE"), cyan("AFTER"), cyan("CHANGE%"), cyan("IMPACT"), cyan("DATE"))
		fmt.Println(strings.Repeat("─", 100))

		for _, c := range result.Changes {
			var impact string
			switch c.ImpactDir {
			case "increase":
				impact = red("+" + c.ImpactEstimate)
			case "savings":
				impact = green(c.ImpactEstimate + " saved")
			default:
				impact = c.ImpactEstimate
			}

			date, _ := time.Parse(time.RFC3339, c.EffectiveDate)
			fmt.Printf("%-20s %-12s %-8s %-12s %-12s %-8s %-12s %s\n",
				truncate(c.ModelID, 20),
				c.Provider,
				c.FieldChanged,
				"$"+c.PreviousValue,
				"$"+c.NewValue,
				c.PercentChange+"%",
				impact,
				date.Format("2006-01-02"),
			)
		}
		fmt.Printf("\nTotal impact: %s\n", result.TotalImpactUSD)
		return nil
	},
}

var cmdModelsChanges = &cobra.Command{
	Use:   "changes",
	Short: "Show currently active pricing-change notifications for models you use",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}

		if flagJSON {
			raw, _, err := doRequestRaw("GET", "/v1/models/changes", nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var changes []struct {
			ModelID       string    `json:"model_id"`
			Provider      string    `json:"provider"`
			FieldChanged  string    `json:"field_changed"`
			PreviousValue *string   `json:"previous_value"`
			NewValue      *string   `json:"new_value"`
			PercentChange *string   `json:"percent_change"`
			EffectiveDate time.Time `json:"effective_date"`
		}
		if err := doRequest("GET", "/v1/models/changes", nil, &changes); err != nil {
			return err
		}

		if len(changes) == 0 {
			fmt.Println("No active pricing changes for models you use.")
			return nil
		}

		fmt.Printf("%-22s  %-10s  %-9s  %-12s  %-12s  %-9s  %s\n",
			cyan("MODEL"), cyan("PROVIDER"), cyan("FIELD"), cyan("BEFORE"), cyan("AFTER"), cyan("CHANGE%"), cyan("EFFECTIVE"))
		for _, c := range changes {
			prev := "—"
			if c.PreviousValue != nil {
				prev = "$" + *c.PreviousValue
			}
			next := "—"
			if c.NewValue != nil {
				next = "$" + *c.NewValue
			}
			pct := "—"
			if c.PercentChange != nil {
				pct = *c.PercentChange + "%"
			}
			fmt.Printf("%-22s  %-10s  %-9s  %-12s  %-12s  %-9s  %s\n",
				truncate(c.ModelID, 22), c.Provider, c.FieldChanged,
				prev, next, pct, c.EffectiveDate.Format("2006-01-02"))
		}
		return nil
	},
}

func init() {
	cmdModels.Flags().StringVar(&modelsProvider, "provider", "", "Filter by provider (openai, anthropic, google)")

	cmdModelsDiff.Flags().StringVar(&modelsDiffLast, "last", "30d", "Show changes in last N days (e.g. 7d, 30d, 90d)")
	cmdModelsDiff.Flags().StringVar(&modelsDiffProvider, "provider", "", "Filter by provider (openai, anthropic, google)")
	cmdModels.AddCommand(cmdModelsDiff, cmdModelsChanges)
}
