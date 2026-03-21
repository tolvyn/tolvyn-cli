package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var cmdKeys = &cobra.Command{
	Use:   "keys",
	Short: "Manage API keys (list, create, revoke)",
}

var cmdKeysList = &cobra.Command{
	Use:   "list",
	Short: "List API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if flagJSON {
			raw, _, err := doRequestRaw("GET", "/v1/api-keys", nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var keys []struct {
			ID          string     `json:"id"`
			Prefix      string     `json:"prefix"`
			Name        string     `json:"name"`
			Environment string     `json:"environment"`
			LastUsedAt  *time.Time `json:"last_used_at"`
			RevokedAt   *time.Time `json:"revoked_at"`
			CreatedAt   time.Time  `json:"created_at"`
		}
		if err := doRequest("GET", "/v1/api-keys", nil, &keys); err != nil {
			return err
		}

		fmt.Printf("%-16s %-20s %-14s %-20s\n",
			cyan("NAME"), cyan("PREFIX"), cyan("ENV"), cyan("LAST USED"))
		for _, k := range keys {
			lastUsed := "never"
			if k.LastUsedAt != nil {
				lastUsed = k.LastUsedAt.Format("2006-01-02 15:04")
			}
			prefix := k.Prefix + "..."
			fmt.Printf("%-16s %-20s %-14s %-20s\n",
				k.Name, prefix, k.Environment, lastUsed)
		}
		return nil
	},
}

var (
	keyCreateName string
	keyCreateEnv  string
	keyCreateTeam string
)

var cmdKeysCreate = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if keyCreateName == "" {
			return fmt.Errorf("--name is required")
		}

		body := map[string]any{
			"name":        keyCreateName,
			"environment": keyCreateEnv,
		}
		if keyCreateTeam != "" {
			body["team_id"] = keyCreateTeam
		}

		if flagJSON {
			raw, _, err := doRequestRaw("POST", "/v1/api-keys", body)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var resp struct {
			ID          string `json:"id"`
			Key         string `json:"key"`
			Prefix      string `json:"prefix"`
			Name        string `json:"name"`
			Environment string `json:"environment"`
		}
		if err := doRequest("POST", "/v1/api-keys", body, &resp); err != nil {
			return err
		}

		fmt.Printf("\nNew API key created: %s\n\n", resp.Name)
		fmt.Println(yellow(resp.Key))
		fmt.Println()
		fmt.Println(red("Save this key — it will not be shown again."))
		fmt.Println()
		return nil
	},
}

var cmdKeysRevoke = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Revoke an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		keyID := args[0]

		fmt.Printf("Revoke key %s? [y/N]: ", keyID)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}

		if flagJSON {
			raw, _, err := doRequestRaw("DELETE", "/v1/api-keys/"+keyID, nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		if err := doRequest("DELETE", "/v1/api-keys/"+keyID, nil, nil); err != nil {
			return err
		}
		fmt.Println("Key revoked.")
		return nil
	},
}

func init() {
	cmdKeysCreate.Flags().StringVar(&keyCreateName, "name", "", "Key name (required)")
	cmdKeysCreate.Flags().StringVar(&keyCreateEnv, "env", "production", "Environment")
	cmdKeysCreate.Flags().StringVar(&keyCreateTeam, "team", "", "Team ID")

	cmdKeys.AddCommand(cmdKeysList, cmdKeysCreate, cmdKeysRevoke)
}
