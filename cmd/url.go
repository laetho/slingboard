package cmd

import (
	"log"

	sc "github.com/laetho/slingboard/internal/slingclient"
	"github.com/spf13/cobra"
)

var slingUrl = &cobra.Command{
	Use:   "url <string>",
	Short: "Sling a url",
	Long:  "Sling a url to the board, slingboard will iframe it.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Fatalf("No url provided")
		}
		url := args[0]

		client := sc.NewClient()
		if err := client.SlingURL("slingboard.global", url); err != nil {
			log.Fatalf("Unable to send message: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(slingUrl)
}
