package main

import (
	"fmt"
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
			path += "?provider=" + modelsProvider
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

func init() {
	cmdModels.Flags().StringVar(&modelsProvider, "provider", "", "Filter by provider (openai, anthropic, google)")
}
