package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/laetho/slingboard/internal/commands"
	"github.com/laetho/slingboard/internal/slingmessage"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func startTestNATS(t *testing.T) (*server.Server, *nats.Conn) {
	t.Helper()

	opts := &server.Options{Port: -1, JetStream: true}
	srv, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("failed to create nats server: %v", err)
	}

	go srv.Start()
	if !srv.ReadyForConnections(10 * time.Second) {
		t.Fatal("nats server not ready")
	}

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		srv.Shutdown()
		t.Fatalf("failed to connect to nats: %v", err)
	}

	return srv, nc
}

func startService(t *testing.T, nc *nats.Conn) *service {
	t.Helper()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("failed to create JetStream context: %v", err)
	}

	configureSubjects()
	indexSubject = strings.ToUpper(indexSubject)
	boardSubjectPrefix = strings.ToUpper(boardSubjectPrefix)
	boardSubjectWildcard = strings.ToUpper(boardSubjectWildcard)
	commandsSubject = strings.ToUpper(commandsSubject)
	styleSubject = strings.ToUpper(styleSubject)
	websocketSubjectPrefix = strings.ToUpper(websocketSubjectPrefix)

	svc := newService(nc, js)
	if err := svc.register(); err != nil {
		t.Fatalf("failed to register service: %v", err)
	}

	return svc
}

func TestIndexResponder(t *testing.T) {
	srv, nc := startTestNATS(t)
	defer srv.Shutdown()
	defer nc.Close()

	svc := startService(t, nc)
	defer svc.shutdown()

	if _, err := svc.ensureBoardStream("testboard"); err != nil {
		t.Fatalf("failed to create board stream: %v", err)
	}

	resp, err := nc.Request(indexSubject, nil, 2*time.Second)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if got := resp.Header.Get("Status-Code"); got != "200" {
		t.Fatalf("expected status 200, got %q", got)
	}
	if got := resp.Header.Get("Content-Type"); got != contentTypeHTML {
		t.Fatalf("expected content type %q, got %q", contentTypeHTML, got)
	}
	if got := resp.Header.Get("Cache-Control"); got != noCacheHeader {
		t.Fatalf("expected cache-control %q, got %q", noCacheHeader, got)
	}
	if len(resp.Data) == 0 {
		t.Fatal("expected html body")
	}
	if !strings.Contains(string(resp.Data), "testboard") {
		t.Fatal("expected board list to include testboard")
	}
}

func TestBoardResponder(t *testing.T) {
	srv, nc := startTestNATS(t)
	defer srv.Shutdown()
	defer nc.Close()

	svc := startService(t, nc)
	defer svc.shutdown()

	resp, err := nc.Request(boardSubjectPrefix+"testboard", nil, 2*time.Second)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if got := resp.Header.Get("Status-Code"); got != "200" {
		t.Fatalf("expected status 200, got %q", got)
	}
	if !strings.Contains(string(resp.Data), "testboard") {
		t.Fatal("expected board name in response")
	}
}

