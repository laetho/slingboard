package cmd

import (
	"fmt"

	"github.com/laetho/slingboard/internal/server"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveNatsURL string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Sling Board server",
	Run: func(cmd *cobra.Command, args []string) {
		if serveNatsURL != "" {
			viper.Set("nats_url", serveNatsURL)
		}
		fmt.Println("Starting Sling Board server...")
		server.Start()
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveNatsURL, "nats-url", "", "NATS server URL")
	rootCmd.AddCommand(serveCmd)
}
