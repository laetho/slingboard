package server

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"

	"github.com/laetho/slingboard/templates"
	"github.com/nats-io/nats.go"
	"golang.org/x/net/websocket"
)

func slingWebSocketHandler(nc *nats.Conn) http.Handler {
	return websocket.Handler(func(ws *websocket.Conn) {
		log.Println("New WebSocket connection established")
		defer func() {
			log.Println("WebSocket connection closed")
			ws.Close()
		}()

		// Subscribe to NATS topic
		sub, err := nc.Subscribe("slingboard.global", func(msg *nats.Msg) {
			// todo this should be a template as well
			var buf bytes.Buffer
			component := templates.Sling(string(msg.Data))
			if err := component.Render(context.Background(), &buf); err != nil {
				log.Printf("Error rendering template: %v", err)
			}
			if err := websocket.Message.Send(ws, buf.String()); err != nil {
				log.Println("Error sending message to WebSocket:", err)
			}
		})
		if err != nil {
			log.Println("NATS subscription failed:", err)
			return
		}
		defer sub.Unsubscribe()

		// Keep the connection open until the client disconnects
		for {
			_, err := ws.Read(make([]byte, 1)) // Dummy read to detect disconnection
			if err != nil {
				log.Println("WebSocket connection closed by client")
				break
			}
		}
	})
}

func serveIndexTemplate(w http.ResponseWriter, r *http.Request) {
	component := templates.Index()
	w.Header().Set("Content-Type", "text/html")
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		log.Printf("Error rendering template: %v", err)
	}
}

func Start() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}
	defer nc.Close()

	http.HandleFunc("/", serveIndexTemplate)
	http.Handle("/slings", slingWebSocketHandler(nc))
	log.Println("Sling Board server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
