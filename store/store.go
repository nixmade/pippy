package store

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/nixmade/orchestrator/store"
)

var (
	HomeDir        string = ""
	defaultStore   store.Store
	ErrKeyNotFound = store.ErrKeyNotFound
	TABLE_NAME     = "pippy"
	PUBLIC_SCHEMA  = store.PUBLIC_SCHEMA
)

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "context value " + k.name
}

var (
	DatabaseSchemaCtx = &contextKey{"DatabaseSchema"}
	DatabaseTableCtx  = &contextKey{"DatabaseTable"}
)

func GetHomeDir() (string, error) {
	if HomeDir != "" {
		return HomeDir, nil
	}
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	HomeDir = userHomeDir
	return HomeDir, nil
}

func Get(ctx context.Context) (store.Store, error) {
	if defaultStore != nil {
		return defaultStore, nil
	}

	if os.Getenv("DATABASE_URL") != "" {
		schemaName := store.PUBLIC_SCHEMA
		tableName := TABLE_NAME
		schema := ctx.Value(DatabaseSchemaCtx)
		if schema != nil {
			schemaName = schema.(string)
		}
		table := ctx.Value(DatabaseTableCtx)
		if table != nil {
			tableName = table.(string)
		}
		return store.NewPgxStore(os.Getenv("DATABASE_URL"), schemaName, tableName)
	}

	userHomeDir, err := GetHomeDir()
	if err != nil {
		return nil, err
	}

	dbDir := path.Join(userHomeDir, ".pippy", "db", "pippy")
	if err := os.MkdirAll(dbDir, os.ModePerm); err != nil {
		return nil, err
	}
	dbStore, err := store.NewBadgerDBStore(dbDir, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create store %v", err)
	}

	return dbStore, nil
}

func Close(dbStore store.Store) error {
	if defaultStore != nil {
		return nil
	}
	return dbStore.Close()
}
