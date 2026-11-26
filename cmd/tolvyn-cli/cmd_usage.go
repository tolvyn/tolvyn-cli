package main

import (
	"fmt"
	"io"
	"os"

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
	cmdUsageExport.Flags().Bool("force", false, "Overwrite the output file if it exists")
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

	path := "/v1/usage/requests?" + buildQueryString(map[string]string{
		"format": "csv", "from": from, "to": to, "team_id": team, "service": service, "model": model,
	})

	body, err := doRequestStream(path)
	if err != nil {
		return err
	}
	defer body.Close()

	var out io.Writer = os.Stdout
	if outPath != "" {
		force, _ := cmd.Flags().GetBool("force")
		f, err := createExportFile(outPath, force)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		defer f.Close()
		out = f
	}
	_, err = io.Copy(out, body)
	return err
}
