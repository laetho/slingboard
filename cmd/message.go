package cmd

import (
	"log"
	"strings"

	sc "github.com/laetho/slingboard/internal/slingclient"
	"github.com/spf13/cobra"
)

var messageBoard string

var slingMessage = &cobra.Command{
	Use:   "message <string>",
	Short: "Sling a chat message",
	Long:  "Sling a chat message (just a string).",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Fatalf("No message provided")
		}
		message := strings.Join(args, " ")
		board := requireBoard(messageBoard)
		client := sc.NewClient(apiURL)
		if err := client.SendText(board, message); err != nil {
			log.Fatalf("Unable to send message: %v", err)
		}
	},
}

func requireBoard(board string) string {
	if board == "" {
		log.Fatalf("--board is required")
	}

	if board != strings.ToLower(board) {
		log.Fatalf("Invalid board name: must be lowercase and URI-safe")
	}

	for _, char := range board {
		isAllowed := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_'
		if !isAllowed {
			log.Fatalf("Invalid board name: must be lowercase and URI-safe")
		}
	}

	return board
}

func init() {
	slingMessage.Flags().StringVarP(&messageBoard, "board", "b", "", "Board name (required)")
	rootCmd.AddCommand(slingMessage)
}
