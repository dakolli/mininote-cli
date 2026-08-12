package spec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestLoadForbidden exercises the documented file format: comment lines, blank
// lines, exact case preservation, and deduplication.
func TestLoadForbidden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "forbidden.txt")
	content := `# header comment — must be ignored
Page.listPrefix   # trailing comment after route is NOT part of the route
workspace.forKey

auth.me
Page.listPrefix
AUTH.me
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadForbidden(path)
	if err != nil {
		t.Fatalf("LoadForbidden: %v", err)
	}

	want := map[string]bool{
		"Page.listPrefix":  true,
		"workspace.forKey": true,
		"auth.me":          true,
		"AUTH.me":          true, // case is preserved, not folded
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadForbidden = %v, want %v", got, want)
	}
}

// TestLoadForbiddenCaptureFormat ensures the generator also tolerates raw
// 403routes.txt-style lines ("403 FORBIDDEN Service.method message") pasted
// into the file — the route is the third whitespace field.
func TestLoadForbiddenCaptureFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "forbidden.txt")
	content := `  403  FORBIDDEN     Page.resolveKey                            this endpoint is not available to API keys
403 FORBIDDEN Admin.stats this endpoint is not available to API keys
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadForbidden(path)
	if err != nil {
		t.Fatalf("LoadForbidden: %v", err)
	}
	want := map[string]bool{
		"Page.resolveKey": true,
		"Admin.stats":     true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadForbidden = %v, want %v", got, want)
	}
}

// TestLoadForbiddenMissing reports the error for a nonexistent file.
func TestLoadForbiddenMissing(t *testing.T) {
	if _, err := LoadForbidden(filepath.Join(t.TempDir(), "nope.txt")); err == nil {
		t.Fatal("LoadForbidden: expected error for missing file, got nil")
	}
}

// TestPruneForbidden covers the prune step: matching methods are dropped,
// types are untouched, removed services and stale entries are reported.
func TestPruneForbidden(t *testing.T) {
	m := &Model{
		Types: []TypeInfo{{Name: "Page", GoName: "Page"}},
		Methods: []MethodInfo{
			{Service: "Activity", Method: "view", GoName: "ActivityView", PostPath: "/rpc/Activity/view"},
			{Service: "Auth", Method: "me", GoName: "AuthMe", PostPath: "/rpc/Auth/me"},
			{Service: "Page", Method: "tree", GoName: "PageTree", PostPath: "/rpc/Page/tree"},
			{Service: "Page", Method: "resolveKey", GoName: "PageResolveKey", PostPath: "/rpc/Page/resolveKey"},
			{Service: "Page", Method: "listPrefix", GoName: "PageListPrefix", PostPath: "/rpc/Page/listPrefix"},
		},
	}
	forbidden := map[string]bool{
		"Activity.view":   true,
		"Auth.me":         true,
		"Page.resolveKey": true,
		"Ghost.method":    true, // stale: matches no method
	}

	report := m.PruneForbidden(forbidden)

	if report.Total != 5 || report.Pruned != 3 || report.Kept != 2 {
		t.Errorf("counts = Total %d, Pruned %d, Kept %d; want 5, 3, 2", report.Total, report.Pruned, report.Kept)
	}
	if want := []string{"Activity", "Auth"}; !reflect.DeepEqual(report.RemovedServices, want) {
		t.Errorf("RemovedServices = %v, want %v", report.RemovedServices, want)
	}
	if want := []string{"Ghost.method"}; !reflect.DeepEqual(report.Stale, want) {
		t.Errorf("Stale = %v, want %v", report.Stale, want)
	}

	var kept []string
	for _, mi := range m.Methods {
		kept = append(kept, mi.Service+"."+mi.Method)
	}
	if want := []string{"Page.tree", "Page.listPrefix"}; !reflect.DeepEqual(kept, want) {
		t.Errorf("surviving methods = %v, want %v", kept, want)
	}

	// Types are never pruned.
	if len(m.Types) != 1 || m.Types[0].GoName != "Page" {
		t.Errorf("types were pruned: %+v", m.Types)
	}
}

// TestPruneForbiddenNoStale ensures a perfectly current list yields no stale
// entries, and that a non-destructive run still reports zero removed services.
func TestPruneForbiddenNoStale(t *testing.T) {
	m := &Model{
		Methods: []MethodInfo{
			{Service: "Page", Method: "tree", GoName: "PageTree"},
			{Service: "Page", Method: "listPrefix", GoName: "PageListPrefix"},
		},
	}
	forbidden := map[string]bool{"Page.tree": true}

	report := m.PruneForbidden(forbidden)
	if len(report.Stale) != 0 {
		t.Errorf("Stale = %v, want none", report.Stale)
	}
	if len(report.RemovedServices) != 0 {
		t.Errorf("RemovedServices = %v, want none (Page.listPrefix survives)", report.RemovedServices)
	}
	if report.Pruned != 1 || report.Kept != 1 {
		t.Errorf("counts = %d/%d, want 1/1", report.Pruned, report.Kept)
	}
}

// TestFetchSpec verifies the live-capture request shape against an httptest
// server: POST, empty body, content type, and bearer header.
func TestFetchSpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		body := make([]byte, 2)
		r.Body.Read(body)
		if string(body) != "{}" {
			t.Errorf("body = %q, want {}", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"services":{"Page":{"router":"","title":"","methods":[]}},"types":{}}`))
	}))
	defer srv.Close()

	raw, err := FetchSpec(context.Background(), srv.URL, "test-token")
	if err != nil {
		t.Fatalf("FetchSpec: %v", err)
	}
	if !strings.Contains(string(raw), `"services"`) {
		t.Errorf("unexpected body: %s", raw)
	}
}

// TestFetchSpecRejectsNon2xx ensures an upstream error surfaces as an error.
func TestFetchSpecRejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"message":"nope","code":"FORBIDDEN"}}`))
	}))
	defer srv.Close()

	if _, err := FetchSpec(context.Background(), srv.URL, ""); err == nil {
		t.Fatal("FetchSpec: expected error for 403, got nil")
	}
}

// TestCaptureSpec verifies the capture path: pretty-printed JSON written to
// the output file (recreating it), and service/type counts returned.
func TestCaptureSpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"services":{"Page":{"router":"","title":"","methods":[]}},"types":{"Page":{"name":"Page","fields":[]}}}`))
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "intro.json")
	// Pre-existing content must be replaced on every capture.
	if err := os.WriteFile(out, []byte("STALE"), 0o644); err != nil {
		t.Fatal(err)
	}

	svcs, types, err := CaptureSpec(context.Background(), srv.URL, "", out)
	if err != nil {
		t.Fatalf("CaptureSpec: %v", err)
	}
	if svcs != 1 || types != 1 {
		t.Errorf("counts = services %d, types %d; want 1, 1", svcs, types)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "{\n  \"services\"") {
		t.Errorf("output is not pretty-printed JSON: %s", raw)
	}
}

// TestCaptureSpecRefusesInvalidJSON ensures a garbage 200 body does not
// clobber the existing (committed) intro.json.
func TestCaptureSpecRefusesInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json at all"))
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "intro.json")
	if err := os.WriteFile(out, []byte("KEEP ME"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := CaptureSpec(context.Background(), srv.URL, "", out); err == nil {
		t.Fatal("CaptureSpec: expected error for invalid JSON, got nil")
	}
	raw, _ := os.ReadFile(out)
	if string(raw) != "KEEP ME" {
		t.Errorf("output file was clobbered: %q", raw)
	}
}
