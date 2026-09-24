//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
)

// TestListener_KeepsEventsThroughALoggerOutage takes the Logger down between
// the Broker accepting an event and the Listener delivering it. The Listener
// used to acknowledge every message on receipt, so an event that met a dead
// Logger was gone; now it stays unacknowledged, is retried, and is stored once
// the Logger is back.
//
// It runs a stack of its own: stopping the shared Logger would break every
// other test, and a second Listener on the shared RabbitMQ would compete for
// the same queue.
func TestListener_KeepsEventsThroughALoggerOutage(t *testing.T) {
	ctx := context.Background()
	mongoURI := dedicatedMongo(t)

	rabbitC, err := rabbitmq.Run(ctx, "rabbitmq:3.9-alpine")
	if err != nil {
		t.Fatalf("rabbitmq container: %v", err)
	}
	t.Cleanup(func() { rabbitC.Terminate(context.Background()) })
	rabbitURI, _ := rabbitC.AmqpURL(ctx)

	projectID := seedProject(t, mongoURI, "logger-outage")
	const keyStem = "lw_loggeroutage"
	key := seedAPIKey(t, mongoURI, projectID, keyStem+strings.Repeat("0", 46-len(keyStem)-1)+"1")

	loggerRPCAddr := freeAddr(t)
	loggerEnv := map[string]string{
		"MONGO_URL":        mongoURI,
		"LOGGER_RPC_PORT":  portOf(loggerRPCAddr),
		"LOGGER_HTTP_PORT": portOf(freeAddr(t)),
		"CLEANUP_INTERVAL": "24h",
	}
	stopLogger := startProcess(t, "../logger/cmd/api", loggerEnv)
	waitForTCP(t, loggerRPCAddr, 60*time.Second)

	startProcess(t, "../listener/cmd/api", map[string]string{
		"RABBITMQ_URL":    rabbitURI,
		"LOGGER_RPC_ADDR": loggerRPCAddr,
	})
	brokerHTTPAddr := freeAddr(t)
	startProcess(t, "../broker/cmd/api", map[string]string{
		"RABBITMQ_URL":        rabbitURI,
		"LOGGER_RPC_ADDR":     loggerRPCAddr,
		"BROKER_PORT":         portOf(brokerHTTPAddr),
		"INTERNAL_API_SECRET": internalSecret,
	})
	brokerURL := "http://" + brokerHTTPAddr
	if err := waitHTTP(brokerURL+"/ping", 60*time.Second); err != nil {
		t.Fatalf("broker: %v", err)
	}
	if err := waitForPipeline(mongoURI, brokerURL, 60*time.Second); err != nil {
		t.Fatal(err)
	}

	// Stored while the Logger is up. It also puts the key in the Broker's
	// cache, so the Broker can accept the next event without the Logger.
	postLog(t, brokerURL, key, "outage-before")
	waitForLog(t, mongoURI, "outage-before")

	stopLogger()
	postLog(t, brokerURL, key, "outage-during")

	// Long enough for the Listener to have met the dead Logger and backed off.
	time.Sleep(3 * time.Second)

	startProcess(t, "../logger/cmd/api", loggerEnv)
	waitForTCP(t, loggerRPCAddr, 60*time.Second)

	waitForLog(t, mongoURI, "outage-during")
}
