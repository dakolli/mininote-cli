package client

// Live integration tests against a real mininote server.
//
// These hit the network and are skipped unless MININOTE_RPC_KEY is set (the
// base URL comes from MININOTE_BASE_URL, defaulting to https://mininote.ink).
// They cover the key-reachable routes only: control-plane endpoints are
// rejected for API keys and are asserted to be blocked.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func newIntegrationClient(t *testing.T) *Client {
	t.Helper()
	key := os.Getenv("MININOTE_RPC_KEY")
	if key == "" {
		t.Skip("MININOTE_RPC_KEY not set; skipping live integration test")
	}
	baseURL := os.Getenv("MININOTE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://mininote.ink"
	}
	c, err := New(baseURL, WithAPIKey(key))
	if err != nil {
		t.Fatalf("New(%q): %v", baseURL, err)
	}
	return c
}

// firstPageID returns the first page from Page.tree, or skips the test.
func firstPageID(t *testing.T, c *Client) string {
	t.Helper()
	tree, err := c.PageTree(context.Background())
	if err != nil {
		t.Fatalf("PageTree: %v", err)
	}
	if len(tree.Pages) == 0 {
		t.Skip("no pages in tree; nothing to anchor page tests on")
	}
	if tree.Pages[0].ID == nil || *tree.Pages[0].ID == "" {
		t.Fatalf("first page has no id: %+v", tree.Pages[0])
	}
	return *tree.Pages[0].ID
}

func TestIntegrationVoidReads(t *testing.T) {
	c := newIntegrationClient(t)
	ctx := context.Background()

	if res, err := c.TagList(ctx); err != nil {
		t.Fatalf("TagList: %v", err)
	} else if res.Tags == nil {
		t.Fatal("TagList: nil Tags")
	}

	if _, err := c.ShareApiDocs(ctx); err != nil {
		t.Fatalf("ShareApiDocs: %v", err)
	}

	if _, err := c.PageExportAll(ctx); err != nil {
		t.Fatalf("PageExportAll: %v", err)
	}
}

func TestIntegrationPageReads(t *testing.T) {
	c := newIntegrationClient(t)
	ctx := context.Background()
	id := firstPageID(t, c)

	if page, err := c.PageGet(ctx, PageGetParams{ID: id}); err != nil {
		t.Fatalf("PageGet(%q): %v", id, err)
	} else if page.ID == nil || *page.ID != id {
		t.Fatalf("PageGet returned id %v, want %q", page.ID, id)
	}

	if got, err := c.PagePathOf(ctx, PagePathOfParams{ID: id}); err != nil {
		t.Fatalf("PagePathOf: %v", err)
	} else if got.Path == nil || *got.Path == "" {
		t.Fatal("PagePathOf: empty path")
	}

	if _, err := c.PageRefs(ctx, PageRefsParams{Ids: []string{id}}); err != nil {
		t.Fatalf("PageRefs: %v", err)
	}

	since := "2026-01-01T00:00:00Z"
	if _, err := c.PageChanges(ctx, PageChangesParams{Since: &since}); err != nil {
		t.Fatalf("PageChanges: %v", err)
	}
}

func TestIntegrationSearch(t *testing.T) {
	c := newIntegrationClient(t)
	ctx := context.Background()

	q, limit := "homepage", float64(3)
	res, err := c.SearchQuery(ctx, SearchQueryParams{Query: &q, Limit: &limit})
	if err != nil {
		t.Fatalf("SearchQuery: %v", err)
	}
	if res.Hits == nil {
		t.Fatal("SearchQuery: nil Hits")
	}
}

func TestIntegrationHistoryAndComments(t *testing.T) {
	c := newIntegrationClient(t)
	ctx := context.Background()
	id := firstPageID(t, c)

	if _, err := c.HistoryList(ctx, HistoryListParams{PageID: id}); err != nil {
		t.Fatalf("HistoryList: %v", err)
	}
	if _, err := c.AnnotationList(ctx, AnnotationListParams{NodeID: id}); err != nil {
		t.Fatalf("AnnotationList: %v", err)
	}
	if _, err := c.CommentList(ctx, CommentListParams{NodeID: id}); err != nil {
		t.Fatalf("CommentList: %v", err)
	}
}

func TestIntegrationPageWriteRoundTrip(t *testing.T) {
	c := newIntegrationClient(t)
	ctx := context.Background()

	path := fmt.Sprintf("cli-integration/test-%d", time.Now().UnixNano())
	body := "# integration " + time.Now().Format(time.RFC3339)

	created, err := c.PageUpsert(ctx, PageUpsertParams{Path: path, Body: &body})
	if err != nil {
		t.Fatalf("PageUpsert(%q): %v", path, err)
	}
	if created.ID == nil || *created.ID == "" {
		t.Fatalf("PageUpsert: no id in %+v", created)
	}
	id := *created.ID
	t.Cleanup(func() {
		if _, err := c.PageDelete(ctx, PageDeleteParams{ID: id}); err != nil {
			t.Errorf("cleanup PageDelete(%q): %v", id, err)
		}
	})

	got, err := c.PageGet(ctx, PageGetParams{ID: id})
	if err != nil {
		t.Fatalf("PageGet(%q) after upsert: %v", id, err)
	}
	if got.Body == nil || *got.Body != body {
		t.Fatalf("round-tripped body = %q, want %q", strp(got.Body), body)
	}
}

func TestIntegrationSessionOnlyBlocked(t *testing.T) {
	c := newIntegrationClient(t)
	ctx := context.Background()

	// Client-side guard: control-plane RPCs from the session-only set.
	_, err := c.AuthMe(ctx)
	assertAPIError(t, err, "AuthMe must be blocked for API keys")

	// Server-side enforcement: key-reachable surface is narrower than the full
	// RPC catalog, so the server rejects these too (proves live error parsing).
	_, err = c.SettingsGet(ctx)
	assertAPIError(t, err, "SettingsGet must be rejected for API keys")
}

func assertAPIError(t *testing.T, err error, msg string) {
	t.Helper()
	var aerr *APIError
	if !errors.As(err, &aerr) {
		t.Fatalf("%s: err = %v (%T), want *APIError", msg, err, err)
	}
	if aerr.StatusCode != 403 {
		t.Fatalf("%s: StatusCode = %d, want 403", msg, aerr.StatusCode)
	}
	if !strings.Contains(aerr.Message, "API keys") {
		t.Fatalf("%s: Message = %q, want mention of API keys", msg, aerr.Message)
	}
}

func strp(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
