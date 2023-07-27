package store

import (
	"fmt"
	"os"
	"testing"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"

	"github.com/hazelcast/hazelcast-commandline-client/internal/check"
)

func bytes(s string) []byte {
	return []byte(s)
}

func TestStore_UseBeforeOpen(t *testing.T) {
	WithTempDir(func(dir string) {
		s := NewStore(dir)
		err := s.SetEntry(bytes(""), bytes(""))
		require.Equal(t, ErrDatabaseNotOpen, err)
	})
}

func TestStore_GetSetEntry(t *testing.T) {
	WithStore(func(s *Store) {
		check.Must(insertValues(s.db, map[string][]byte{
			"key1": bytes("val"),
		}))
		valb := check.MustValue(s.GetEntry(bytes("key1")))
		require.Equal(t, bytes("val"), valb)
		check.Must(s.SetEntry(bytes("key1"), bytes("valnew")))
		valnew := check.MustValue(s.GetEntry(bytes("key1")))
		require.Equal(t, bytes("valnew"), valnew)
	})
}

func TestStore_GetKeysWithPrefix(t *testing.T) {
	WithStore(func(s *Store) {
		check.Must(insertValues(s.db, map[string][]byte{
			"prefix.key1": bytes(""),
			"prefix.key2": bytes(""),
			"noprefix":    bytes(""),
		}))
		vals := check.MustValue(s.GetKeysWithPrefix("prefix"))
		expected := [][]byte{bytes("prefix.key1"), bytes("prefix.key2")}
		require.ElementsMatch(t, expected, vals)
	})
}

func TestStore_UpdateEntry(t *testing.T) {
	WithStore(func(s *Store) {
		check.Must(s.UpdateEntry(bytes("key"), func(current []byte, found bool) []byte {
			if !found {
				return bytes("notexist")
			}
			return nil
		}))
		valb := check.MustValue(s.GetEntry(bytes("key")))
		require.Equal(t, bytes("notexist"), valb)
		check.Must(s.UpdateEntry(bytes("key"), func(current []byte, found bool) []byte {
			if found {
				return append(current, bytes(".nowexist")...)
			}
			return nil
		}))
		valnew := check.MustValue(s.GetEntry(bytes("key")))
		require.Equal(t, bytes("notexist.nowexist"), valnew)
	})
}

func TestStore_RunForeachWithPrefix(t *testing.T) {
	fromStore := make(map[string][]byte)
	WithStore(func(s *Store) {
		check.Must(insertValues(s.db, map[string][]byte{
			"prefix.key1": bytes(""),
			"prefix.key2": bytes(""),
			"noprefix":    bytes(""),
		}))
		check.Must(s.RunForeachWithPrefix("prefix", func(key, val []byte) {
			fromStore[string(key)] = val
		}))
		expected := map[string][]byte{
			"prefix.key1": nil,
			"prefix.key2": nil,
		}
		require.EqualValues(t, expected, fromStore)
	})
}

func TestStore_DeleteEntriesWithPrefix(t *testing.T) {
	WithStore(func(s *Store) {
		check.Must(insertValues(s.db, map[string][]byte{
			"prefix.key1": bytes(""),
			"prefix.key2": bytes(""),
			"noprefix":    bytes(""),
		}))
		check.Must(s.DeleteEntriesWithPrefix("prefix"))
		entries := check.MustValue(getAllEntries(s.db))
		expected := map[string][]byte{
			"noprefix": nil,
		}
		require.EqualValues(t, expected, entries)
	})
}

func getAllEntries(db *badger.DB) (map[string][]byte, error) {
	m := make(map[string][]byte)
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			b, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			m[string(k)] = b
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

func insertValues(db *badger.DB, vals map[string][]byte) error {
	err := db.Update(func(txn *badger.Txn) error {
		for k, v := range vals {
			err := txn.SetEntry(badger.NewEntry(bytes(k), v))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func WithTempDir(fn func(string)) {
	dir, err := os.MkdirTemp("", "clc-store-*")
	if err != nil {
		panic(fmt.Errorf("creating temp dir: %w", err))
	}
	defer func() {
		// errors are ignored
		os.RemoveAll(dir)
	}()
	fn(dir)
}

func WithStore(fn func(s *Store)) {
	WithTempDir(func(dir string) {
		s := NewStore(dir)
		err := s.Open()
		if err != nil {
			panic(fmt.Errorf("opening store: %w", err))
		}
		defer func() {
			// errors are ignored
			s.Close()
		}()
		fn(s)
	})
}
