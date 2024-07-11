package store

import (
	"encoding/json"
	"sort"

	"github.com/dgraph-io/badger/v4"
	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

// BadgerDBStore store for db
type BadgerDBStore struct {
	db *badger.DB
}

// NewBadgerDBStore creates a new badger store
func NewBadgerDBStore(dir, masterKey string) (Store, error) {
	// Use .WithValueDir(valuedir) for better speed, but shoudnt matter in our case
	opts := badger.DefaultOptions(dir).
		WithIndexCacheSize(100 << 20).
		WithLoggingLevel(badger.ERROR).
		WithValueLogFileSize(2 << 28)
	if dir == "" {
		opts = opts.WithInMemory(true)
	}
	if masterKey != "" {
		opts = opts.WithEncryptionKey([]byte(masterKey))
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &BadgerDBStore{db: db}, nil
}

// Close db, by closing all the open handles
func (s *BadgerDBStore) Close() error {
	return s.db.Close()
}

// Save db with key value pair
func (s *BadgerDBStore) save(key, value string) error {
	// Update DB
	return s.db.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(key), []byte(value))
		return err
	})
}

// Save db with key json value pair
func (s *BadgerDBStore) SaveJSON(key string, jsonValue interface{}) error {
	// Marshal json value
	value, err := json.Marshal(jsonValue)
	if err != nil {
		return err
	}
	// Update DB
	return s.save(key, string(value))
}

// Delete deletes key from db
func (s *BadgerDBStore) Delete(key string) error {
	// Update DB
	return s.db.Update(func(txn *badger.Txn) error {
		err := txn.Delete([]byte(key))
		return err
	})
}

// Delete deletes prefix from db
func (s *BadgerDBStore) DeletePrefix(prefix string) error {
	keys, err := s.LoadKeys(prefix)
	if err != nil {
		return err
	}
	// Update DB
	return s.db.Update(func(txn *badger.Txn) error {
		for _, key := range keys {
			err := txn.Delete([]byte(key))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Load for key, return value string
func (s *BadgerDBStore) load(key string) (string, error) {
	var value string
	// Get the JSON value for this key
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		if err := item.Value(func(byteVal []byte) error {
			value = string(byteVal)
			return nil
		}); err != nil {
			return err
		}

		return nil
	})

	if err == badger.ErrKeyNotFound {
		return "", ErrKeyNotFound
	}

	return value, err
}

// LoadJSON loads key, value and unmarshals json value
func (s *BadgerDBStore) LoadJSON(key string, value interface{}) error {
	jsonValue, err := s.load(key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(jsonValue), value)
}

// LoadAll loads all keys with a prefix from db
func (s *BadgerDBStore) LoadKeys(prefix string) ([]string, error) {
	var keys []string
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		prefix := []byte(prefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			keys = append(keys, string(it.Item().Key()))
		}
		return nil
	})

	return keys, err
}

func (s *BadgerDBStore) LoadValues(prefix string, iter ValueIterator) error {
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		prefix := []byte(prefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			err := item.Value(func(value []byte) error {
				return iter(string(key), string(value))
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func (s *BadgerDBStore) Count(prefix string) (uint64, error) {
	keys, err := s.LoadKeys(prefix)
	if err != nil {
		return 0, err
	}

	return uint64(len(keys)), nil
}

func (s *BadgerDBStore) QueryJsonPath(prefix, jsonPath string, iter ValueIterator) error {
	valueIter := func(key any, value interface{}) error {
		obj, err := oj.ParseString(value.(string))
		if err != nil {
			return err
		}
		path, err := jp.ParseString(jsonPath)
		if err != nil {
			return err
		}

		for _, res := range path.Get(obj) {
			return iter(key, res)
		}

		return nil
	}
	return s.LoadValues(prefix, valueIter)
}

func (s *BadgerDBStore) CountJsonPath(prefix, jsonPath string, iter ValueIterator) error {
	valCount := make(map[any]int64)
	valueIter := func(key any, value any) error {
		obj, err := oj.ParseString(value.(string))
		if err != nil {
			return err
		}
		path, err := jp.ParseString(jsonPath)
		if err != nil {
			return err
		}

		for _, val := range path.Get(obj) {
			if count, ok := valCount[val]; ok {
				valCount[val] = count + int64(1)
			} else {
				valCount[val] = int64(1)
			}
		}

		return nil
	}
	err := s.LoadValues(prefix, valueIter)
	if err != nil {
		return err
	}

	for key, value := range valCount {
		if err := iter(key, value); err != nil {
			return err
		}
	}

	return nil
}

func (s *BadgerDBStore) SortedAscN(prefix, jsonPath string, limit int64, iter ValueIterator) error {
	return s.sortedN(prefix, jsonPath, "ASC", limit, iter)
}

func (s *BadgerDBStore) SortedDescN(prefix, jsonPath string, limit int64, iter ValueIterator) error {
	return s.sortedN(prefix, jsonPath, "DESC", limit, iter)
}

func (s *BadgerDBStore) sortedN(prefix, jsonPath string, order string, limit int64, iter ValueIterator) error {
	// Major problem here is that the ordering is done by converting the value to strings
	// This is terrible for integers
	var sorted [][]string
	keyVal := make(map[string]string)
	valueIter := func(key any, value interface{}) error {
		obj, err := oj.ParseString(value.(string))
		if err != nil {
			return err
		}
		path, err := jp.ParseString(jsonPath)
		if err != nil {
			return err
		}

		for _, res := range path.Get(obj) {
			sorted = append(sorted, []string{oj.JSON(res), key.(string)})
		}

		keyVal[key.(string)] = value.(string)

		return nil
	}
	if err := s.LoadValues(prefix, valueIter); err != nil {
		return err
	}

	sort.Slice(sorted, func(i, j int) bool {
		if order == "DESC" {
			return sorted[i][0] > sorted[j][0]
		}
		return sorted[i][0] < sorted[j][0]
	})

	count := int64(limit)
	if count <= 0 {
		count = int64(len(sorted))
	}
	for _, pathKey := range sorted {
		if count <= 0 {
			break
		}
		key := pathKey[1]
		if err := iter(key, keyVal[key]); err != nil {
			return err
		}
		count -= int64(1)
	}
	return nil
}
