package cmd

import (
	"fmt"
	"os"

	"github.com/laetho/slingboard/internal/config"
	"github.com/spf13/cobra"
)

var slingConfig = &cobra.Command{
	Use:   "config",
	Short: "Manipulate the configuration of the board",
	Long:  "Set, get, and list configuration options for the board.",
}

var slingConfigSet = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration option",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err)
			os.Exit(1)
		}

		switch key {
		case "nats_url":
			cfg.NATSURL = value
		case "nats_credentials":
			cfg.NATSCredentials = value
		default:
			fmt.Printf("Invalid configuration key: %s\n", key)
			os.Exit(1)
		}

		// Validate before saving
		if err := cfg.Validate(); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		// Save config
		if err := config.SaveConfig(cfg); err != nil {
			fmt.Println("Error saving config:", err)
			os.Exit(1)
		}

		fmt.Printf("Configuration updated: %s = %s\n", key, value)
	},
}

var slingConfigGet = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration option",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]

		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err)
			os.Exit(1)
		}

		switch key {
		case "nats_url":
			fmt.Printf("%s: %s\n", key, cfg.NATSURL)
		case "nats_credentials":
			fmt.Printf("%s: %s\n", key, cfg.NATSCredentials)
		default:
			fmt.Printf("Invalid configuration key: %s\n", key)
			os.Exit(1)
		}
	},
}

var slingConfigList = &cobra.Command{
	Use:   "list",
	Short: "List all configuration options",
	Long:  "Display all currently stored configuration options.",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err)
			os.Exit(1)
		}

		fmt.Println("Current Configuration:")
		fmt.Printf("  NATS URL         : %s\n", cfg.NATSURL)
		fmt.Printf("  NATS Credentials : %s\n", cfg.NATSCredentials)
	},
}

func init() {
	slingConfig.AddCommand(slingConfigSet, slingConfigGet, slingConfigList)
	rootCmd.AddCommand(slingConfig)
}
