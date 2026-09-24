//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"logwolf-toolbox/data"
)

// TestStartup_FailedPassIsRetried boots the Logger against a database where one
// of its startup tasks cannot succeed: an index under the name the Logger uses,
// on other keys. The Logger used to log "will retry on the next start" and
// serve with the task undone for as long as it ran. Now it serves, reports the
// failure on /health, and retries in the background, so it recovers once the
// cause is gone, without a restart.
func TestStartup_FailedPassIsRetried(t *testing.T) {
	ctx := context.Background()
	mongoURI, client := migrationMongo(t)
	logs := client.Database("logs").Collection("logs")

	if _, err := logs.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetName("project_id_created_at"),
	}); err != nil {
		t.Fatalf("seed conflicting index: %v", err)
	}

	rpcAddr, httpAddr := freeAddr(t), freeAddr(t)
	startProcess(t, "../logger/cmd/api", map[string]string{
		"MONGO_URL":        mongoURI,
		"LOGGER_RPC_PORT":  portOf(rpcAddr),
		"LOGGER_HTTP_PORT": portOf(httpAddr),
		"CLEANUP_INTERVAL": "24h",
	})

	// It still serves.
	waitForTCP(t, rpcAddr, 60*time.Second)
	healthURL := "http://" + httpAddr + "/health"
	if err := waitHTTPStatus(healthURL, http.StatusServiceUnavailable, 30*time.Second); err != nil {
		t.Fatalf("degraded logger: %v", err)
	}
	code, st := loggerHealth(t, healthURL)
	if code != http.StatusServiceUnavailable || st.Ready || !strings.Contains(st.StartupError, "logs indexes") {
		t.Fatalf("/health = %d %+v, want 503 naming the logs indexes", code, st)
	}

	if _, err := logs.Indexes().DropOne(ctx, "project_id_created_at"); err != nil {
		t.Fatalf("drop conflicting index: %v", err)
	}

	// The first retry is 30s after the failed pass.
	if err := waitHTTPStatus(healthURL, http.StatusOK, 90*time.Second); err != nil {
		t.Fatalf("logger never recovered: %v", err)
	}
	code, st = loggerHealth(t, healthURL)
	if code != http.StatusOK || !st.Ready || st.StartupAttempts < 2 || st.StartupError != "" {
		t.Errorf("/health = %d %+v, want 200 after a retry", code, st)
	}
	if !hasIndex(t, client.Database("logs"), "logs", "project_id_created_at") {
		t.Error("the retry did not create the logs index")
	}
}

func loggerHealth(t *testing.T, url string) (int, data.LoggerStatus) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	var st data.LoggerStatus
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
	return resp.StatusCode, st
}

// waitHTTPStatus polls url until it answers with status.
func waitHTTPStatus(url string, status int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	last := 0
	for time.Now().Before(deadline) {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			last = resp.StatusCode
			if last == status {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("GET %s: never answered %d (last: %d)", url, status, last)
}
