package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alecthomas/chroma/quick"
	"github.com/laetho/slingboard/internal/commands"
	"github.com/laetho/slingboard/internal/slingmessage"
	"github.com/laetho/slingboard/internal/slingnats"
	staticfiles "github.com/laetho/slingboard/static"
	"github.com/laetho/slingboard/templates"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nuid"
	"github.com/spf13/viper"
	"github.com/yuin/goldmark"
)

const (
	defaultBoard           = "global"
	commandSubjectPrefix   = "slingboard."
	streamPrefix           = "sb_"
	defaultFQDN            = "localhost"
	websocketEstablished   = "h8s.control.ws.conn.established"
	websocketClosed        = "h8s.control.ws.conn.closed"
	websocketPublishHeader = "X-H8s-PublishSubject"
	noCacheHeader          = "no-cache"
	contentTypeHTML        = "text/html; charset=utf-8"
	contentTypeJSON        = "application/json"
	contentTypeCSS         = "text/css; charset=utf-8"
	markdownMimeType       = "text/markdown"
)

var (
	indexSubject           = ""
	boardSubjectPrefix     = ""
	boardSubjectWildcard   = ""
	commandsSubject        = ""
	styleSubject           = ""
	websocketSubjectPrefix = ""
	configuredFQDN         = ""
)

var markdownRenderer = goldmark.New()

type wsConnection struct {
	board        string
	reply        string
	consumerName string
	streamName   string
	stop         chan struct{}
}

type service struct {
	nc      *nats.Conn
	js      nats.JetStreamContext
	wsMu    sync.RWMutex
	wsConns map[string]*wsConnection
	subs    []*nats.Subscription
}

func (s *service) hasWebsocketReply(board string, reply string) bool {
	s.wsMu.RLock()
	defer s.wsMu.RUnlock()
	conn, ok := s.wsConns[reply]
	return ok && conn.board == board
}

func newService(nc *nats.Conn, js nats.JetStreamContext) *service {
	return &service{
		nc:      nc,
		js:      js,
		wsConns: make(map[string]*wsConnection),
	}
}

func (s *service) register() error {
	if err := s.queueSubscribe(indexSubject, s.handleIndex); err != nil {
		return err
	}
	if err := s.queueSubscribe(boardSubjectWildcard, s.handleBoard); err != nil {
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
	return nil
}

func reversedFQDN(fqdn string) string {
	trimmed := strings.TrimSpace(fqdn)
	if trimmed == "" {
		return defaultFQDN
	}
	parts := strings.Split(trimmed, ".")
	for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
		parts[left], parts[right] = parts[right], parts[left]
	}
	return strings.Join(parts, ".")
}

func configureSubjects() {
	fqdn := viper.GetString("fqdn")
	if fqdn == "" {
		fqdn = defaultFQDN
	}
	configuredFQDN = fqdn
	reversed := strings.ToLower(reversedFQDN(fqdn))
	indexSubject = fmt.Sprintf("h8s.http.get.%s", reversed)
	boardSubjectPrefix = fmt.Sprintf("h8s.http.get.%s.board.", reversed)
	boardSubjectWildcard = fmt.Sprintf("h8s.http.get.%s.board.*", reversed)
	commandsSubject = fmt.Sprintf("h8s.http.post.%s.api.commands", reversed)
	styleSubject = fmt.Sprintf("h8s.http.get.%s.static.style%%2Ecss", reversed)
	websocketSubjectPrefix = fmt.Sprintf("h8s.ws.ws.%s.board.", reversed)
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

func (s *service) cleanupWebsocketConsumers() {
	for streamName := range s.js.StreamNames() {
		if !strings.HasPrefix(streamName, streamPrefix) {
			continue
		}
		for consumerName := range s.js.ConsumerNames(streamName) {
			if !strings.HasPrefix(consumerName, "ws-") {
				continue
			}
			if err := s.js.DeleteConsumer(streamName, consumerName); err != nil {
				log.Printf("Failed to delete websocket consumer %s for stream %s: %v", consumerName, streamName, err)
				continue
			}
			log.Printf("Deleted websocket consumer %s for stream %s", consumerName, streamName)
		}
	}
}

func (s *service) shutdown() {
	for _, sub := range s.subs {
		_ = sub.Unsubscribe()
	}

	s.wsMu.Lock()
	replies := make([]string, 0, len(s.wsConns))
	for reply := range s.wsConns {
		replies = append(replies, reply)
	}
	s.wsMu.Unlock()

	for _, reply := range replies {
		s.stopWebsocketConsumer(reply)
	}
}

func Start() {
	nc, err := slingnats.ConnectNATS()
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("Error creating JetStream context: %v", err)
	}

	configureSubjects()
	svc := newService(nc, js)
	svc.cleanupWebsocketConsumers()
	if err := svc.register(); err != nil {
		log.Fatalf("Error registering subscriptions: %v", err)
	}

	log.Printf("Sling Board NATS service started for host %s", configuredFQDN)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	log.Printf("Shutting down Sling Board NATS service")
	svc.shutdown()
	if err := nc.Drain(); err != nil {
		log.Printf("Failed to drain NATS connection: %v", err)
	}
	nc.Close()
}

