package cmd

import (
	"log"
	"strings"

	sc "github.com/laetho/slingboard/internal/slingclient"
	"github.com/spf13/cobra"
)

var slingMessage = &cobra.Command{
	Use:   "message <string>",
	Short: "Sling a chat message",
	Long:  "Sling a chat message (just a string).",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Fatalf("No message provided")
		}
		message := strings.Join(args, " ")
		client := sc.NewClient()
		if err := client.SendStringMessage("slingboard.global", []byte(message)); err != nil {
			log.Fatalf("Unable to send message: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(slingMessage)
}
