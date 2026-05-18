package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	ledgerLimit  int
	ledgerOffset int
	ledgerFrom   string
	ledgerTo     string
	ledgerOutput string
)

var cmdLedger = &cobra.Command{
	Use:   "ledger",
	Short: "View, verify, and export the immutable cost ledger",
}

var cmdLedgerList = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List ledger records (paginated)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		params := map[string]string{
			"limit":  fmt.Sprintf("%d", ledgerLimit),
			"offset": fmt.Sprintf("%d", ledgerOffset),
		}
		if ledgerFrom != "" {
			params["from"] = ledgerFrom
		}
		if ledgerTo != "" {
			params["to"] = ledgerTo
		}
		path := "/v1/ledger?" + buildQueryString(params)

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
				SequenceNumber int64     `json:"sequence_number"`
				RequestID      string    `json:"request_id"`
				Provider       string    `json:"provider"`
				ModelID        string    `json:"model_id"`
				CostUSD        string    `json:"cost_usd"`
				HierarchyPath  string    `json:"hierarchy_path"`
				RecordHash     string    `json:"record_hash"`
				CreatedAt      time.Time `json:"created_at"`
			} `json:"data"`
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		}
		if err := doRequest("GET", path, nil, &result); err != nil {
			return err
		}

		if len(result.Data) == 0 {
			fmt.Println("No ledger records found.")
			return nil
		}

		fmt.Printf("%-7s  %-36s  %-9s  %-22s  %-10s  %-14s\n",
			cyan("SEQ"), cyan("REQUEST ID"), cyan("PROVIDER"), cyan("MODEL"), cyan("COST"), cyan("HASH"))
		for _, r := range result.Data {
			hash := r.RecordHash
			if len(hash) > 12 {
				hash = hash[:12]
			}
			fmt.Printf("%-7d  %-36s  %-9s  %-22s  %-10s  %-14s\n",
				r.SequenceNumber,
				truncate(r.RequestID, 36),
				r.Provider,
				truncate(r.ModelID, 22),
				green(r.CostUSD),
				hash,
			)
		}
		fmt.Printf("\nShowing %d of %d records (limit=%d offset=%d)\n",
			len(result.Data), result.Total, result.Limit, result.Offset)
		return nil
	},
}

var (
	verifyFromSeq int64
	verifyToSeq   int64
)

var cmdLedgerVerify = &cobra.Command{
	Use:   "verify",
	Short: "Verify ledger chain integrity (exits non-zero on broken chain)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		params := map[string]string{}
		if verifyFromSeq > 0 {
			params["from_seq"] = fmt.Sprintf("%d", verifyFromSeq)
		}
		if verifyToSeq > 0 {
			params["to_seq"] = fmt.Sprintf("%d", verifyToSeq)
		}
		path := "/v1/ledger/verify"
		if q := buildQueryString(params); q != "" {
			path += "?" + q
		}

		if flagJSON {
			raw, _, err := doRequestRaw("GET", path, nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var result struct {
			Valid                bool   `json:"valid"`
			RecordsChecked       int    `json:"records_checked"`
			FirstInvalidSequence int64  `json:"first_invalid_sequence,omitempty"`
			Reason               string `json:"reason,omitempty"`
		}
		if err := doRequest("GET", path, nil, &result); err != nil {
			return err
		}

		if result.Valid {
			fmt.Printf("%s Chain verified — %d records, all hashes valid\n",
				green("✓"), result.RecordsChecked)
			return nil
		}

		fmt.Printf("%s Chain broken at sequence %d — %s\n",
			red("✗"), result.FirstInvalidSequence, result.Reason)
		fmt.Printf("Records checked before break: %d\n", result.RecordsChecked)
		os.Exit(1)
		return nil
	},
}

var cmdLedgerExport = &cobra.Command{
	Use:   "export",
	Short: "Export the ledger as CSV (full payload including hashes)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		params := map[string]string{"format": "csv"}
		if ledgerFrom != "" {
			params["from"] = ledgerFrom
		}
		if ledgerTo != "" {
			params["to"] = ledgerTo
		}
		path := "/v1/ledger?" + buildQueryString(params)

		req, err := http.NewRequest("GET", getAPIURL()+path, nil)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+getToken())

		resp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("server returned %d", resp.StatusCode)
		}

		var out io.Writer = os.Stdout
		if ledgerOutput != "" {
			f, err := os.Create(ledgerOutput)
			if err != nil {
				return fmt.Errorf("create %s: %w", ledgerOutput, err)
			}
			defer f.Close()
			out = f
		}

		n, err := io.Copy(out, resp.Body)
		if err != nil {
			return err
		}
		if ledgerOutput != "" {
			fmt.Fprintf(os.Stderr, "%s Wrote %d bytes to %s\n", green("✓"), n, ledgerOutput)
		}
		return nil
	},
}

func init() {
	cmdLedgerList.Flags().IntVar(&ledgerLimit, "limit", 50, "Page size")
	cmdLedgerList.Flags().IntVar(&ledgerOffset, "offset", 0, "Page offset")
	cmdLedgerList.Flags().StringVar(&ledgerFrom, "from", "", "Start date (RFC3339 or YYYY-MM-DD)")
	cmdLedgerList.Flags().StringVar(&ledgerTo, "to", "", "End date (RFC3339 or YYYY-MM-DD)")

	cmdLedgerVerify.Flags().Int64Var(&verifyFromSeq, "from-seq", 0, "Starting sequence (default: 1)")
	cmdLedgerVerify.Flags().Int64Var(&verifyToSeq, "to-seq", 0, "Ending sequence (default: current max)")

	cmdLedgerExport.Flags().StringVar(&ledgerFrom, "from", "", "Start date (RFC3339 or YYYY-MM-DD)")
	cmdLedgerExport.Flags().StringVar(&ledgerTo, "to", "", "End date (RFC3339 or YYYY-MM-DD)")
	cmdLedgerExport.Flags().StringVarP(&ledgerOutput, "output", "o", "", "Write to file (default: stdout)")

	cmdLedger.AddCommand(cmdLedgerList, cmdLedgerVerify, cmdLedgerExport)
}
