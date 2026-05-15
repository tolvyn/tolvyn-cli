package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	killScope  string
	killTarget string
	killReason string
)

var cmdKill = &cobra.Command{
	Use:   "kill",
	Short: "Emergency kill switch — block AI requests immediately",
	Long: `Instantly block AI requests for a scope.

Examples:
  tolvyn kill --scope team    --target eng-team  --reason "runaway loop"
  tolvyn kill --scope service --target chatbot-api
  tolvyn kill --scope agent   --target refactor-bot
  tolvyn kill --scope all     --reason "emergency"
  tolvyn kill list
  tolvyn kill undo <kill-id>`,
	RunE: runKillActivate,
}

var cmdKillActivate = &cobra.Command{
	Use:   "activate",
	Short: "Activate a kill switch (default subcommand)",
	RunE:  runKillActivate,
}

func runKillActivate(cmd *cobra.Command, args []string) error {
	if getToken() == "" {
		return fmt.Errorf("not authenticated. Run 'tolvyn login'")
	}
	if killScope == "" {
		return fmt.Errorf("--scope is required (team, service, agent, api_key, all)")
	}
	validScopes := map[string]bool{"team": true, "service": true, "agent": true, "api_key": true, "all": true}
	if !validScopes[killScope] {
		return fmt.Errorf("--scope must be one of: team, service, agent, api_key, all")
	}
	if killTarget == "" && killScope != "all" {
		return fmt.Errorf("--target is required when --scope is not 'all'")
	}
	scopeValue := killTarget
	if killScope == "all" && scopeValue == "" {
		scopeValue = "*"
	}

	targetLabel := fmt.Sprintf("%q", scopeValue)
	if killScope == "all" {
		targetLabel = "ALL requests for your organization"
	}
	fmt.Printf("Kill %s %s? This will immediately block all AI requests. [y/N]: ",
		killScope, targetLabel)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	body := map[string]any{
		"scope_type":  killScope,
		"scope_value": scopeValue,
	}
	if killReason != "" {
		body["reason"] = killReason
	}

	if flagJSON {
		raw, _, err := doRequestRaw("POST", "/v1/kill", body)
		if err != nil {
			return err
		}
		printJSON(raw)
		return nil
	}

	var resp struct {
		ID          string `json:"id"`
		ScopeType   string `json:"scope_type"`
		ScopeValue  string `json:"scope_value"`
		Reason      string `json:"reason"`
		ActivatedAt string `json:"activated_at"`
	}
	if err := doRequest("POST", "/v1/kill", body, &resp); err != nil {
		return err
	}

	fmt.Println(red("✓ Kill switch activated. All matching AI requests are now blocked."))
	fmt.Printf("  ID:    %s\n", resp.ID)
	fmt.Printf("  Scope: %s / %s\n", resp.ScopeType, resp.ScopeValue)
	if resp.Reason != "" {
		fmt.Printf("  Reason: %s\n", resp.Reason)
	}
	fmt.Printf("\nTo undo: %s\n", cyan("tolvyn kill undo "+resp.ID))
	return nil
}

var cmdKillList = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List active kill switches",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}

		if flagJSON {
			raw, _, err := doRequestRaw("GET", "/v1/kill", nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var resp struct {
			Kills []struct {
				ID          string `json:"id"`
				ScopeType   string `json:"scope_type"`
				ScopeValue  string `json:"scope_value"`
				Reason      string `json:"reason"`
				ActivatedBy string `json:"activated_by"`
				ActivatedAt string `json:"activated_at"`
			} `json:"kills"`
			Total int `json:"total"`
		}
		if err := doRequest("GET", "/v1/kill", nil, &resp); err != nil {
			return err
		}

		if resp.Total == 0 {
			fmt.Println("No active kill switches.")
			return nil
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			cyan("ID"), cyan("SCOPE"), cyan("TARGET"), cyan("REASON"), cyan("ACTIVATED"))
		for _, k := range resp.Kills {
			reason := k.Reason
			if reason == "" {
				reason = "—"
			}
			activatedAt := k.ActivatedAt
			if t, err := time.Parse(time.RFC3339, k.ActivatedAt); err == nil {
				activatedAt = t.Local().Format("2006-01-02 15:04")
			}
			idShort := k.ID
			if len(idShort) > 8 {
				idShort = idShort[:8] + "…"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
				red(idShort), k.ScopeType, k.ScopeValue, reason, activatedAt)
		}
		tw.Flush()
		fmt.Printf("\n%d active kill switch(es)\n", resp.Total)
		return nil
	},
}

var cmdKillUndo = &cobra.Command{
	Use:     "undo <kill-id>",
	Aliases: []string{"deactivate", "rm"},
	Short:   "Deactivate a kill switch",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		killID := args[0]

		if err := doRequest("DELETE", "/v1/kill/"+killID, nil, nil); err != nil {
			return err
		}
		fmt.Printf("%s Kill switch %s deactivated. AI requests are flowing again.\n",
			green("✓"), killID)
		return nil
	},
}

func init() {
	// Flags for activate (both the parent and subcommand share them).
	for _, c := range []*cobra.Command{cmdKill, cmdKillActivate} {
		c.Flags().StringVar(&killScope, "scope", "", "Scope: team, service, agent, api_key, all (required)")
		c.Flags().StringVar(&killTarget, "target", "", "Target value (team name, service name, etc.)")
		c.Flags().StringVar(&killReason, "reason", "", "Optional reason for the kill switch")
	}

	cmdKill.AddCommand(cmdKillActivate, cmdKillList, cmdKillUndo)
}
