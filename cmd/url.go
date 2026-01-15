package cmd

import (
	"log"

	sc "github.com/laetho/slingboard/internal/slingclient"
	"github.com/spf13/cobra"
)

var urlBoard string

var slingUrl = &cobra.Command{
	Use:   "url <string>",
	Short: "Sling a url",
	Long:  "Sling a url to the board, slingboard will iframe it.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Fatalf("No url provided")
		}
		url := args[0]
		board := requireBoard(urlBoard)

		client := sc.NewClient(apiURL)
		if err := client.SendURL(board, url); err != nil {
			log.Fatalf("Unable to send message: %v", err)
		}
	},
}

func init() {
	slingUrl.Flags().StringVarP(&urlBoard, "board", "b", "", "Board name (required)")
	rootCmd.AddCommand(slingUrl)
}
