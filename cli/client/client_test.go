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
	var meBody, recentBody, loginBody []byte
	var meAuthHeader string

	mux := http.NewServeMux()
	mux.HandleFunc("/rpc/Auth/me", func(w http.ResponseWriter, r *http.Request) {
		meBody, _ = io.ReadAll(r.Body)
		meAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"subject":"u1","handle":"alice","email":"a@b.c","has_password":true}}`))
	})
	mux.HandleFunc("/rpc/Activity/favorite", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"p1": true, "p2": false}}`))
	})
	mux.HandleFunc("/rpc/Activity/recent", func(w http.ResponseWriter, r *http.Request) {
		recentBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"ids": ["a","b"]}}`))
	})
	mux.HandleFunc("/rpc/Auth/login", func(w http.ResponseWriter, r *http.Request) {
		loginBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"bad password","code":"UNAUTHORIZED"}}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := New(srv.URL, WithToken("tok"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	t.Run("AuthMe decodes typed result and sends token", func(t *testing.T) {
		me, err := c.AuthMe(ctx)
		if err != nil {
			t.Fatalf("AuthMe: %v", err)
		}
		if me.Subject == nil || *me.Subject != "u1" {
			t.Fatalf("me.Subject = %v, want %q", me.Subject, "u1")
		}
		if me.Handle == nil || *me.Handle != "alice" {
			t.Fatalf("me.Handle = %v, want %q", me.Handle, "alice")
		}
		if me.HasPassword == nil || !*me.HasPassword {
			t.Fatalf("me.HasPassword = %v, want true", me.HasPassword)
		}
		if meAuthHeader != "Bearer tok" {
			t.Fatalf("Authorization = %q, want %q", meAuthHeader, "Bearer tok")
		}
		if string(meBody) != `{"args":{}}` {
			t.Fatalf("request body = %q, want %q", meBody, `{"args":{}}`)
		}
	})

	t.Run("ActivityFavorite decodes map result", func(t *testing.T) {
		res, err := c.ActivityFavorite(ctx, ActivityFavoriteParams{PageID: "p1"})
		if err != nil {
			t.Fatalf("ActivityFavorite: %v", err)
		}
		if len(res) != 2 || !res["p1"] || res["p2"] {
			t.Fatalf("result = %v, want {p1:true, p2:false}", res)
		}
	})

	t.Run("ActivityRecent sends args envelope and decodes IDsResult", func(t *testing.T) {
		res, err := c.ActivityRecent(ctx)
		if err != nil {
			t.Fatalf("ActivityRecent: %v", err)
		}
		if len(res.Ids) != 2 || res.Ids[0] != "a" || res.Ids[1] != "b" {
			t.Fatalf("ids = %v, want [a b]", res.Ids)
		}
		if string(recentBody) != `{"args":{}}` {
			t.Fatalf("request body = %q, want %q (void request)", recentBody, `{"args":{}}`)
		}
	})

	t.Run("AuthLogin returns APIError on 500", func(t *testing.T) {
		_, err := c.AuthLogin(ctx, AuthLoginParams{Handle: "alice", Password: "hunter2"})
		var aerr *APIError
		if !errors.As(err, &aerr) {
			t.Fatalf("err = %v (%T), want *APIError", err, err)
		}
		if aerr.StatusCode != http.StatusInternalServerError {
			t.Fatalf("StatusCode = %d, want 500", aerr.StatusCode)
		}
		if !strings.Contains(aerr.Message, "bad password") {
			t.Fatalf("Message = %q, want it to contain %q", aerr.Message, "bad password")
		}
		if aerr.Code != "UNAUTHORIZED" {
			t.Fatalf("Code = %q, want %q", aerr.Code, "UNAUTHORIZED")
		}
		if string(loginBody) != `{"args":{"handle":"alice","password":"hunter2"}}` {
			t.Fatalf("login body = %q", loginBody)
		}
	})
}

func TestSessionOnlyBlockedForAPIKey(t *testing.T) {
	c, err := New("http://example.invalid", WithAPIKey("mnk_somekey"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	for _, call := range []struct {
		name string
		fn   func() error
	}{
		{"AuthMe", func() error { _, err := c.AuthMe(ctx); return err }},
		{"AuthCreateAPIKey", func() error { _, err := c.AuthCreateAPIKey(ctx, AuthCreateAPIKeyParams{Name: "x"}); return err }},
		{"AuthListAPIKeys", func() error { _, err := c.AuthListAPIKeys(ctx); return err }},
		{"AuthLogin", func() error { _, err := c.AuthLogin(ctx, AuthLoginParams{Handle: "h", Password: "p"}); return err }},
	} {
		t.Run(call.name, func(t *testing.T) {
			err := call.fn()
			var aerr *APIError
			if !errors.As(err, &aerr) {
				t.Fatalf("err = %v (%T), want *APIError", err, err)
			}
			if aerr.StatusCode != http.StatusForbidden {
				t.Fatalf("StatusCode = %d, want 403", aerr.StatusCode)
			}
			if !strings.Contains(aerr.Message, "not available to API keys") {
				t.Fatalf("Message = %q, want mention of API keys", aerr.Message)
			}
		})
	}

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
