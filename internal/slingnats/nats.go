package slingnats

import (
	"log"

	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

func ConnectNATS() (*nats.Conn, error) {
	natsURL := viper.GetString("nats_url")
	natsCreds := viper.GetString("nats_credentials")
	if len(natsURL) == 0 {
		natsURL = nats.DefaultURL
	}

	var opts []nats.Option
	if natsCreds != "" {
		opts = append(opts, nats.UserCredentials(natsCreds))
	}
	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		log.Fatalf("Error connecting to NATS server %s: %v", natsURL, err)
		return nil, err
	}

	return nc, nil
}
