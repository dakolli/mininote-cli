package spec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	// IntrospectPath is the RPC route that publishes the API catalog.
	IntrospectPath = "/rpc/_introspect"
	// DefaultIntrospectURL is the default live-capture endpoint used by the
	// generators' -introspect flag.
	DefaultIntrospectURL = "https://mininote.ink" + IntrospectPath
)

// FetchSpec POSTs an empty JSON body to the introspection route and returns the
// raw response body. When token is non-empty it is sent as a Bearer header.
func FetchSpec(ctx context.Context, url, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(body, 300))
	}
	return body, nil
}

// CaptureSpec fetches the live catalog and writes it pretty-printed to outPath,
// recreating the file on every generation. It refuses to overwrite outPath when
// the captured body is not valid JSON, and returns the captured spec's service
// and type counts for logging.
func CaptureSpec(ctx context.Context, url, token, outPath string) (services, types int, err error) {
	raw, err := FetchSpec(ctx, url, token)
	if err != nil {
		return 0, 0, err
	}
	if !json.Valid(raw) {
		return 0, 0, fmt.Errorf("introspect response is not valid JSON (%d bytes)", len(raw))
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		return 0, 0, fmt.Errorf("re-indent spec: %w", err)
	}

	var meta struct {
		Services map[string]json.RawMessage `json:"services"`
		Types    map[string]json.RawMessage `json:"types"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return 0, 0, fmt.Errorf("summarize spec: %w", err)
	}

	if err := os.WriteFile(outPath, pretty.Bytes(), 0o644); err != nil {
		return 0, 0, err
	}
	return len(meta.Services), len(meta.Types), nil
}

// SpecToken resolves the bearer token for live captures with the same env
// precedence as the runtime client: MININOTE_ADMIN_AGENT_KEY (admin keys see
// the widest surface), then MININOTE_RPC_KEY, then MININOTE_TOKEN.
func SpecToken() string {
	for _, name := range []string{"MININOTE_ADMIN_AGENT_KEY", "MININOTE_RPC_KEY", "MININOTE_TOKEN"} {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}

// StaleSpecWarning logs the loud banner shown when a generation consumes the
// committed intro.json (explicit -in) instead of a fresh live capture. It is a
// warning, never fatal — the committed file remains the offline fallback.
func StaleSpecWarning(path string) {
	log.Printf("WARNING: STALE SPEC — generation did not re-capture intro.json (used %s)", path)
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	return string(b)
}
