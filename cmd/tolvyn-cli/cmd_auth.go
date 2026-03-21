package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// ─── tolvyn init ────────────────────────────────────────────────────────────

var cmdInit = &cobra.Command{
	Use:   "init",
	Short: "Configure TOLVYN CLI",
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		fmt.Printf("API URL [http://localhost:8081]: ")
		apiURL, _ := reader.ReadString('\n')
		apiURL = strings.TrimSpace(apiURL)
		if apiURL == "" {
			apiURL = "http://localhost:8081"
		}

		fmt.Printf("Email: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)

		fmt.Printf("Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading password: %w", err)
		}
		password := strings.TrimSpace(string(passwordBytes))

		return doLogin(apiURL, email, password)
	},
}

// ─── tolvyn login ───────────────────────────────────────────────────────────

var cmdLogin = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with your account",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := loadConfig()
		apiURL := cfg.APIURL
		if apiURL == "" {
			apiURL = defaultAPIURL
		}
		if apiURLOverride != "" {
			apiURL = apiURLOverride
		}

		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Email: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)

		fmt.Printf("Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading password: %w", err)
		}
		password := strings.TrimSpace(string(passwordBytes))

		return doLogin(apiURL, email, password)
	},
}

// ─── tolvyn logout ──────────────────────────────────────────────────────────

var cmdLogout = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			cfg = &Config{APIURL: defaultAPIURL}
		}
		cfg.Token = ""
		if err := saveConfig(cfg); err != nil {
			return err
		}
		fmt.Println("Logged out.")
		return nil
	},
}

// ─── shared ─────────────────────────────────────────────────────────────────

func doLogin(apiURL, email, password string) error {
	var resp struct {
		Token string `json:"token"`
	}
	err := doRequestAt(apiURL, "POST", "/v1/auth/login",
		map[string]string{"email": email, "password": password}, &resp)
	if err != nil {
		return err
	}
	if resp.Token == "" {
		return fmt.Errorf("server returned no token")
	}

	cfg, _ := loadConfig()
	if cfg == nil {
		cfg = &Config{}
	}
	cfg.APIURL = apiURL
	cfg.Token = resp.Token
	if cfg.DefaultEnvironment == "" {
		cfg.DefaultEnvironment = "production"
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("Authenticated. Config saved to %s\n", configPath())
	return nil
}
