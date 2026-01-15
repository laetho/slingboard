package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/chroma/quick"
	"github.com/laetho/slingboard/internal/commands"
	"github.com/laetho/slingboard/internal/slingmessage"
	"github.com/laetho/slingboard/internal/slingnats"
	staticfiles "github.com/laetho/slingboard/static"
	"github.com/laetho/slingboard/templates"
	"github.com/nats-io/nats.go"
)

const (
	commandSubject         = "slingboard.global"
	hostName               = "localhost"
	indexSubject           = "h8s.http.GET.localhost"
	commandsSubject        = "h8s.http.POST.localhost.api.commands"
	styleSubject           = "h8s.http.GET.localhost.static.style%2Ecss"
	websocketSubject       = "h8s.ws.ws.localhost.slings"
	websocketEstablished   = "h8s.control.ws.conn.established"
	websocketClosed        = "h8s.control.ws.conn.closed"
	websocketPublishHeader = "X-H8s-PublishSubject"
	noCacheHeader          = "no-cache"
	contentTypeHTML        = "text/html; charset=utf-8"
	contentTypeJSON        = "application/json"
	contentTypeCSS         = "text/css; charset=utf-8"
)

type service struct {
	nc        *nats.Conn
	wsMu      sync.RWMutex
	wsReplies map[string]struct{}
	subs      []*nats.Subscription
}

func (s *service) hasWebsocketReply(reply string) bool {
	s.wsMu.RLock()
	defer s.wsMu.RUnlock()
	_, ok := s.wsReplies[reply]
	return ok
}

func newService(nc *nats.Conn) *service {
	return &service{
		nc:        nc,
		wsReplies: make(map[string]struct{}),
	}
}

func (s *service) register() error {
	if err := s.queueSubscribe(indexSubject, s.handleIndex); err != nil {
		return err
	}
	if err := s.queueSubscribe(commandsSubject, s.handleCommands); err != nil {
		return err
	}
	if err := s.queueSubscribe(styleSubject, s.handleStyle); err != nil {
		return err
	}
	if err := s.subscribe(websocketEstablished, s.handleWebsocketControl); err != nil {
		return err
	}
	if err := s.subscribe(websocketClosed, s.handleWebsocketControl); err != nil {
		return err
	}
	if err := s.subscribe(commandSubject, s.broadcastSling); err != nil {
		return err
	}
	return nil
}

func (s *service) queueSubscribe(subject string, handler nats.MsgHandler) error {
	sub, err := s.nc.QueueSubscribe(subject, "slingboard", handler)
	if err != nil {
		return err
	}
	s.subs = append(s.subs, sub)
	return nil
}

func (s *service) subscribe(subject string, handler nats.MsgHandler) error {
	sub, err := s.nc.Subscribe(subject, handler)
	if err != nil {
		return err
	}
	s.subs = append(s.subs, sub)
	return nil
}

func (s *service) shutdown() {
	for _, sub := range s.subs {
		_ = sub.Unsubscribe()
	}
}

func Start() {
	nc, err := slingnats.ConnectNATS()
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}

	svc := newService(nc)
	if err := svc.register(); err != nil {
		log.Fatalf("Error registering subscriptions: %v", err)
	}

	log.Printf("Sling Board NATS service started for host %s", hostName)
	select {}
}

func (s *service) handleIndex(msg *nats.Msg) {
	var buf bytes.Buffer
	component := templates.Index()
	if err := component.Render(context.Background(), &buf); err != nil {
		s.respondError(msg, http.StatusInternalServerError, "failed to render index")
		return
	}

	s.respond(msg, http.StatusOK, contentTypeHTML, buf.Bytes())
}

func (s *service) handleStyle(msg *nats.Msg) {
	css, err := staticfiles.FS.ReadFile("style.css")
	if err != nil {
		s.respondError(msg, http.StatusInternalServerError, "failed to load stylesheet")
		return
	}

	s.respond(msg, http.StatusOK, contentTypeCSS, css)
}

func (s *service) handleCommands(msg *nats.Msg) {
	var request commands.CommandRequest
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		s.respondCommandError(msg, http.StatusBadRequest, "invalid command payload")
		return
	}

	payload, mimeType, err := commandPayload(request)
	if err != nil {
		s.respondCommandError(msg, http.StatusBadRequest, err.Error())
		return
	}

	id := strconv.FormatInt(time.Now().UnixNano(), 10)
	timestamp := time.Now().UTC()
	board := request.Board
	if board == "" {
		board = "global"
	}

	sling := slingmessage.SlingMessage{
		ID:        id,
		Timestamp: timestamp,
		MimeType:  mimeType,
		Content:   payload,
	}

	jsonData, err := json.Marshal(&sling)
	if err != nil {
		s.respondCommandError(msg, http.StatusInternalServerError, "failed to encode message")
		return
	}

	if err := s.nc.Publish(commandSubject, jsonData); err != nil {
		s.respondCommandError(msg, http.StatusBadGateway, "failed to publish message")
		return
	}

	if err := s.nc.Flush(); err != nil {
		s.respondCommandError(msg, http.StatusBadGateway, "failed to flush message")
		return
	}

	s.respondJSON(msg, http.StatusOK, commands.CommandResponse{
		ID:        id,
		Status:    "ok",
		Message:   "queued",
		Board:     board,
		Timestamp: timestamp,
	})
}

