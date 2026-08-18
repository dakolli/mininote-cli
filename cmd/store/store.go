package store

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// StoreFile is the default filename for the bbolt database.
const StoreFile = "mininote.db"

// Bucket names.
const (
	KeysBucket     = "keys"
	SettingsBucket = "settings"
)

// Setting keys.
const (
	ActiveKeySetting = "active_key"
)

// Store wraps a bbolt DB. A nil db is the no-op store (path was empty).
type Store struct {
	db *bolt.DB
}

// DefaultPath returns $XDG_CONFIG_HOME/mininote/mininote.db, falling back
// to ~/.config/mininote/mininote.db.
func DefaultPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "mininote", StoreFile)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return StoreFile
	}
	return filepath.Join(home, ".config", "mininote", StoreFile)
}

// GetStore opens or creates a bbolt database at path with 0600 permissions.
// If path is empty, it uses DefaultPath().
func GetStore(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create store dir %s: %w", filepath.Dir(path), err)
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open store %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

// Enabled reports whether the store is backed by an active database.
func (s *Store) Enabled() bool { return s != nil && s.db != nil }

// Close closes the underlying bbolt database.
func (s *Store) Close() error {
	if !s.Enabled() {
		return nil
	}
	return s.db.Close()
}

// Get returns a copy of the value at bucket/key, or nil if absent / no-op.
func (s *Store) Get(bucket, key string) []byte {
	if !s.Enabled() {
		return nil
	}
	var out []byte
	_ = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		if v := b.Get([]byte(key)); v != nil {
			out = append([]byte(nil), v...)
		}
		return nil
	})
	return out
}

// Put writes value at bucket/key. No-op (nil) when the store is disabled.
func (s *Store) Put(bucket, key string, value []byte) error {
	if !s.Enabled() {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), value)
	})
}

// Delete removes bucket/key. No-op when disabled.
func (s *Store) Delete(bucket, key string) error {
	if !s.Enabled() {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(key))
	})
}

// ForEach calls fn for every key/value in a bucket (value is a copy). No-op when
// disabled; a non-nil return from fn stops the walk and is returned.
func (s *Store) ForEach(bucket string, fn func(key, value []byte) error) error {
	if !s.Enabled() {
		return nil
	}
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			return fn(append([]byte(nil), k...), append([]byte(nil), v...))
		})
	})
}

// SetActiveKey persists the active key name in the settings bucket.
func (s *Store) SetActiveKey(name string) error {
	return s.Put(SettingsBucket, ActiveKeySetting, []byte(name))
}

// GetActiveKey retrieves the active key name from the settings bucket.
func (s *Store) GetActiveKey() (string, error) {
	val := s.Get(SettingsBucket, ActiveKeySetting)
	return string(val), nil
}

// ClearActiveKey removes the active key setting.
func (s *Store) ClearActiveKey() error {
	return s.Delete(SettingsBucket, ActiveKeySetting)
}
