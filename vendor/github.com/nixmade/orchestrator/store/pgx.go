package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
)

const (
	PUBLIC_SCHEMA = "public"
	TABLE_NAME    = "orchestrator"
)

// PgxStore stores key value pairs in postgres db
type PgxStore struct {
	pgconn *pgx.Conn
	schema string
	table  string
}

// NewPgxStore creates a new postgres store
func NewPgxStore(databaseURL, schema, table string) (Store, error) {
	pgconn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}
	return &PgxStore{
		pgconn: pgconn,
		schema: schema,
		table:  table,
	}, nil
}

func NewDefaultPgxStore(databaseURL string) (Store, error) {
	pgconn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}
	return &PgxStore{
		pgconn: pgconn,
		schema: PUBLIC_SCHEMA,
		table:  TABLE_NAME,
	}, nil
}

// Save saves to store
func (s *PgxStore) save(key, value string) error {
	query := fmt.Sprintf("INSERT INTO %s.%s (KEY, VALUE) VALUES ('%s', '%s') ON CONFLICT(KEY) DO UPDATE SET VALUE = '%s';", s.schema, s.table, key, value, value)
	_, err := s.pgconn.Exec(context.Background(), query)
	return err
}

// Save db with key json value pair
func (s *PgxStore) SaveJSON(key string, jsonValue interface{}) error {
	// Marshal json value
	value, err := json.Marshal(jsonValue)
	if err != nil {
		return err
	}
	// Update DB
	return s.save(key, string(value))
}

// Delete deletes key from store
func (s *PgxStore) Delete(key string) error {
	query := fmt.Sprintf("DELETE FROM %s.%s WHERE KEY = '%s';", s.schema, s.table, key)
	_, err := s.pgconn.Exec(context.Background(), query)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil
		}
		return err
	}
	return nil
}

// Delete deletes prefix pattern from store
func (s *PgxStore) DeletePrefix(prefix string) error {
	query := fmt.Sprintf("DELETE FROM %s.%s WHERE KEY LIKE '%s%%';", s.schema, s.table, prefix)
	_, err := s.pgconn.Exec(context.Background(), query)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil
		}
		return err
	}
	return nil
}

// Load loads from an store
func (s *PgxStore) load(key string) (string, error) {
	query := fmt.Sprintf("SELECT VALUE FROM %s.%s WHERE KEY = '%s';", s.schema, s.table, key)
	value := ""
	err := s.pgconn.QueryRow(context.Background(), query).Scan(&value)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", ErrKeyNotFound
		}
		return "", err
	}
	return value, nil
}

// LoadJSON loads key, value and unmarshals json value
func (s *PgxStore) LoadJSON(key string, value interface{}) error {
	jsonValue, err := s.load(key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(jsonValue), value)
}

// LoadAll loads all keys
func (s *PgxStore) LoadKeys(prefix string) ([]string, error) {
	query := fmt.Sprintf("SELECT KEY FROM %s.%s WHERE KEY LIKE '%s%%';", s.schema, s.table, prefix)
	rows, err := s.pgconn.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	var keys []string
	for rows.Next() {
		var key string
		err := rows.Scan(&key)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (s *PgxStore) LoadValues(prefix string, iter ValueIterator) error {
	query := fmt.Sprintf("SELECT KEY, VALUE FROM %s.%s WHERE KEY LIKE '%s%%';", s.schema, s.table, prefix)
	rows, err := s.pgconn.Query(context.Background(), query)
	if err != nil {
		return err
	}
	for rows.Next() {
		var key string
		var value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return err
		}
		if err = iter(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *PgxStore) QueryJsonPath(prefix, jsonPath string, iter ValueIterator) error {
	// example: '$.state'
	// to get only successful state you can do '$.state ? (@ == "Success")'
	query := fmt.Sprintf("SELECT KEY, jsonb_path_query(\"value\",'%s') AS JSONPATH FROM %s.%s WHERE KEY LIKE '%s%%';", jsonPath, s.schema, s.table, prefix)
	rows, err := s.pgconn.Query(context.Background(), query)
	if err != nil {
		return err
	}
	for rows.Next() {
		var key string
		var value any
		err := rows.Scan(&key, &value)
		if err != nil {
			return err
		}
		if err = iter(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *PgxStore) SortedAscN(prefix, jsonPath string, limit int64, iter ValueIterator) error {
	return s.sortedN(prefix, jsonPath, "ASC", limit, iter)
}

func (s *PgxStore) SortedDescN(prefix, jsonPath string, limit int64, iter ValueIterator) error {
	return s.sortedN(prefix, jsonPath, "DESC", limit, iter)
}

func (s *PgxStore) sortedN(prefix, jsonPath string, order string, limit int64, iter ValueIterator) error {
	// example: '$.state'
	// to get only successful state you can do '$.state ? (@ == "Success")'
	query := fmt.Sprintf("SELECT KEY, VALUE FROM %s.%s WHERE KEY LIKE '%s%%' ORDER BY jsonb_path_query(\"value\",'%s') %s", s.schema, s.table, prefix, jsonPath, order)
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	query += ";"
	rows, err := s.pgconn.Query(context.Background(), query)
	if err != nil {
		return err
	}
	for rows.Next() {
		var key string
		var value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return err
		}
		if err = iter(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *PgxStore) CountJsonPath(prefix, jsonPath string, iter ValueIterator) error {
	// example: '$.state'
	// to get only successful state you can do '$.state ? (@ == "Success")'
	query := fmt.Sprintf("SELECT jsonb_path_query(\"value\",'%s') AS JSONPATH, COUNT(KEY) FROM %s.%s WHERE KEY LIKE '%s%%' GROUP BY JSONPATH;", jsonPath, s.schema, s.table, prefix)
	rows, err := s.pgconn.Query(context.Background(), query)
	if err != nil {
		return err
	}
	for rows.Next() {
		var key any
		var value int64
		err := rows.Scan(&key, &value)
		if err != nil {
			return err
		}
		if err = iter(key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *PgxStore) Count(prefix string) (uint64, error) {
	var count uint64
	query := fmt.Sprintf("SELECT COUNT(key) FROM %s.%s WHERE KEY LIKE '%s%%';", s.schema, s.table, prefix)
	err := s.pgconn.QueryRow(context.Background(), query).Scan(&count)
	return count, err
}

// Close
func (s *PgxStore) Close() error {
	s.pgconn.Close(context.Background())
	return nil
}

type PgxStoreTest struct {
	PgxStore
}

// NewPgxStore creates a new postgres store
func NewPgxStoreWithTable(databaseURL, table string) (Store, error) {
	if !testing.Testing() {
		// check if testing mode
		return nil, fmt.Errorf("this is only allowed in testing mode")
	}
	pgconn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		return nil, err
	}
	_, err = pgconn.Exec(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s.%s;", PUBLIC_SCHEMA, table))
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("CREATE TABLE %s (ID  SERIAL PRIMARY KEY, DATE timestamp not null default CURRENT_TIMESTAMP, KEY VARCHAR UNIQUE, VALUE JSONB);", table)
	_, err = pgconn.Exec(context.Background(), query)
	if err != nil {
		return nil, err
	}
	return &PgxStoreTest{
		PgxStore: PgxStore{pgconn: pgconn, schema: PUBLIC_SCHEMA, table: table},
	}, nil
}

// Close
func (s *PgxStoreTest) Close() error {
	_, err := s.pgconn.Exec(context.Background(), fmt.Sprintf("DROP TABLE '%s.%s';", s.schema, s.table))
	if err != nil {
		return err
	}
	s.pgconn.Close(context.Background())
	return nil
}