func (s *service) handleWebsocketControl(msg *nats.Msg) {
	publishSubject := msg.Header.Get(websocketPublishHeader)
	if publishSubject != websocketSubject {
		return
	}

	s.wsMu.Lock()
	defer s.wsMu.Unlock()

	switch msg.Subject {
	case websocketEstablished:
		s.wsReplies[msg.Reply] = struct{}{}
	case websocketClosed:
		delete(s.wsReplies, msg.Reply)
	}
}

func (s *service) broadcastSling(msg *nats.Msg) {
	var sling slingmessage.SlingMessage
	if err := json.Unmarshal(msg.Data, &sling); err != nil {
		log.Printf("Error unmarshalling message: %v", err)
		return
	}

	payload, err := renderSling(&sling)
	if err != nil {
		log.Printf("Error rendering sling message: %v", err)
		return
	}

	s.wsMu.RLock()
	defer s.wsMu.RUnlock()

	for reply := range s.wsReplies {
		if err := s.nc.Publish(reply, []byte(payload)); err != nil {
			log.Printf("Error sending websocket reply: %v", err)
		}
	}
}

func renderSling(sling *slingmessage.SlingMessage) (string, error) {
	var buf bytes.Buffer

	switch {
	case strings.HasPrefix(sling.MimeType, "image/"):
		component := templates.SlingImage(
			fmt.Sprintf(
				"data:%s;base64,%s", sling.MimeType, base64.RawStdEncoding.EncodeToString(sling.Content)),
		)
		if err := component.Render(context.Background(), &buf); err != nil {
			return "", err
		}

	case strings.HasPrefix(sling.MimeType, "text/x-go"):
		var buffer bytes.Buffer
		if err := quick.Highlight(&buffer, string(sling.Content), "go", "html", "monokai"); err != nil {
			return "", err
		}
		component := templates.SlingCode(buffer.String())
		if err := component.Render(context.Background(), &buf); err != nil {
			return "", err
		}

	case strings.HasPrefix(sling.MimeType, "text/x-uri"):
		component := templates.SlingURL(string(sling.Content))
		if err := component.Render(context.Background(), &buf); err != nil {
			return "", err
		}

	case strings.HasPrefix(sling.MimeType, "text/"):
		component := templates.Sling(string(sling.Content))
		if err := component.Render(context.Background(), &buf); err != nil {
			return "", err
		}
	}

	return buf.String(), nil
}

func commandPayload(request commands.CommandRequest) ([]byte, string, error) {
	switch request.Type {
	case commands.CommandText:
		if request.Content == "" {
			return nil, "", fmt.Errorf("message content is required")
		}
		return []byte(request.Content), "text/plain", nil
	case commands.CommandURL:
		if request.Content == "" {
			return nil, "", fmt.Errorf("url content is required")
		}
		return []byte(request.Content), "text/x-uri", nil
	case commands.CommandFile:
		if request.Content == "" {
			return nil, "", fmt.Errorf("file content is required")
		}
		decoded, err := base64.StdEncoding.DecodeString(request.Content)
		if err != nil {
			return nil, "", fmt.Errorf("invalid file content")
		}
		mimeType := request.MimeType
		if mimeType == "" {
			mimeType = detectMimeType(request.Filename, decoded)
		}
		return decoded, mimeType, nil
	default:
		return nil, "", fmt.Errorf("unsupported command type")
	}
}

func detectMimeType(filename string, data []byte) string {
	peekSize := min(len(data), 512)
	mimeType := http.DetectContentType(data[:peekSize])
	if isTextMime(mimeType) {
		if ext := filepath.Ext(filename); ext != "" {
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

func (s *service) respond(msg *nats.Msg, status int, contentType string, body []byte) {
	if msg.Reply == "" {
		log.Printf("missing reply subject for %s", msg.Subject)
		return
	}

	response := &nats.Msg{
		Header: nats.Header{
			"Status-Code":    []string{strconv.Itoa(status)},
			"Content-Type":   []string{contentType},
			"Cache-Control":  []string{noCacheHeader},
			"Content-Length": []string{strconv.Itoa(len(body))},
		},
		Data: body,
	}

	if err := msg.RespondMsg(response); err != nil {
		log.Printf("Failed to respond to %s: %v", msg.Subject, err)
	}
}

func (s *service) respondJSON(msg *nats.Msg, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		s.respondError(msg, http.StatusInternalServerError, "failed to encode response")
		return
	}

	s.respond(msg, status, contentTypeJSON, body)
}

func (s *service) respondError(msg *nats.Msg, status int, message string) {
	s.respond(msg, status, "text/plain; charset=utf-8", []byte(message))
}

func (s *service) respondCommandError(msg *nats.Msg, status int, message string) {
	s.respondJSON(msg, status, commands.CommandResponse{
		Status:  "error",
		Message: message,
	})
}
