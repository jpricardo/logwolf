package event

import (
	"errors"
	"logwolf-toolbox/data"
	"net"
	"net/rpc"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeLogger stands in for the logger's RPCServer.
type fakeLogger struct{ refuse bool }

func (f *fakeLogger) LogInfo(p data.RPCLogPayload, reply *string) error {
	if f.refuse {
		return data.ErrUnknownProject
	}
	*reply = "ok"
	return nil
}

// rpcServer serves fakeLogger on a local port and counts the connections it
// accepts. drop closes every connection it has accepted, as a logger restart
// would.
type rpcServer struct {
	addr     string
	accepted atomic.Int32

	mu    sync.Mutex
	conns []net.Conn
}

func startRPCServer(t *testing.T, logger *fakeLogger) *rpcServer {
	t.Helper()
	srv := rpc.NewServer()
	if err := srv.RegisterName("RPCServer", logger); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &rpcServer{addr: l.Addr().String()}
	t.Cleanup(func() { l.Close(); s.drop() })

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			s.accepted.Add(1)
			s.mu.Lock()
			s.conns = append(s.conns, conn)
			s.mu.Unlock()
			go srv.ServeConn(conn)
		}
	}()
	return s
}

func (s *rpcServer) drop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		c.Close()
	}
	s.conns = nil
}

// TestLoggerClient_ReusesOneConnection is the leak: every event used to dial the
// logger and never close the client.
func TestLoggerClient_ReusesOneConnection(t *testing.T) {
	srv := startRPCServer(t, &fakeLogger{})
	c := newLoggerClient(srv.addr)
	defer c.Close()

	for i := range 50 {
		if err := c.LogInfo(data.RPCLogPayload{Name: "evt"}); err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
	}
	if n := srv.accepted.Load(); n != 1 {
		t.Errorf("50 events opened %d connections, want 1", n)
	}
}

// TestLoggerClient_RedialsAfterTheConnectionBreaks: a logger restart costs one
// failed call, which the consumer retries, and the next one reconnects.
func TestLoggerClient_RedialsAfterTheConnectionBreaks(t *testing.T) {
	srv := startRPCServer(t, &fakeLogger{})
	c := newLoggerClient(srv.addr)
	defer c.Close()

	if err := c.LogInfo(data.RPCLogPayload{}); err != nil {
		t.Fatal(err)
	}
	srv.drop()

	var err error
	for range 3 {
		if err = c.LogInfo(data.RPCLogPayload{}); err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("still failing after the connection broke: %v", err)
	}
	if n := srv.accepted.Load(); n != 2 {
		t.Errorf("accepted %d connections, want 2 (one redial)", n)
	}
}

// TestLoggerClient_RefusalKeepsTheConnection: an error the logger answered with
// says nothing about the connection, so it is kept.
func TestLoggerClient_RefusalKeepsTheConnection(t *testing.T) {
	srv := startRPCServer(t, &fakeLogger{refuse: true})
	c := newLoggerClient(srv.addr)
	defer c.Close()

	for range 3 {
		err := c.LogInfo(data.RPCLogPayload{})
		var refused rpc.ServerError
		if !errors.As(err, &refused) {
			t.Fatalf("want an rpc.ServerError, got %v", err)
		}
	}
	if n := srv.accepted.Load(); n != 1 {
		t.Errorf("refusals opened %d connections, want 1", n)
	}
}

func TestLoggerClient_UnreachableLogger(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	c := newLoggerClient(addr)
	err = c.LogInfo(data.RPCLogPayload{})
	var refused rpc.ServerError
	if err == nil || errors.As(err, &refused) {
		t.Errorf("want a connection error, got %v", err)
	}
}
