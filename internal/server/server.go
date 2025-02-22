package server

import (
	"fmt"
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
		sub, err := nc.Subscribe("slingboard.>", func(msg *nats.Msg) {
			log.Println("Received message from NATS:", string(msg.Data))

			htmlMessage := fmt.Sprintf(`<div class="sling-message">%s</div>`, msg.Data)
			fmt.Println(htmlMessage)
			// Send NATS message to WebSocket client
			if err := websocket.Message.Send(ws, htmlMessage); err != nil {
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

func serveTemplate(w http.ResponseWriter, r *http.Request) {
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

	http.HandleFunc("/", serveTemplate)
	http.Handle("/slings", slingWebSocketHandler(nc))
	log.Println("Sling Board server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
