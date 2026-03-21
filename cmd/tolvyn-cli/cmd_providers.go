package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var cmdProviders = &cobra.Command{
	Use:   "providers",
	Short: "Manage provider API keys (list, add)",
}

var cmdProvidersList = &cobra.Command{
	Use:   "list",
	Short: "List connected providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		if flagJSON {
			raw, _, err := doRequestRaw("GET", "/v1/provider-keys", nil)
			if err != nil {
				return err
			}
			printJSON(raw)
			return nil
		}

		var keys []struct {
			ID            string     `json:"id"`
			Provider      string     `json:"provider"`
			KeyVersion    int        `json:"key_version"`
			LastRotatedAt *time.Time `json:"last_rotated_at"`
			CreatedAt     time.Time  `json:"created_at"`
		}
		if err := doRequest("GET", "/v1/provider-keys", nil, &keys); err != nil {
			return err
		}

		if len(keys) == 0 {
			fmt.Println("No provider keys configured. Use 'tolvyn providers add <provider>' to add one.")
			return nil
		}

		header := fmt.Sprintf("%-12s  %-36s  %-5s  %-20s",
			"PROVIDER", "ID", "VER", "ADDED")
		fmt.Println(cyan(header))
		fmt.Println(strings.Repeat("─", len(header)))
		for _, k := range keys {
			fmt.Printf("%-12s  %-36s  %-5d  %-20s\n",
				k.Provider, k.ID, k.KeyVersion,
				k.CreatedAt.Local().Format("2006-01-02 15:04"))
		}
		return nil
	},
}

var cmdProvidersAdd = &cobra.Command{
	Use:   "add <provider>",
	Short: "Add a provider API key (openai, anthropic, google)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if getToken() == "" {
			return fmt.Errorf("not authenticated. Run 'tolvyn login'")
		}
		provider := strings.ToLower(args[0])
		switch provider {
		case "openai", "anthropic", "google":
		default:
			return fmt.Errorf("unsupported provider %q (use: openai, anthropic, google)", provider)
		}

		fmt.Printf("Enter %s API key (input hidden): ", provider)
		keyBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("read key: %w", err)
		}
		apiKey := strings.TrimSpace(string(keyBytes))
		if apiKey == "" {
			return fmt.Errorf("API key cannot be empty")
		}

		var resp struct {
			ID       string `json:"id"`
			Provider string `json:"provider"`
		}
		if err := doRequest("POST", "/v1/provider-keys", map[string]string{
			"provider": provider,
			"key":      apiKey,
		}, &resp); err != nil {
			return err
		}

		fmt.Printf("%s %s provider key stored (id: %s)\n",
			green("✓"), resp.Provider, resp.ID)
		return nil
	},
}

func init() {
	cmdProviders.AddCommand(cmdProvidersList, cmdProvidersAdd)
}
