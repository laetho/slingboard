package slingclient

import (
	"log"
	"os"

	"github.com/nats-io/nats.go"
)

func ConnectNATS() (*nats.Conn, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
		return nil, err
	}

	return nc, nil
}