func (s *service) handleIndex(msg *nats.Msg) {
	boards, err := s.listBoards()
	if err != nil {
		s.respondError(msg, http.StatusInternalServerError, "failed to list boards")
		return
	}

	boardCards, err := renderBoardCards(boards)
	if err != nil {
		s.respondError(msg, http.StatusInternalServerError, "failed to render board list")
		return
	}

	var buf bytes.Buffer
	component := templates.BoardsIndex(boardCards)
	if err := component.Render(context.Background(), &buf); err != nil {
		s.respondError(msg, http.StatusInternalServerError, "failed to render index")
		return
	}

	s.respond(msg, http.StatusOK, contentTypeHTML, buf.Bytes())
}

func (s *service) handleBoard(msg *nats.Msg) {
	board, ok := boardFromSubject(msg.Subject, boardSubjectPrefix)
	if !ok {
		s.respondError(msg, http.StatusNotFound, "board not found")
		return
	}

	if _, err := s.ensureBoardStream(board); err != nil {
		s.respondError(msg, http.StatusInternalServerError, "failed to ensure board stream")
		return
	}

	var buf bytes.Buffer
	component := templates.BoardView(board)
	if err := component.Render(context.Background(), &buf); err != nil {
		s.respondError(msg, http.StatusInternalServerError, "failed to render board")
		return
	}

	s.respond(msg, http.StatusOK, contentTypeHTML, buf.Bytes())
}

func (s *service) listBoards() ([]string, error) {
	boardSet := make(map[string]struct{})
	for name := range s.js.StreamNames() {
		if !strings.HasPrefix(name, streamPrefix) {
			continue
		}
		board := strings.TrimPrefix(name, streamPrefix)
		if board == "" {
			continue
		}
		boardSet[board] = struct{}{}
	}

	boards := make([]string, 0, len(boardSet))
	for board := range boardSet {
		boards = append(boards, board)
	}
	sort.Strings(boards)
	return boards, nil
}