func TestCommandsResponderPublishesMessage(t *testing.T) {
	srv, nc := startTestNATS(t)
	defer srv.Shutdown()
	defer nc.Close()

	svc := startService(t, nc)
	defer svc.shutdown()

	payload, _ := json.Marshal(commands.CommandRequest{
		Type:    commands.CommandText,
		Board:   "testboard",
		Content: "hello",
	})

	resp, err := nc.Request(commandsSubject, payload, 2*time.Second)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	var commandResp commands.CommandResponse
	if err := json.Unmarshal(resp.Data, &commandResp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if commandResp.Status != "ok" {
		t.Fatalf("expected ok status, got %q", commandResp.Status)
	}
}

func TestCommandsBoardListJSON(t *testing.T) {
	srv, nc := startTestNATS(t)
	defer srv.Shutdown()
	defer nc.Close()

	svc := startService(t, nc)
	defer svc.shutdown()

	if _, err := svc.ensureBoardStream("alpha"); err != nil {
		t.Fatalf("failed to create board stream: %v", err)
	}
	if _, err := svc.ensureBoardStream("beta"); err != nil {
		t.Fatalf("failed to create board stream: %v", err)
	}

	payload, _ := json.Marshal(commands.CommandRequest{Type: commands.CommandBoardList})
	req := &nats.Msg{Subject: commandsSubject, Data: payload, Header: nats.Header{"Accept": []string{"application/json"}}}
	resp, err := nc.RequestMsg(req, 2*time.Second)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	var commandResp commands.CommandResponse
	if err := json.Unmarshal(resp.Data, &commandResp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	foundAlpha := false
	foundBeta := false
	for _, board := range commandResp.Boards {
		if board == "alpha" {
			foundAlpha = true
		}
		if board == "beta" {
			foundBeta = true
		}
	}
	if !foundAlpha || !foundBeta {
		t.Fatalf("expected boards alpha and beta, got %v", commandResp.Boards)
	}
}

func TestCommandsBoardCreateJSON(t *testing.T) {
	srv, nc := startTestNATS(t)
	defer srv.Shutdown()
	defer nc.Close()

	svc := startService(t, nc)
	defer svc.shutdown()

	payload, _ := json.Marshal(commands.CommandRequest{Type: commands.CommandBoardCreate, Board: "alpha"})
	req := &nats.Msg{Subject: commandsSubject, Data: payload, Header: nats.Header{"Accept": []string{"application/json"}}}
	resp, err := nc.RequestMsg(req, 2*time.Second)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	var commandResp commands.CommandResponse
	if err := json.Unmarshal(resp.Data, &commandResp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if commandResp.Board != "alpha" {
		t.Fatalf("expected board alpha, got %q", commandResp.Board)
	}
	if _, err := svc.js.StreamInfo(streamPrefix + "alpha"); err != nil {
		t.Fatalf("expected stream to exist: %v", err)
	}
}

func waitForWebsocketReply(t *testing.T, svc *service, board string, reply string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if svc.hasWebsocketReply(board, reply) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("websocket reply not registered")
}

func TestWebsocketBroadcast(t *testing.T) {
	srv, nc := startTestNATS(t)
	defer srv.Shutdown()
	defer nc.Close()

	svc := startService(t, nc)
	defer svc.shutdown()

	if _, err := svc.ensureBoardStream("testboard"); err != nil {
		t.Fatalf("failed to create board stream: %v", err)
	}

	replySubject := "_INBOX.test"
	wsCh := make(chan *nats.Msg, 1)
	wsSub, err := nc.Subscribe(replySubject, func(msg *nats.Msg) {
		wsCh <- msg
	})
	if err != nil {
		t.Fatalf("failed to subscribe to websocket inbox: %v", err)
	}
	defer wsSub.Unsubscribe()

	boardSubject := websocketSubjectPrefix + "testboard"
	ctrl := &nats.Msg{
		Subject: websocketEstablished,
		Reply:   replySubject,
		Header:  nats.Header{websocketPublishHeader: []string{boardSubject}},
	}
	if err := nc.PublishMsg(ctrl); err != nil {
		t.Fatalf("failed to publish control: %v", err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatalf("failed to flush control: %v", err)
	}

	waitForWebsocketReply(t, svc, "testboard", replySubject)

	slingPayload, _ := json.Marshal(slingmessage.SlingMessage{
		MimeType: "text/plain",
		Content:  []byte("hello"),
	})
	if err := nc.Publish(commandSubjectPrefix+"testboard", slingPayload); err != nil {
		t.Fatalf("failed to publish sling: %v", err)
	}

	select {
	case msg := <-wsCh:
		if len(msg.Data) == 0 {
			t.Fatal("expected websocket payload")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected websocket broadcast")
	}
}

func TestMarkdownRender(t *testing.T) {
	sling := slingmessage.SlingMessage{
		MimeType: markdownMimeType,
		Sender:   "tester",
		Content:  []byte("# Title\n\nSome text"),
	}

	payload, err := renderSling(&sling)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if !strings.Contains(payload, "<h1") {
		t.Fatalf("expected markdown header in output: %s", payload)
	}
}
