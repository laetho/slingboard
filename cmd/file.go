package cmd

import (
	"log"

	sc "github.com/laetho/slingboard/internal/slingclient"
	"github.com/spf13/cobra"
)

var fileBoard string

var slingFile = &cobra.Command{
	Use:   "file <filename>",
	Short: "Sling a file",
	Long:  "Sling a file to the board, slingboard will attempt to detect the filetype and handle it properly.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Fatalf("No file provided")
		}
		filename := args[0]
		board := requireBoard(fileBoard)

		client := sc.NewClient(apiURL)
		if err := client.SendFile(board, filename); err != nil {
			log.Fatalf("Unable to send message: %v", err)
		}
	},
}

func init() {
	slingFile.Flags().StringVarP(&fileBoard, "board", "b", "", "Board name (required)")
	rootCmd.AddCommand(slingFile)
}
