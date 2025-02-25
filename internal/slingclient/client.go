package slingclient

import (
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
)

type Client struct {
	nc *nats.Conn
}

func NewClient() *Client {
	var client Client
	nc, err := ConnectNATS()
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}
	client.nc = nc
	return &client
}

func (c *Client) SendStringMessage(subject, message string) error {
	fmt.Println("Message:", message)
	fmt.Println("Subject:", subject)
	if err := c.nc.Publish(subject, []byte(message)); err != nil {
		fmt.Println("Error publishing message to NATS:", err)
		return err
	}
	fmt.Println("Message sent successfully")
	return nil
}
