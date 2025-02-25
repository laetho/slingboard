package cmd

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"

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

		client := sc.NewClient()
		if err := client.SlingFile("slingboard.global", filename); err != nil {
			log.Fatalf("Unable to send message: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(slingFile)
}

func detectMimeType(filename string) (string, error) {
	ext := filepath.Ext(filename)
	if ext != "" {
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			return mimeType, nil
		}
	}

	// Open the file and read a small portion to sniff the type
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	// Read a small portion of the file
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("could not read file: %w", err)
	}

	// Detect the MIME type using net/http package
	return http.DetectContentType(buffer[:n]), nil
}
