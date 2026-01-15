package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/laetho/slingboard/internal/commands"
	"github.com/laetho/slingboard/internal/slingmessage"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func startTestNATS(t *testing.T) (*server.Server, *nats.Conn) {
	t.Helper()

	opts := &server.Options{Port: -1}
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

	svc := newService(nc)
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
}

func TestCommandsResponderPublishesMessage(t *testing.T) {
	srv, nc := startTestNATS(t)
	defer srv.Shutdown()
	defer nc.Close()

	svc := startService(t, nc)
	defer svc.shutdown()

	msgCh := make(chan *nats.Msg, 1)
	sub, err := nc.Subscribe(commandSubject, func(msg *nats.Msg) {
		msgCh <- msg
	})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	payload, _ := json.Marshal(commands.CommandRequest{
		Type:    commands.CommandText,
		Board:   "global",
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

	select {
	case msg := <-msgCh:
		var sling slingmessage.SlingMessage
		if err := json.Unmarshal(msg.Data, &sling); err != nil {
			t.Fatalf("invalid sling message: %v", err)
		}
		if string(sling.Content) != "hello" {
			t.Fatalf("expected content to match, got %q", string(sling.Content))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected sling message publish")
	}
}

func waitForWebsocketReply(t *testing.T, svc *service, reply string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if svc.hasWebsocketReply(reply) {
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

	replySubject := "_INBOX.test"
	wsCh := make(chan *nats.Msg, 1)
	wsSub, err := nc.Subscribe(replySubject, func(msg *nats.Msg) {
		wsCh <- msg
	})
	if err != nil {
		t.Fatalf("failed to subscribe to websocket inbox: %v", err)
	}
	defer wsSub.Unsubscribe()

	ctrl := &nats.Msg{
		Subject: websocketEstablished,
		Reply:   replySubject,
		Header:  nats.Header{websocketPublishHeader: []string{websocketSubject}},
	}
	if err := nc.PublishMsg(ctrl); err != nil {
		t.Fatalf("failed to publish control: %v", err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatalf("failed to flush control: %v", err)
	}

	waitForWebsocketReply(t, svc, replySubject)

	slingPayload, _ := json.Marshal(slingmessage.SlingMessage{
		MimeType: "text/plain",
		Content:  []byte("hello"),
	})
	if err := nc.Publish(commandSubject, slingPayload); err != nil {
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
