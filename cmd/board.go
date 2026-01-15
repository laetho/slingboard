package cmd

import (
	"fmt"
	"log"

	sc "github.com/laetho/slingboard/internal/slingclient"
	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Manage slingBoards",
}

var boardListCmd = &cobra.Command{
	Use:   "list",
	Short: "List slingBoards",
	Run: func(cmd *cobra.Command, args []string) {
		client := sc.NewClient(apiURL)
		response, err := client.BoardList()
		if err != nil {
			log.Fatalf("Unable to list slingBoards: %v", err)
		}
		for _, board := range response.Boards {
			fmt.Fprintln(cmd.OutOrStdout(), board)
		}
	},
}

var boardCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a slingBoard",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		board := requireBoard(args[0])
		client := sc.NewClient(apiURL)
		response, err := client.BoardCreate(board)
		if err != nil {
			log.Fatalf("Unable to create slingBoard: %v", err)
		}
		if response.Board != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Created slingBoard: %s\n", response.Board)
		}
	},
}

func init() {
	boardCmd.AddCommand(boardListCmd)
	boardCmd.AddCommand(boardCreateCmd)
	rootCmd.AddCommand(boardCmd)
}
