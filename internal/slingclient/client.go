package slingclient

import (
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/laetho/slingboard/internal/slingmessage"
	"github.com/laetho/slingboard/internal/slingnats"
	"github.com/nats-io/nats.go"
)

type Client struct {
	nc *nats.Conn
}

func NewClient() *Client {
	nc, err := slingnats.ConnectNATS()
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}

	log.Printf("Connected to NATS at %s", nc.ConnectedUrl())

	return &Client{nc: nc}
}

func (c *Client) SendStringMessage(subject string, message []byte) error {
	sling := &slingmessage.SlingMessage{}
	sling.MimeType = "text/plain"
	sling.Content = message

	jsonData, err := json.Marshal(sling)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = c.nc.Publish(subject, jsonData)
	if err != nil {
		return err
	}

	// Ensure message is actually sent
	err = c.nc.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush message: %w", err)
	}

	return nil
}

func (c *Client) SlingURL(subject string, url string) error {
	sling := &slingmessage.SlingMessage{}
	sling.MimeType = "text/x-uri"
	sling.Content = []byte(url)

	jsonData, err := json.Marshal(sling)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = c.nc.Publish(subject, jsonData)
	if err != nil {
		return err
	}

	// Ensure message is actually sent
	err = c.nc.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush message: %w", err)
	}

	return nil
}

func (c *Client) SlingFile(subject string, file string) error {
	sling := &slingmessage.SlingMessage{}

	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	// Extract first 512 bytes (or less if file is smaller)
	peekSize := min(len(data), 512)
	mimeType := http.DetectContentType(data[:peekSize])
	ext := filepath.Ext(file)
	mimeTypeExt := mime.TypeByExtension(ext)
	finalMime := mimeType
	if isTextMime(mimeType) {
		finalMime = mimeTypeExt
	}

	fmt.Println("Detected MIME Type:", mimeType)
	fmt.Println("Detected MIME Type by extension:", mimeTypeExt)
	fmt.Println("Final MIME Type:", finalMime)

	sling.MimeType = finalMime
	sling.Content = []byte(data)

	jsonData, err := json.Marshal(sling)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = c.nc.Publish(subject, jsonData)
	if err != nil {
		return err
	}

	// Ensure message is actually sent
	err = c.nc.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush message: %w", err)
	}

	return nil
}

func isTextMime(mimeType string) bool {
	return len(mimeType) >= 5 && mimeType[:5] == "text/"
}
