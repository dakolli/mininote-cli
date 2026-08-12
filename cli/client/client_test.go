package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSmoke(t *testing.T) {
	var treeBody, tagListBody, shareMineBody []byte
	var treeAuthHeader string

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc/Page/tree", func(w http.ResponseWriter, r *http.Request) {
		treeBody, _ = io.ReadAll(r.Body)
		treeAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"pages":[{"id":"p1"}]}}`))
	})
	mux.HandleFunc("/rpc/Tag/delete", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"p1": true, "p2": false}}`))
	})
	mux.HandleFunc("/rpc/Tag/list", func(w http.ResponseWriter, r *http.Request) {
		tagListBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"tags": [{"id":"a"},{"id":"b"}]}}`))
	})
	mux.HandleFunc("/rpc/Share/mine", func(w http.ResponseWriter, r *http.Request) {
		shareMineBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"internal error","code":"INTERNAL"}}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := New(srv.URL, WithToken("tok"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	t.Run("PageTree decodes typed result and sends token", func(t *testing.T) {
		tree, err := c.PageTree(ctx)
		if err != nil {
			t.Fatalf("PageTree: %v", err)
		}
		if len(tree.Pages) != 1 || tree.Pages[0].ID == nil || *tree.Pages[0].ID != "p1" {
			t.Fatalf("tree.Pages = %+v, want one page with id %q", tree.Pages, "p1")
		}
		if treeAuthHeader != "Bearer tok" {
			t.Fatalf("Authorization = %q, want %q", treeAuthHeader, "Bearer tok")
		}
		if string(treeBody) != `{"args":{}}` {
			t.Fatalf("request body = %q, want %q", treeBody, `{"args":{}}`)
		}
	})

	t.Run("TagDelete decodes map result", func(t *testing.T) {
		res, err := c.TagDelete(ctx, TagDeleteParams{ID: "tag_1"})
		if err != nil {
			t.Fatalf("TagDelete: %v", err)
		}
		if len(res) != 2 || !res["p1"] || res["p2"] {
			t.Fatalf("result = %v, want {p1:true, p2:false}", res)
		}
	})

	t.Run("TagList sends args envelope and decodes TagCatalogResult", func(t *testing.T) {
		res, err := c.TagList(ctx)
		if err != nil {
			t.Fatalf("TagList: %v", err)
		}
		if len(res.Tags) != 2 || res.Tags[0].ID == nil || *res.Tags[0].ID != "a" {
			t.Fatalf("tags = %+v, want two tags starting with id %q", res.Tags, "a")
		}
		if string(tagListBody) != `{"args":{}}` {
			t.Fatalf("request body = %q, want %q (void request)", tagListBody, `{"args":{}}`)
		}
	})

	t.Run("ShareMine returns APIError on 500", func(t *testing.T) {
		_, err := c.ShareMine(ctx)
		var aerr *APIError
		if !errors.As(err, &aerr) {
			t.Fatalf("err = %v (%T), want *APIError", err, err)
		}
		if aerr.StatusCode != http.StatusInternalServerError {
			t.Fatalf("StatusCode = %d, want 500", aerr.StatusCode)
		}
		if !strings.Contains(aerr.Message, "internal error") {
			t.Fatalf("Message = %q, want it to contain %q", aerr.Message, "internal error")
		}
		if aerr.Code != "INTERNAL" {
			t.Fatalf("Code = %q, want %q", aerr.Code, "INTERNAL")
		}
		if string(shareMineBody) != `{"args":{}}` {
			t.Fatalf("request body = %q, want %q (void request)", shareMineBody, `{"args":{}}`)
		}
	})
}

// TestSessionOnlyBlockedForAPIKey verifies the client-side fast-fail: an API
// key never sends a request to a sessionOnly route. The generated client is
// strictly key-available, so no generated method targets a sessionOnly route
// anymore — the guard is exercised through the unexported do path, which is the
// exact code every (future) method would go through.
func TestSessionOnlyBlockedForAPIKey(t *testing.T) {
	c, err := New("http://example.invalid", WithAPIKey("mnk_somekey"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	t.Run("sessionOnly routes blocked client-side", func(t *testing.T) {
		for _, path := range []string{"/rpc/Auth/login", "/rpc/Auth/me", "/rpc/Auth/createAPIKey"} {
			var resp any
			err := c.do(ctx, path, struct{}{}, &resp)
			var aerr *APIError
			if !errors.As(err, &aerr) {
				t.Fatalf("%s: err = %v (%T), want *APIError", path, err, err)
			}
			if aerr.StatusCode != http.StatusForbidden {
				t.Fatalf("%s: StatusCode = %d, want 403", path, aerr.StatusCode)
			}
			if !strings.Contains(aerr.Message, "not available to API keys") {
				t.Fatalf("%s: Message = %q, want mention of API keys", path, aerr.Message)
			}
		}
	})

	t.Run("PageTree still allowed with API key", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/rpc/Page/tree", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"pages":[]}}`))
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()
		c, err := New(srv.URL, WithAPIKey("mnk_somekey"))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if _, err := c.PageTree(ctx); err != nil {
			t.Fatalf("PageTree with API key should be allowed: %v", err)
		}
	})
}