func renderBoardCards(boards []string) (string, error) {
	var buf bytes.Buffer
	for _, board := range boards {
		component := templates.BoardCard(board)
		if err := component.Render(context.Background(), &buf); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func (s *service) ensureBoardStream(board string) (string, error) {
	streamName := streamPrefix + board
	if _, err := s.js.StreamInfo(streamName); err == nil {
		return streamName, nil
	} else if !errors.Is(err, nats.ErrStreamNotFound) {
		return "", err
	}

	_, err := s.js.AddStream(&nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{commandSubjectPrefix + board},
		Retention: nats.InterestPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil {
		return "", err
	}

	return streamName, nil
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

	switch request.Type {
	case commands.CommandBoardList:
		s.handleBoardList(msg)
		return
	case commands.CommandBoardCreate:
		s.handleBoardCreate(msg, request)
		return
	}

	payload, mimeType, err := commandPayload(request)
	if err != nil {
		s.respondCommandError(msg, http.StatusBadRequest, err.Error())
		return
	}

	id := strconv.FormatInt(time.Now().UnixNano(), 10)
	timestamp := time.Now().UTC()
	board := normalizeBoardName(request.Board)
	if board == "" {
		board = defaultBoard
	}
	author := strings.TrimSpace(request.Author)
	if author == "" {
		author = "anonymous"
	}

	if _, err := s.ensureBoardStream(board); err != nil {
		s.respondCommandError(msg, http.StatusInternalServerError, "failed to ensure board stream")
		return
	}

	sling := slingmessage.SlingMessage{
		ID:        id,
		Sender:    author,
		Timestamp: timestamp,
		MimeType:  mimeType,
		Content:   payload,
	}

	jsonData, err := json.Marshal(&sling)
	if err != nil {
		s.respondCommandError(msg, http.StatusInternalServerError, "failed to encode message")
		return
	}

	publishSubject := commandSubjectPrefix + board
	if err := s.nc.Publish(publishSubject, jsonData); err != nil {
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
	board, ok := boardFromSubject(publishSubject, websocketSubjectPrefix)
	if !ok {
		return
	}

	switch msg.Subject {
	case websocketEstablished:
		s.startWebsocketConsumer(board, msg.Reply)
	case websocketClosed:
		s.stopWebsocketConsumer(msg.Reply)
	}
}

func (s *service) handleBoardList(msg *nats.Msg) {
	boards, err := s.listBoards()
	if err != nil {
		s.respondCommandError(msg, http.StatusInternalServerError, "failed to list boards")
		return
	}

	if wantsJSON(msg) {
		s.respondJSON(msg, http.StatusOK, commands.CommandResponse{
			Status: "ok",
			Boards: boards,
		})
		return
	}

	var buf strings.Builder
	buf.WriteString("<ul>")
	for _, board := range boards {
		buf.WriteString("<li>")
		buf.WriteString(board)
		buf.WriteString("</li>")
	}
	buf.WriteString("</ul>")
	s.respond(msg, http.StatusOK, contentTypeHTML, []byte(buf.String()))
}

func (s *service) handleBoardCreate(msg *nats.Msg, request commands.CommandRequest) {
	board := normalizeBoardName(request.Board)
	if board == "" {
		s.respondCommandError(msg, http.StatusBadRequest, "board is required")
		return
	}

	if _, err := s.ensureBoardStream(board); err != nil {
		s.respondCommandError(msg, http.StatusInternalServerError, "failed to ensure board stream")
		return
	}

	if wantsJSON(msg) {
		s.respondJSON(msg, http.StatusOK, commands.CommandResponse{
			Status: "ok",
			Board:  board,
		})
		return
	}

	html := "<ul><li>" + board + "</li></ul>"
	s.respond(msg, http.StatusOK, contentTypeHTML, []byte(html))
}

func wantsJSON(msg *nats.Msg) bool {
	accept := msg.Header.Get("Accept")
	return strings.Contains(accept, "application/json")
}

func (s *service) startWebsocketConsumer(board string, reply string) {
	if reply == "" {
		return
	}

	streamName, err := s.ensureBoardStream(board)
	if err != nil {
		log.Printf("Failed to ensure board stream: %v", err)
		return
	}

	s.wsMu.Lock()
	if _, exists := s.wsConns[reply]; exists {
		s.wsMu.Unlock()
		return
	}
	consumerName := "ws-" + nuid.Next()
	conn := &wsConnection{
		board:        board,
		reply:        reply,
		consumerName: consumerName,
		streamName:   streamName,
		stop:         make(chan struct{}),
	}
	s.wsConns[reply] = conn
	s.wsMu.Unlock()

	_, err = s.js.AddConsumer(streamName, &nats.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     nats.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		FilterSubject: commandSubjectPrefix + board,
		DeliverPolicy: nats.DeliverAllPolicy,
		ReplayPolicy:  nats.ReplayInstantPolicy,
	})
	if err != nil {
		log.Printf("Failed to create consumer: %v", err)
		s.stopWebsocketConsumer(reply)
		return
	}

	sub, err := s.js.PullSubscribe(commandSubjectPrefix+board, consumerName, nats.Bind(streamName, consumerName), nats.ManualAck())
	if err != nil {
		log.Printf("Failed to subscribe to consumer: %v", err)
		s.stopWebsocketConsumer(reply)
		return
	}

	go s.consumeWebsocket(conn, sub)
}

func (s *service) consumeWebsocket(conn *wsConnection, sub *nats.Subscription) {
	defer sub.Unsubscribe()
	for {
		select {
		case <-conn.stop:
			return
		default:
			msgs, err := sub.Fetch(1, nats.MaxWait(2*time.Second))
			if err != nil {
				if errors.Is(err, nats.ErrTimeout) {
					continue
				}
				if errors.Is(err, nats.ErrConnectionClosed) || errors.Is(err, nats.ErrBadSubscription) {
					return
				}
				log.Printf("Failed to fetch messages: %v", err)
				continue
			}
			for _, msg := range msgs {
				var sling slingmessage.SlingMessage
				if err := json.Unmarshal(msg.Data, &sling); err != nil {
					log.Printf("Error unmarshalling message: %v", err)
					_ = msg.Ack()
					continue
				}

				payload, err := renderSling(&sling)
				if err != nil {
					log.Printf("Error rendering sling message: %v", err)
					_ = msg.Ack()
					continue
				}

				if err := s.nc.Publish(conn.reply, []byte(payload)); err != nil {
					log.Printf("Error sending websocket reply: %v", err)
				}
				_ = msg.Ack()
			}
		}
	}
}

func (s *service) stopWebsocketConsumer(reply string) {
	s.wsMu.Lock()
	conn, ok := s.wsConns[reply]
	if ok {
		delete(s.wsConns, reply)
		close(conn.stop)
	}
	s.wsMu.Unlock()

	if ok {
		_ = s.js.DeleteConsumer(conn.streamName, conn.consumerName)
	}
}

func renderSling(sling *slingmessage.SlingMessage) (string, error) {
	var buf bytes.Buffer
	id := sling.ID
	timestamp := sling.Timestamp
	if id == "" {
		if !timestamp.IsZero() {
			id = strconv.FormatInt(timestamp.UnixNano(), 10)
		} else {
			timestamp = time.Now().UTC()
			id = strconv.FormatInt(timestamp.UnixNano(), 10)
		}
	}
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	timestampLabel := timestamp.UTC().Format(time.RFC3339)

	switch {
	case strings.HasPrefix(sling.MimeType, "image/"):
		component := templates.SlingImage(
			id,
			sling.Sender,
			timestampLabel,
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
		component := templates.SlingCode(id, sling.Sender, timestampLabel, buffer.String())
		if err := component.Render(context.Background(), &buf); err != nil {
			return "", err
		}

	case strings.HasPrefix(sling.MimeType, "text/x-uri"):
		component := templates.SlingURL(id, sling.Sender, timestampLabel, string(sling.Content))
		if err := component.Render(context.Background(), &buf); err != nil {
			return "", err
		}

	case sling.MimeType == markdownMimeType || strings.HasPrefix(sling.MimeType, "text/x-markdown"):
		var mdBuffer bytes.Buffer
		if err := markdownRenderer.Convert(sling.Content, &mdBuffer); err != nil {
			return "", err
		}
		component := templates.SlingMarkdown(id, sling.Sender, timestampLabel, mdBuffer.String())
		if err := component.Render(context.Background(), &buf); err != nil {
			return "", err
		}

	case strings.HasPrefix(sling.MimeType, "text/"):
		component := templates.Sling(id, sling.Sender, timestampLabel, string(sling.Content))
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
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".md" || ext == ".markdown" {
		return markdownMimeType
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

func boardFromSubject(subject string, prefix string) (string, bool) {
	if !strings.HasPrefix(subject, prefix) {
		return "", false
	}

	segment := strings.TrimPrefix(subject, prefix)
	if segment == "" {
		return "", false
	}

	decoded, err := url.PathUnescape(segment)
	if err != nil {
		decoded = segment
	}

	board := normalizeBoardName(decoded)
	if board == "" {
		return "", false
	}

	return board, true
}

func boardFromCommandSubject(subject string) (string, bool) {
	if !strings.HasPrefix(subject, commandSubjectPrefix) {
		return "", false
	}

	board := strings.TrimPrefix(subject, commandSubjectPrefix)
	if board == "" {
		return "", false
	}

	board = normalizeBoardName(board)
	if board == "" {
		return "", false
	}

	return board, true
}

func normalizeBoardName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, char := range name {
		isAllowed := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_'
		if isAllowed {
			builder.WriteRune(char)
			lastDash = false
			continue
		}

		if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}
