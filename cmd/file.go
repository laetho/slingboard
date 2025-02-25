package cmd

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/h2non/filetype"
	"github.com/spf13/cobra"
)

var slingFile = &cobra.Command{
	Use:   "file <filename>",
	Short: "Sling a file",
	Long:  "Sling a file to the board, slingboard will attempt to detect the filetype and handle it properly.",
	Run: func(cmd *cobra.Command, args []string) {
		filename := args[0]

		if len(filename) == 0 {
			log.Fatalf("No filename provided")
		}

		f, err := os.Open(filename)
		if err != nil {
			log.Fatalf("Unable to open file %v", err)
		}
		defer f.Close()

		buf := make([]byte, 261) // 261 bytes is enough for detection
		_, err = f.Read(buf)
		if err != nil {
			log.Fatal(err)
		}

		kind, _ := filetype.Match(buf)
		if kind == filetype.Unknown {
			fmt.Println("Unknown file type")
		} else {
			fmt.Printf("Detected file type: %s, MIME: %s\n", kind.Extension, kind.MIME.Value)
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
