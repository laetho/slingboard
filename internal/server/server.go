package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/alecthomas/chroma/quick"
	"github.com/laetho/slingboard/internal/slingmessage"
	"github.com/laetho/slingboard/internal/slingnats"
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
			var buf bytes.Buffer // templ output buffer
			var sling slingmessage.SlingMessage

			err := json.Unmarshal(msg.Data, &sling)
			if err != nil {
				log.Printf("Error unmarshalling message: %v", err)
				return
			}

			switch {
			case strings.HasPrefix(sling.MimeType, "image/"):
				component := templates.SlingImage(
					fmt.Sprintf(
						"data:%s;base64,%s", sling.MimeType, base64.RawStdEncoding.EncodeToString(sling.Content)),
				)
				if err := component.Render(context.Background(), &buf); err != nil {
					log.Printf("Error rendering template: %v", err)
				}

			case strings.HasPrefix(sling.MimeType, "text/x-go"):
				base64.StdEncoding.Decode(sling.Content, sling.Content)
				var buffer bytes.Buffer
				err := quick.Highlight(&buffer, string(sling.Content), "go", "html", "monokai")
				if err != nil {
					log.Printf("Error highlighting Go code: %v", err)
					return
				}
				component := templates.SlingCode(buffer.String())
				if err := component.Render(context.Background(), &buf); err != nil {
					log.Printf("Error rendering template: %v", err)
				}
			case strings.HasPrefix(sling.MimeType, "text/x-uri"):
				component := templates.SlingURL(string(sling.Content))
				if err := component.Render(context.Background(), &buf); err != nil {
					log.Printf("Error rendering template: %v", err)
				}

			case strings.HasPrefix(sling.MimeType, "text/"):
				component := templates.Sling(string(sling.Content))
				if err := component.Render(context.Background(), &buf); err != nil {
					log.Printf("Error rendering template: %v", err)
				}

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
	nc, err := slingnats.ConnectNATS()
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}
	defer nc.Close()

	http.HandleFunc("/", serveIndexTemplate)
	http.Handle("/slings", slingWebSocketHandler(nc))
	log.Println("Sling Board server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
