package graph_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/farulwananda/dataprobe/internal/graph"
	"github.com/farulwananda/dataprobe/internal/model"
)

func TestGraphServer_LocalLoopbackAndTokenEnforcement(t *testing.T) {
	snap := &model.Snapshot{
		SchemaVersion: 1,
		ToolVersion:   "0.1.0",
		CapturedAt:    time.Now().UTC(),
		Source: model.SourceMetadata{
			Engine:  "mysql",
			Schemas: []string{"test_db"},
		},
		Tables: []model.Table{
			{Schema: "test_db", Name: "users", Kind: "base_table"},
		},
	}

	server, err := graph.NewServer(graph.Config{
		Snapshot: snap,
		Port:     0, // Automatically pick free ephemeral port
	})
	if err != nil {
		t.Fatalf("graph.NewServer failed: %v", err)
	}
	defer server.Close()

	go func() {
		_ = server.Serve()
	}()

	client := &http.Client{Timeout: 3 * time.Second}

	// 1. Request without token -> should be rejected with 401
	respNoToken, err := client.Get(fmtSprintf("http://%s/api/snapshot", server.Addr()))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respNoToken.Body.Close()
	if respNoToken.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized without token, got %d", respNoToken.StatusCode)
	}

	// 2. Request with valid token -> should succeed with 200 and return snapshot
	viewerURL := server.URL()
	respValid, err := client.Get(viewerURL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer respValid.Body.Close()
	if respValid.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK with token URL, got %d", respValid.StatusCode)
	}

	// 3. Test static embedded asset serving
	staticURL := fmtSprintf("http://%s/static/vendor/cytoscape.min.js", server.Addr())
	respStatic, err := client.Get(staticURL)
	if err != nil {
		t.Fatalf("request static failed: %v", err)
	}
	defer respStatic.Body.Close()
	if respStatic.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for vendored cytoscape, got %d", respStatic.StatusCode)
	}
	staticBytes, _ := io.ReadAll(respStatic.Body)
	if len(staticBytes) < 100000 {
		t.Errorf("expected vendored cytoscape to be full bundle, got %d bytes", len(staticBytes))
	}
}

func fmtSprintf(format string, a ...any) string {
	var snap model.Snapshot
	_ = json.Unmarshal([]byte("{}"), &snap)
	return httpGetUrl(format, a...)
}

func httpGetUrl(format string, a ...any) string {
	res := format
	for _, val := range a {
		res = replaceFirst(res, "%s", val.(string))
	}
	return res
}

func replaceFirst(s, old, new string) string {
	i := 0
	for ; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}
