package cmd

import (
	"fmt"

	"github.com/laetho/slingboard/internal/server"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveNatsURL string
var serveNatsCreds string
var serveFQDN string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Sling Board server",
	Run: func(cmd *cobra.Command, args []string) {
		if serveNatsURL != "" {
			viper.Set("nats_url", serveNatsURL)
		}
		if serveNatsCreds != "" {
			viper.Set("nats_credentials", serveNatsCreds)
		}
		if serveFQDN != "" {
			viper.Set("fqdn", serveFQDN)
		}
		fmt.Println("Starting Sling Board server...")
		server.Start()
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveNatsURL, "nats-url", "", "NATS server URL")
	serveCmd.Flags().StringVar(&serveNatsCreds, "nats-creds", "", "NATS credentials file")
	serveCmd.Flags().StringVar(&serveNatsCreds, "nats-credentials", "", "NATS credentials file")
	serveCmd.Flags().StringVar(&serveFQDN, "fqdn", "", "FQDN for h8s subjects")
	rootCmd.AddCommand(serveCmd)
}
