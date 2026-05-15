package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var cmdReconcile = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile provider invoices against tolvyn data",
}

var (
	reconcileMonth    string
	reconcileProvider string
)

var cmdReconcileUpload = &cobra.Command{
	Use:   "upload <file.csv>",
	Short: "Upload a provider invoice CSV and run reconciliation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		csvPath := args[0]
		if reconcileMonth == "" {
			return fmt.Errorf("--month is required (YYYY-MM)")
		}

		f, err := os.Open(csvPath)
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		defer f.Close()

		// Build multipart body.
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("invoice_month", reconcileMonth)
		_ = mw.WriteField("provider", reconcileProvider)
		part, err := mw.CreateFormFile("file", filepath.Base(csvPath))
		if err != nil {
			return fmt.Errorf("create form file: %w", err)
		}
		if _, err := io.Copy(part, f); err != nil {
			return fmt.Errorf("copy file: %w", err)
		}
		mw.Close()

		req, err := http.NewRequest("POST", getAPIURL()+"/v1/reconciliation/upload", &buf)
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
		if tok := getToken(); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)

		if resp.StatusCode >= 300 {
			var e struct{ Message string `json:"message"` }
			json.Unmarshal(data, &e) //nolint:errcheck
			if e.Message != "" {
				return fmt.Errorf("%s", e.Message)
			}
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		if flagJSON {
			printJSON(data)
			return nil
		}

		var result struct {
			RunID           string  `json:"run_id"`
			InvoiceMonth    string  `json:"invoice_month"`
			InvoiceTotalUSD float64 `json:"invoice_total_usd"`
			TolvynTotalUSD  float64 `json:"tolvyn_total_usd"`
			GapUSD          float64 `json:"gap_usd"`
			MatchedLines    int     `json:"matched_lines"`
			UnmatchedLines  int     `json:"unmatched_lines"`
			ShadowAILines   int     `json:"shadow_ai_lines"`
			Lines           []struct {
				ModelID         string  `json:"model_id"`
				InvoiceRequests int64   `json:"invoice_requests"`
				InvoiceCostUSD  float64 `json:"invoice_cost_usd"`
				TolvynRequests  int64   `json:"tolvyn_requests"`
				TolvynCostUSD   float64 `json:"tolvyn_cost_usd"`
				GapUSD          float64 `json:"gap_usd"`
				Matched         bool    `json:"matched"`
				ShadowAI        bool    `json:"shadow_ai"`
			} `json:"lines"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}

		// Summary header.
		fmt.Printf("\n%s  %s reconciliation\n\n",
			cyan("RUN"), result.InvoiceMonth)
		fmt.Printf("  Invoice total : $%.4f\n", result.InvoiceTotalUSD)
		fmt.Printf("  Tolvyn total  : $%.4f\n", result.TolvynTotalUSD)
		gapStr := fmt.Sprintf("$%.4f", result.GapUSD)
		if result.GapUSD > 0 {
			fmt.Printf("  Gap           : %s\n", red("+"+gapStr))
		} else if result.GapUSD < 0 {
			fmt.Printf("  Gap           : %s\n", green(gapStr))
		} else {
			fmt.Printf("  Gap           : %s\n", green("balanced"))
		}
		fmt.Printf("  Matched lines : %d  Unmatched: %d  Shadow AI: %d\n\n",
			result.MatchedLines, result.UnmatchedLines, result.ShadowAILines)

		// Line item table.
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			cyan("MODEL"),
			cyan("INV REQ"),
			cyan("INV COST"),
			cyan("TLVN REQ"),
			cyan("TLVN COST"),
			cyan("GAP"),
			cyan("STATUS"),
		)
		for _, l := range result.Lines {
			status := green("matched")
			if l.ShadowAI {
				status = red("SHADOW AI")
			} else if !l.Matched {
				status = yellow("unmatched")
			}

			gapFmt := fmt.Sprintf("$%.4f", l.GapUSD)
			var gapColored string
			switch {
			case l.GapUSD > 0.001:
				gapColored = red("+" + gapFmt)
			case l.GapUSD < -0.001:
				gapColored = green(gapFmt)
			default:
				gapColored = gapFmt
			}

			fmt.Fprintf(tw, "%s\t%d\t$%.4f\t%d\t$%.4f\t%s\t%s\n",
				l.ModelID,
				l.InvoiceRequests, l.InvoiceCostUSD,
				l.TolvynRequests, l.TolvynCostUSD,
				gapColored,
				status,
			)
		}
		tw.Flush()
		fmt.Printf("\nRun ID: %s\n", result.RunID)
		return nil
	},
}

var cmdReconcileList = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List past reconciliation runs",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}

		if flagJSON {
			raw, _, err := doRequestRaw("GET", "/v1/reconciliation", nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var runs []struct {
			ID              string   `json:"id"`
			Provider        string   `json:"provider"`
			InvoiceMonth    string   `json:"invoice_month"`
			Filename        string   `json:"filename"`
			UploadedAt      string   `json:"uploaded_at"`
			InvoiceTotalUSD *float64 `json:"invoice_total_usd"`
			TolvynTotalUSD  *float64 `json:"tolvyn_total_usd"`
			GapUSD          *float64 `json:"gap_usd"`
			ShadowAILines   *int     `json:"shadow_ai_lines"`
		}
		if err := doRequest("GET", "/v1/reconciliation", nil, &runs); err != nil {
			return err
		}

		if len(runs) == 0 {
			fmt.Println("No reconciliation runs found.")
			return nil
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			cyan("MONTH"), cyan("PROVIDER"), cyan("INV TOTAL"), cyan("GAP"), cyan("SHADOW AI"), cyan("FILE"))
		for _, r := range runs {
			invStr := "—"
			if r.InvoiceTotalUSD != nil {
				invStr = fmt.Sprintf("$%.4f", *r.InvoiceTotalUSD)
			}
			gapStr := "—"
			if r.GapUSD != nil {
				g := *r.GapUSD
				if g > 0.001 {
					gapStr = red(fmt.Sprintf("+$%.4f", g))
				} else if g < -0.001 {
					gapStr = green(fmt.Sprintf("$%.4f", g))
				} else {
					gapStr = green("balanced")
				}
			}
			shadowStr := "—"
			if r.ShadowAILines != nil {
				shadowStr = fmt.Sprintf("%d", *r.ShadowAILines)
				if *r.ShadowAILines > 0 {
					shadowStr = red(shadowStr)
				}
			}
			fname := r.Filename
			if len(fname) > 30 {
				fname = "…" + fname[len(fname)-29:]
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
				r.InvoiceMonth, r.Provider, invStr, gapStr, shadowStr, fname)
		}
		tw.Flush()
		return nil
	},
}

var cmdReconcileDelete = &cobra.Command{
	Use:     "delete <run-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a reconciliation run",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		runID := args[0]
		fmt.Printf("Delete reconciliation run %s? [y/N]: ", runID)
		var answer string
		fmt.Scanln(&answer)
		if !strings.EqualFold(strings.TrimSpace(answer), "y") &&
			!strings.EqualFold(strings.TrimSpace(answer), "yes") {
			fmt.Println("Cancelled.")
			return nil
		}
		if err := doRequest("DELETE", "/v1/reconciliation/"+runID, nil, nil); err != nil {
			return err
		}
		fmt.Printf("Run %s deleted.\n", runID)
		return nil
	},
}

func init() {
	cmdReconcileUpload.Flags().StringVar(&reconcileMonth, "month", "", "Invoice month YYYY-MM (required)")
	cmdReconcileUpload.Flags().StringVar(&reconcileProvider, "provider", "openai", "Provider: openai")

	cmdReconcile.AddCommand(cmdReconcileUpload, cmdReconcileList, cmdReconcileDelete)
}
