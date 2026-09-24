package event

import (
	"errors"
	"logwolf-toolbox/data"
	"net"
	"net/rpc"
	"time"
)

const (
	// loggerDialTimeout bounds connecting to the logger, so an unreachable one
	// fails the attempt instead of hanging it.
	loggerDialTimeout = 5 * time.Second

	// loggerCallTimeout bounds one LogInfo call. net/rpc has no deadline of its
	// own, and a logger that accepts but never answers would otherwise stall the
	// queue for good.
	loggerCallTimeout = 30 * time.Second
)

var errLoggerCallTimeout = errors.New("logger RPC call timed out")

// logSink is where the consumer sends events: the logger, or a fake in tests.
type logSink interface {
	LogInfo(p data.RPCLogPayload) error
}

// loggerClient is the listener's one connection to the logger's RPC server. It
// dials on first use and reuses the connection for every event after that,
// dialing again only once it breaks. Dialing per event, and never closing the
// client, used to leak a socket and its goroutines on both ends for every event.
//
// It is not safe for concurrent use; the consumer handles one event at a time.
type loggerClient struct {
	addr   string
	client *rpc.Client
}

func newLoggerClient(addr string) *loggerClient {
	return &loggerClient{addr: addr}
}

// LogInfo stores one event through RPCServer.LogInfo. An error the logger
// answered with comes back as an rpc.ServerError and leaves the connection in
// place; any other error means the connection is unusable, so it is closed and
// the next call dials afresh.
func (l *loggerClient) LogInfo(p data.RPCLogPayload) error {
	if l.client == nil {
		conn, err := net.DialTimeout("tcp", l.addr, loggerDialTimeout)
		if err != nil {
			return err
		}
		l.client = rpc.NewClient(conn)
	}

	var reply string
	var err error
	call := l.client.Go("RPCServer.LogInfo", p, &reply, nil)
	timer := time.NewTimer(loggerCallTimeout)
	select {
	case <-call.Done:
		err = call.Error
	case <-timer.C:
		err = errLoggerCallTimeout
	}
	timer.Stop()

	var refused rpc.ServerError
	if err != nil && !errors.As(err, &refused) {
		l.Close()
	}
	return err
}

// Close closes the connection, if there is one.
func (l *loggerClient) Close() error {
	if l.client == nil {
		return nil
	}
	err := l.client.Close()
	l.client = nil
	return err
}
