package store

import (
	"encoding/json"
	"errors"
	"fmt"
)

// KeysRecord stores metadata and credentials for an API or MCP key.
type KeysRecord struct {
	Name      string `json:"name"`
	Workspace string `json:"workspace,omitempty"`
	Token     string `json:"token"`
	Type      string `json:"type"`
}

// PutKey persists a KeysRecord in the keys bucket, keyed by Name.
func (s *Store) PutKey(k KeysRecord) error {
	if k.Name == "" {
		return errors.New("key name is required")
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

// ResolveKey resolves a key record from the store. If explicitKeyName is provided,
// it is looked up directly. Otherwise, it resolves the active key, falls back to
// the single available key if only one exists, or returns a clear error.
func (s *Store) ResolveKey(explicitKeyName string) (*KeysRecord, error) {
	if explicitKeyName != "" {
		rec, err := s.GetKey(explicitKeyName)
		if err != nil {
			return nil, err
		}
		if rec == nil {
			return nil, fmt.Errorf("key %q not found in store; run 'mininote config list-tokens' to view available keys", explicitKeyName)
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
			return rec, nil
		}
	}

	keys, err := s.Keys()
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, errors.New("no API key found; please add a key to use the CLI:\n  mininote config add-token <token> --name <name>")
	}
	if len(keys) == 1 {
		return &keys[0], nil
	}
	return nil, errors.New("multiple keys found but no active key set; use 'mininote config use <name>' or pass '--key <name>'")
}
