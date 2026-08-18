package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := GetStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})
	return st
}

func TestStore_KeysCRUD(t *testing.T) {
	st := newTestStore(t)

	key1 := KeysRecord{
		Name:      "personal",
		Workspace: "PLUG",
		Token:     "mnk_12345",
		Type:      KeyTypeRPC,
	}

	if err := st.PutKey(key1); err != nil {
		t.Fatalf("PutKey failed: %v", err)
	}

	got, err := st.GetKey("personal")
	if err != nil {
		t.Fatalf("GetKey failed: %v", err)
	}
	if got == nil || got.Token != key1.Token || got.Type != KeyTypeRPC {
		t.Fatalf("unexpected key retrieved: %+v", got)
	}

	keys, err := st.Keys()
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}
	if len(keys) != 1 || keys[0].Name != "personal" {
		t.Fatalf("unexpected keys list: %+v", keys)
	}

	if err := st.DeleteKey("personal"); err != nil {
		t.Fatalf("DeleteKey failed: %v", err)
	}

	gotDeleted, err := st.GetKey("personal")
	if err != nil {
		t.Fatalf("GetKey after delete failed: %v", err)
	}
	if gotDeleted != nil {
		t.Fatalf("expected nil after delete, got %+v", gotDeleted)
	}
}

func TestStore_KeyTypesAndCapabilities(t *testing.T) {
	st := newTestStore(t)

	if err := st.PutKey(KeysRecord{Name: "invalid", Token: "mnk_inv", Type: "bogus"}); err == nil {
		t.Fatal("expected error for invalid KeyType, got nil")
	}

	rpcKey := KeysRecord{Name: "rpc_only", Token: "mnk_rpc", Type: KeyTypeRPC}
	mcpKey := KeysRecord{Name: "mcp_only", Token: "mnk_mcp", Type: KeyTypeMCP}
	multiKey := KeysRecord{Name: "multi", Token: "mnk_multi", Type: KeyTypeMulti}

	_ = st.PutKey(rpcKey)
	_ = st.PutKey(mcpKey)
	_ = st.PutKey(multiKey)

	// Explicit lookup RPC
	if _, err := st.ResolveKeyFor("rpc_only", KeyTypeRPC); err != nil {
		t.Fatalf("expected rpc_only to satisfy RPC, got %v", err)
	}
	if _, err := st.ResolveKeyFor("rpc_only", KeyTypeMCP); err == nil {
		t.Fatal("expected rpc_only to fail MCP check, got nil")
	}

	// Explicit lookup MCP
	if _, err := st.ResolveKeyFor("mcp_only", KeyTypeMCP); err != nil {
		t.Fatalf("expected mcp_only to satisfy MCP, got %v", err)
	}
	if _, err := st.ResolveKeyFor("mcp_only", KeyTypeRPC); err == nil {
		t.Fatal("expected mcp_only to fail RPC check, got nil")
	}

	// Explicit lookup Multi (satisfies both)
	if _, err := st.ResolveKeyFor("multi", KeyTypeRPC); err != nil {
		t.Fatalf("expected multi to satisfy RPC, got %v", err)
	}
	if _, err := st.ResolveKeyFor("multi", KeyTypeMCP); err != nil {
		t.Fatalf("expected multi to satisfy MCP, got %v", err)
	}
}

func TestStore_ResolveKey(t *testing.T) {
	t.Run("empty store", func(t *testing.T) {
		st := newTestStore(t)
		_, err := st.ResolveKey("")
		if err == nil {
			t.Fatal("expected error on empty store, got nil")
		}
	})

	t.Run("single key fallback", func(t *testing.T) {
		st := newTestStore(t)
		_ = st.PutKey(KeysRecord{Name: "solo", Token: "mnk_solo", Type: KeyTypeRPC})

		rec, err := st.ResolveKey("")
		if err != nil {
			t.Fatalf("expected single key fallback, got err: %v", err)
		}
		if rec.Name != "solo" {
			t.Fatalf("expected solo key, got %s", rec.Name)
		}
	})

	t.Run("multiple keys with active key", func(t *testing.T) {
		st := newTestStore(t)
		_ = st.PutKey(KeysRecord{Name: "k1", Token: "mnk_1", Type: KeyTypeRPC})
		_ = st.PutKey(KeysRecord{Name: "k2", Token: "mnk_2", Type: KeyTypeRPC})

		// without active key
		_, err := st.ResolveKey("")
		if err == nil {
			t.Fatal("expected error for multiple keys without active key")
		}

		// set active key
		_ = st.SetActiveKey("k2")
		rec, err := st.ResolveKey("")
		if err != nil {
			t.Fatalf("failed to resolve active key: %v", err)
		}
		if rec.Name != "k2" {
			t.Fatalf("expected k2, got %s", rec.Name)
		}

		// explicit override
		recExplicit, err := st.ResolveKey("k1")
		if err != nil {
			t.Fatalf("failed to resolve explicit key: %v", err)
		}
		if recExplicit.Name != "k1" {
			t.Fatalf("expected k1, got %s", recExplicit.Name)
		}
	})

	t.Run("delete active key clears active", func(t *testing.T) {
		st := newTestStore(t)
		_ = st.PutKey(KeysRecord{Name: "k1", Token: "mnk_1", Type: KeyTypeRPC})
		_ = st.SetActiveKey("k1")

		_ = st.DeleteKey("k1")
		active, _ := st.GetActiveKey()
		if active != "" {
			t.Fatalf("expected active key to be cleared, got %q", active)
		}
	})
}
