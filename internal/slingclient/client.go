package slingclient

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/laetho/slingboard/internal/commands"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type BoardList struct {
	Status string   `json:"status"`
	Boards []string `json:"boards"`
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) SendText(board string, message string) error {
	return c.sendCommand(commands.CommandRequest{
		Type:    commands.CommandText,
		Board:   board,
		Content: message,
	})
}

func (c *Client) SendURL(board string, url string) error {
	return c.sendCommand(commands.CommandRequest{
		Type:    commands.CommandURL,
		Board:   board,
		Content: url,
	})
}

func (c *Client) SendFile(board string, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	mimeType := detectMimeType(file, data)
	encoded := base64.StdEncoding.EncodeToString(data)

	return c.sendCommand(commands.CommandRequest{
		Type:     commands.CommandFile,
		Board:    board,
		Content:  encoded,
		MimeType: mimeType,
		Filename: filepath.Base(file),
	})
}

func (c *Client) sendCommand(command commands.CommandRequest) error {
	_, err := c.sendCommandResponse(command)
	return err
}

func (c *Client) sendCommandResponse(command commands.CommandRequest) (commands.CommandResponse, error) {
	payload, err := json.Marshal(command)
	if err != nil {
		return commands.CommandResponse{}, fmt.Errorf("failed to marshal command: %w", err)
	}

	request, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/commands", bytes.NewReader(payload))
	if err != nil {
		return commands.CommandResponse{}, fmt.Errorf("failed to build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return commands.CommandResponse{}, fmt.Errorf("failed to send command: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return commands.CommandResponse{}, fmt.Errorf("failed to read response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return commands.CommandResponse{}, fmt.Errorf("command failed: %s", strings.TrimSpace(string(body)))
	}

	if len(body) == 0 {
		return commands.CommandResponse{}, nil
	}

	var commandResponse commands.CommandResponse
	if err := json.Unmarshal(body, &commandResponse); err != nil {
		return commands.CommandResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if strings.EqualFold(commandResponse.Status, "error") {
		return commands.CommandResponse{}, fmt.Errorf("command error: %s", commandResponse.Message)
	}

	return commandResponse, nil
}

func (c *Client) BoardList() (BoardList, error) {
	response, err := c.sendCommandResponse(commands.CommandRequest{Type: commands.CommandBoardList})
	if err != nil {
		return BoardList{}, err
	}

	return BoardList{Status: response.Status, Boards: response.Boards}, nil
}

func (c *Client) BoardCreate(board string) (commands.CommandResponse, error) {
	return c.sendCommandResponse(commands.CommandRequest{Type: commands.CommandBoardCreate, Board: board})
}

func detectMimeType(filename string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".md" || ext == ".markdown" {
		return "text/markdown"
	}

	peekSize := min(len(data), 512)
	mimeType := http.DetectContentType(data[:peekSize])
	if isTextMime(mimeType) {
		if ext != "" {
			if fromExt := mime.TypeByExtension(ext); fromExt != "" {
				return fromExt
			}
		}
	}
	return mimeType
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func isTextMime(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/")
}
