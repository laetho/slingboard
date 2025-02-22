package cmd

import (
	"fmt"

	"github.com/laetho/slingboard/internal/server"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Sling Board server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting Sling Board server...")
		server.Start()
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
