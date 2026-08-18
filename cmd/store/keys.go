package store

import (
	"encoding/json"
	"errors"
	"fmt"
)

// KeyType represents the scope and capability of an API key.
type KeyType string

const (
	KeyTypeRPC   KeyType = "rpc"   // REST/RPC API access only
	KeyTypeMCP   KeyType = "mcp"   // MCP tool server access only
	KeyTypeMulti KeyType = "multi" // Valid for both RPC and MCP access
)

// IsValid reports whether kt is a recognized KeyType.
func (kt KeyType) IsValid() bool {
	switch kt {
	case KeyTypeRPC, KeyTypeMCP, KeyTypeMulti:
		return true
	default:
		return false
	}
}

// AllowsRPC reports whether the key type can be used for RPC calls.
func (kt KeyType) AllowsRPC() bool {
	return kt == KeyTypeRPC || kt == KeyTypeMulti || kt == ""
}

// AllowsMCP reports whether the key type can be used for MCP server connections.
func (kt KeyType) AllowsMCP() bool {
	return kt == KeyTypeMCP || kt == KeyTypeMulti
}

// KeysRecord stores metadata and credentials for an API or MCP key.
type KeysRecord struct {
	Name      string  `json:"name"`
	Workspace string  `json:"workspace,omitempty"`
	Token     string  `json:"token"`
	Type      KeyType `json:"type"`
}

// PutKey persists a KeysRecord in the keys bucket, keyed by Name.
func (s *Store) PutKey(k KeysRecord) error {
	if k.Name == "" {
		return errors.New("key name is required")
	}
	if k.Type == "" {
		k.Type = KeyTypeRPC
	}
	if !k.Type.IsValid() {
		return fmt.Errorf("invalid key type %q (must be %q, %q, or %q)", k.Type, KeyTypeRPC, KeyTypeMCP, KeyTypeMulti)
	}
	data, err := json.Marshal(k)
	if err != nil {
		return fmt.Errorf("marshal key record: %w", err)
	}
	return s.Put(KeysBucket, k.Name, data)
}

// DeleteKey removes a key record by name from the keys bucket and clears the
// active key setting if the deleted key was active.
func (s *Store) DeleteKey(name string) error {
	if name == "" {
		return errors.New("key name is required")
	}
	active, _ := s.GetActiveKey()
	if active == name {
		_ = s.ClearActiveKey()
	}
	return s.Delete(KeysBucket, name)
}

// GetKey retrieves a key record by name from the keys bucket.
func (s *Store) GetKey(name string) (*KeysRecord, error) {
	data := s.Get(KeysBucket, name)
	if len(data) == 0 {
		return nil, nil
	}
	var k KeysRecord
	if err := json.Unmarshal(data, &k); err != nil {
		return nil, fmt.Errorf("unmarshal key record: %w", err)
	}
	return &k, nil
}

// Keys returns all stored key records from the keys bucket.
func (s *Store) Keys() ([]KeysRecord, error) {
	var records []KeysRecord
	err := s.ForEach(KeysBucket, func(_, value []byte) error {
		var k KeysRecord
		if err := json.Unmarshal(value, &k); err != nil {
			return fmt.Errorf("unmarshal key record: %w", err)
		}
		records = append(records, k)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

// ResolveKey resolves a key record from the store for RPC usage.
func (s *Store) ResolveKey(explicitKeyName string) (*KeysRecord, error) {
	return s.ResolveKeyFor(explicitKeyName, KeyTypeRPC)
}

// ResolveKeyFor resolves a key record from the store with a specific purpose (RPC or MCP).
func (s *Store) ResolveKeyFor(explicitKeyName string, purpose KeyType) (*KeysRecord, error) {
	if explicitKeyName != "" {
		rec, err := s.GetKey(explicitKeyName)
		if err != nil {
			return nil, err
		}
		if rec == nil {
			return nil, fmt.Errorf("key %q not found in store; run 'mininote key list' to view available keys", explicitKeyName)
		}
		if purpose == KeyTypeMCP && !rec.Type.AllowsMCP() {
			return nil, fmt.Errorf("key %q is configured with type %q, but MCP requires type %q or %q", rec.Name, rec.Type, KeyTypeMCP, KeyTypeMulti)
		}
		if purpose == KeyTypeRPC && !rec.Type.AllowsRPC() {
			return nil, fmt.Errorf("key %q is configured with type %q, but RPC calls require type %q or %q", rec.Name, rec.Type, KeyTypeRPC, KeyTypeMulti)
		}
		return rec, nil
	}

	activeName, _ := s.GetActiveKey()
	if activeName != "" {
		rec, err := s.GetKey(activeName)
		if err != nil {
			return nil, err
		}
		if rec != nil {
			if purpose == KeyTypeMCP && rec.Type.AllowsMCP() {
				return rec, nil
			}
			if purpose == KeyTypeRPC && rec.Type.AllowsRPC() {
				return rec, nil
			}
		}
	}

	allKeys, err := s.Keys()
	if err != nil {
		return nil, err
	}
	if len(allKeys) == 0 {
		if purpose == KeyTypeMCP {
			return nil, errors.New("no MCP key found; please add an MCP key:\n  mininote key add <token> --name <name> --type mcp")
		}
		return nil, errors.New("no API key found; please add a key to use the CLI:\n  mininote key add <token> --name <name>")
	}

	var candidates []KeysRecord
	for _, k := range allKeys {
		if purpose == KeyTypeMCP && k.Type.AllowsMCP() {
			candidates = append(candidates, k)
		} else if purpose == KeyTypeRPC && k.Type.AllowsRPC() {
			candidates = append(candidates, k)
		}
	}

	if len(candidates) == 0 {
		if purpose == KeyTypeMCP {
			return nil, fmt.Errorf("no keys supporting MCP found (%d keys in vault are RPC-only); add an MCP key with '--type mcp' or '--type multi'", len(allKeys))
		}
		return nil, fmt.Errorf("no keys supporting RPC found (%d keys in vault are MCP-only); add an RPC key with '--type rpc' or '--type multi'", len(allKeys))
	}
	if len(candidates) == 1 {
		return &candidates[0], nil
	}
	return nil, fmt.Errorf("multiple %s-compatible keys found but no active key set; use 'mininote key use <name>' or pass '--key <name>'", purpose)
}
