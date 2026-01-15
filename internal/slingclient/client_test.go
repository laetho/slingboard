package slingclient

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Mattilsynet/h8s/pkg/h8sproxy"
	"github.com/laetho/slingboard/internal/commands"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type testHarness struct {
	natsServer *server.Server
	natsConn   *nats.Conn
	httpServer *httptest.Server
	baseURL    string
}

func startHarness(t *testing.T) *testHarness {
	t.Helper()

	natsServer, natsConn := startNATS(t)

	proxy := h8sproxy.NewH8Sproxy(natsConn)
	httpServer := httptest.NewServer(http.HandlerFunc(proxy.Handler))
	baseURL := strings.Replace(httpServer.URL, "127.0.0.1", "localhost", 1)

	t.Cleanup(func() {
		httpServer.Close()
		natsConn.Close()
		natsServer.Shutdown()
	})

	return &testHarness{
		natsServer: natsServer,
		natsConn:   natsConn,
		httpServer: httpServer,
		baseURL:    baseURL,
	}
}

func startNATS(t *testing.T) (*server.Server, *nats.Conn) {
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

func setupResponder(t *testing.T, nc *nats.Conn, handler func(req commands.CommandRequest)) {
	t.Helper()

	_, err := nc.Subscribe("h8s.http.POST.localhost.api.commands", func(msg *nats.Msg) {
		var req commands.CommandRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			resp, _ := json.Marshal(commands.CommandResponse{
				Status:  "error",
				Message: "bad payload",
			})
			msg.RespondMsg(&nats.Msg{
				Header: nats.Header{
					"Status-Code":    []string{"400"},
					"Content-Type":   []string{"application/json"},
					"Content-Length": []string{strconv.Itoa(len(resp))},
				},
				Data: resp,
			})
			return
		}

		handler(req)

		resp, _ := json.Marshal(commands.CommandResponse{Status: "ok"})
		msg.RespondMsg(&nats.Msg{
			Header: nats.Header{
				"Status-Code":    []string{"200"},
				"Content-Type":   []string{"application/json"},
				"Content-Length": []string{strconv.Itoa(len(resp))},
			},
			Data: resp,
		})

	})
	if err != nil {
		t.Fatalf("failed to subscribe to command subject: %v", err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatalf("failed to flush subscription: %v", err)
	}
}

func TestSendTextUsesH8SProxy(t *testing.T) {
	harness := startHarness(t)

	var got commands.CommandRequest
	setupResponder(t, harness.natsConn, func(req commands.CommandRequest) {
		got = req
	})

	client := NewClient(harness.baseURL)
	if err := client.SendText("testboard", "hello"); err != nil {
		t.Fatalf("send text failed: %v", err)
	}

	if got.Type != commands.CommandText {
		t.Fatalf("expected type text, got %q", got.Type)
	}
	if got.Board != "testboard" {
		t.Fatalf("expected board testboard, got %q", got.Board)
	}
	if got.Content != "hello" {
		t.Fatalf("expected content hello, got %q", got.Content)
	}
	if got.MimeType != "" || got.Filename != "" {
		t.Fatalf("expected text request to omit file metadata")
	}
}

func TestSendURLUsesH8SProxy(t *testing.T) {
	harness := startHarness(t)

	var got commands.CommandRequest
	setupResponder(t, harness.natsConn, func(req commands.CommandRequest) {
		got = req
	})

	client := NewClient(harness.baseURL)
	if err := client.SendURL("testboard", "https://example.com"); err != nil {
		t.Fatalf("send url failed: %v", err)
	}

	if got.Type != commands.CommandURL {
		t.Fatalf("expected type url, got %q", got.Type)
	}
	if got.Content != "https://example.com" {
		t.Fatalf("expected url content, got %q", got.Content)
	}
	if got.Board != "testboard" {
		t.Fatalf("expected board testboard, got %q", got.Board)
	}
}

func TestSendFileUsesH8SProxy(t *testing.T) {
	harness := startHarness(t)

	var got commands.CommandRequest
	setupResponder(t, harness.natsConn, func(req commands.CommandRequest) {
		got = req
	})

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "note.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	client := NewClient(harness.baseURL)
	if err := client.SendFile("testboard", filePath); err != nil {
		t.Fatalf("send file failed: %v", err)
	}

	if got.Type != commands.CommandFile {
		t.Fatalf("expected type file, got %q", got.Type)
	}
	if got.Board != "testboard" {
		t.Fatalf("expected board testboard, got %q", got.Board)
	}
	if got.Filename != "note.txt" {
		t.Fatalf("expected filename note.txt, got %q", got.Filename)
	}
	if got.MimeType != "text/plain; charset=utf-8" {
		t.Fatalf("expected mime type text/plain; charset=utf-8, got %q", got.MimeType)
	}

	decoded, err := base64.StdEncoding.DecodeString(got.Content)
	if err != nil {
		t.Fatalf("failed to decode base64 content: %v", err)
	}
	if string(decoded) != "hello" {
		t.Fatalf("expected decoded content hello, got %q", string(decoded))
	}
}

func TestBoardListUsesJSON(t *testing.T) {
	harness := startHarness(t)

	_, err := harness.natsConn.Subscribe("h8s.http.POST.localhost.api.commands", func(msg *nats.Msg) {
		var req commands.CommandRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return
		}
		if req.Type != commands.CommandBoardList {
			return
		}
		resp, _ := json.Marshal(commands.CommandResponse{Status: "ok", Boards: []string{"alpha", "beta"}})
		msg.RespondMsg(&nats.Msg{
			Header: nats.Header{
				"Status-Code":    []string{"200"},
				"Content-Type":   []string{"application/json"},
				"Content-Length": []string{strconv.Itoa(len(resp))},
			},
			Data: resp,
		})
	})
	if err != nil {
		t.Fatalf("failed to subscribe to command subject: %v", err)
	}
	if err := harness.natsConn.Flush(); err != nil {
		t.Fatalf("failed to flush subscription: %v", err)
	}

	client := NewClient(harness.baseURL)
	list, err := client.BoardList()
	if err != nil {
		t.Fatalf("board list failed: %v", err)
	}
	if len(list.Boards) != 2 {
		t.Fatalf("expected 2 boards, got %d", len(list.Boards))
	}
}

