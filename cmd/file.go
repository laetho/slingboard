package cmd

import (
	"log"

	sc "github.com/laetho/slingboard/internal/slingclient"
	"github.com/spf13/cobra"
)

var slingFile = &cobra.Command{
	Use:   "file <filename>",
	Short: "Sling a file",
	Long:  "Sling a file to the board, slingboard will attempt to detect the filetype and handle it properly.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Fatalf("No file provided")
		}
		filename := args[0]

		client := sc.NewClient(apiURL)
		if err := client.SendFile("global", filename); err != nil {
			log.Fatalf("Unable to send message: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(slingFile)
}
