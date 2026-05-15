package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var cmdUsage = &cobra.Command{
	Use:   "usage",
	Short: "Usage data commands",
}

var cmdUsageExport = &cobra.Command{
	Use:   "export",
	Short: "Export request log as CSV (up to 10 000 rows)",
	RunE:  runUsageExport,
}

func init() {
	cmdUsageExport.Flags().String("from", "", "Start date (YYYY-MM-DD)")
	cmdUsageExport.Flags().String("to", "", "End date (YYYY-MM-DD)")
	cmdUsageExport.Flags().String("team", "", "Team ID filter")
	cmdUsageExport.Flags().String("service", "", "Service name filter")
	cmdUsageExport.Flags().String("model", "", "Model ID filter")
	cmdUsageExport.Flags().StringP("output", "o", "", "Write to file instead of stdout")
	cmdUsage.AddCommand(cmdUsageExport)
}

func runUsageExport(cmd *cobra.Command, args []string) error {
	if getToken() == "" {
		return fmt.Errorf("not authenticated. Run 'tolvyn login'")
	}

	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	team, _ := cmd.Flags().GetString("team")
	service, _ := cmd.Flags().GetString("service")
	model, _ := cmd.Flags().GetString("model")
	outPath, _ := cmd.Flags().GetString("output")

	parts := []string{"format=csv"}
	if from != "" {
		parts = append(parts, "from="+from)
	}
	if to != "" {
		parts = append(parts, "to="+to)
	}
	if team != "" {
		parts = append(parts, "team_id="+team)
	}
	if service != "" {
		parts = append(parts, "service="+service)
	}
	if model != "" {
		parts = append(parts, "model="+model)
	}

	path := "/v1/usage/requests?" + strings.Join(parts, "&")

	data, status, err := doRequestRaw("GET", path, nil)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("server returned %d: %s", status, data)
	}

	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		defer f.Close()
		_, err = f.Write(data)
		return err
	}

	_, err = os.Stdout.Write(data)
	return err
}