func TestBoardCreateUsesJSON(t *testing.T) {
	harness := startHarness(t)

	_, err := harness.natsConn.Subscribe("h8s.http.POST.localhost.api.commands", func(msg *nats.Msg) {
		var req commands.CommandRequest
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			return
		}
		if req.Type != commands.CommandBoardCreate {
			return
		}
		resp, _ := json.Marshal(commands.CommandResponse{Status: "ok", Board: req.Board})
		msg.RespondMsg(&nats.Msg{
			Header: nats.Header{
				"Status-Code":    []string{"200"},
				"Content-Type":   []string{"application/json"},
				"Content-Length": []string{strconv.Itoa(len(resp))},
			},
			Data: resp,
		})
	})
	if err != nil {
		t.Fatalf("failed to subscribe to command subject: %v", err)
	}
	if err := harness.natsConn.Flush(); err != nil {
		t.Fatalf("failed to flush subscription: %v", err)
	}

	client := NewClient(harness.baseURL)
	resp, err := client.BoardCreate("alpha")
	if err != nil {
		t.Fatalf("board create failed: %v", err)
	}
	if resp.Board != "alpha" {
		t.Fatalf("expected board alpha, got %q", resp.Board)
	}
}

func TestCommandErrorResponse(t *testing.T) {
	harness := startHarness(t)

	_, err := harness.natsConn.Subscribe("h8s.http.POST.localhost.api.commands", func(msg *nats.Msg) {
		resp, _ := json.Marshal(commands.CommandResponse{Status: "error", Message: "boom"})
		msg.RespondMsg(&nats.Msg{
			Header: nats.Header{
				"Status-Code":    []string{"200"},
				"Content-Type":   []string{"application/json"},
				"Content-Length": []string{strconv.Itoa(len(resp))},
			},
			Data: resp,
		})
	})
	if err != nil {
		t.Fatalf("failed to subscribe to command subject: %v", err)
	}
	if err := harness.natsConn.Flush(); err != nil {
		t.Fatalf("failed to flush subscription: %v", err)
	}

	client := NewClient(harness.baseURL)
	if err := client.SendText("testboard", "hello"); err == nil {
		t.Fatal("expected error response")
	}
}
