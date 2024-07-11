package core

import (
	"context"
	"io"
	"os"
	"path"
	"strings"

	"github.com/nixmade/orchestrator/store"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	namespacePrefix = "namespace:"
)

// Namespace
// | - Entity
//     | - Target

// Engine holds the namespaces serves
type Engine struct {
	ctx    context.Context
	store  store.Store
	logger zerolog.Logger
}

// Provides an input config for new orchestrator engine
type Config struct {
	// Name for logs
	ApplicationName string
	// trace, debug, info, error, warn, fatal, panic
	LogLevel string
	// enable console logging
	ConsoleLogging bool
	// log dir to out json formatted logs
	// these logs are auto rotated
	LogDirectory string
	// Use postgres db
	StoreDatabaseURL string
	// postgres db schema
	StoreDatabaseSchema string
	// postgres db table
	StoreDatabaseTable string
	// Location to store badger db
	StoreDirectory string
	// Masterkey for encrypting badger store
	StoreMasterKey string
}

func namespaceKey(name string) string {
	return namespacePrefix + name
}

func (e *Engine) getNamespace(name string) (*Namespace, error) {
	namespace, err := e.findNamespace(name)
	if err == store.ErrKeyNotFound {
		e.logger.Info().Msgf("Creating new namespace %s", name)
		namespace, err = e.createNamespace(name)
		if err != nil {
			return nil, err
		}
		namespace.store = e.store
		namespace.logger = e.logger.With().Str("Namespace", name).Logger()
	}

	return namespace, err
}

func (e *Engine) findNamespace(name string) (*Namespace, error) {
	namespace := &Namespace{}
	if err := e.store.LoadJSON(namespaceKey(name), namespace); err != nil {
		return nil, err
	}

	namespace.store = e.store
	namespace.logger = e.logger.With().Str("Namespace", name).Logger()

	return namespace, nil
}

// Save saves engine specific components only
func (e *Engine) Save() error {
	return nil
}

// SaveNamespaceEntity saves entityName with
func (e *Engine) SaveNamespaceEntity(namespaceName, entityName string) error {
	if err := e.Save(); err != nil {
		return err
	}

	namespace, err := e.getNamespace(namespaceName)
	if err != nil {
		return err
	}

	return namespace.SaveEntity(entityName)
}

// Load loads list of namespaces for a userID
func (e *Engine) Load() error {
	e.logger.Info().Msg("Loading engine")

	return nil
}

func NewDefaultConfig() *Config {
	return &Config{
		ApplicationName:     "",
		LogLevel:            "fatal",
		ConsoleLogging:      true,
		LogDirectory:        "",
		StoreDatabaseURL:    "",
		StoreDatabaseSchema: store.PUBLIC_SCHEMA,
		StoreDatabaseTable:  store.TABLE_NAME,
		StoreDirectory:      "",
		StoreMasterKey:      "",
	}
}

// NewOrchestratorEngine creates a new Orchestration Context
func NewOrchestratorEngine(config *Config) (*Engine, error) {
	level, err := zerolog.ParseLevel(strings.ToLower(config.LogLevel))
	if err != nil {
		level = zerolog.FatalLevel
	}

	writers := []io.Writer{}

	if config.ConsoleLogging {
		writers = append(writers, zerolog.ConsoleWriter{Out: os.Stderr})
	}

	if config.LogDirectory != "" {
		writers = append(writers, &lumberjack.Logger{
			Filename:   path.Join(config.LogDirectory, "orchestrator.log"),
			MaxBackups: 3,  // files
			MaxSize:    10, // megabytes
			MaxAge:     7,  // days
		})
	}

	logger := zerolog.New(io.MultiWriter(writers...)).
		With().
		Str("Application", config.ApplicationName).
		Caller().
		Timestamp().
		Logger().
		Level(level)

	var dbStore store.Store
	if config.StoreDatabaseURL != "" {
		dbStore, err = store.NewPgxStore(config.StoreDatabaseURL, config.StoreDatabaseSchema, config.StoreDatabaseTable)
	} else {
		dbStore, err = store.NewBadgerDBStore(config.StoreDirectory, config.StoreMasterKey)
	}

	if err != nil {
		logger.Error().Err(err).Msg("failed to create store")
		return nil, err
	}

	logger.Info().Msg("Creating orchestrator engine")

	e := &Engine{
		ctx:    context.Background(),
		logger: logger,
		store:  dbStore,
	}

	if err := e.Load(); err != nil {
		return nil, err
	}

	//go e.saveStateAsync()

	return e, nil
}

// NewOrchestratorEngineWithApp creates a new Orchestration Context
func NewOrchestratorEngineWithApp(app *App) (*Engine, error) {
	app.logger.Info().Msg("Creating orchestrator engine")

	e := &Engine{
		ctx:    context.Background(),
		logger: app.logger,
		store:  app.dbStore,
	}

	if err := e.Load(); err != nil {
		return nil, err
	}

	//go e.saveStateAsync()

	return e, nil
}

// Shutdown the engine when process is shutdown
func (e *Engine) Shutdown() error {
	e.logger.Info().Msg("Shutdown orchestrator engine")
	return nil
}

// Shutdown the engine when process is shutdown
func (e *Engine) ShutdownAndClose() error {
	e.logger.Info().Msg("Shutdown orchestrator engine")
	return e.store.Close()
}

// saveStateAsync saves all namespaces and associated entitied to storage
// func (e *Engine) saveStateAsync() {
// 	for {
// 		if err := e.Save(); err != nil {
// 			e.logger.Errorf("Failed to Save Engine")
// 		}

// 		var namespaceState []*Namespace
// 		e.lock.Lock()
// 		for _, namespace := range e.namespaces {
// 			namespaceState = append(namespaceState, namespace)
// 		}
// 		e.lock.Unlock()

// 		for _, namespace := range namespaceState {
// 			if err := namespace.SaveNamespace(e.store, e.userID); err != nil {
// 				e.logger.Errorf("Failed to Save Namespace '%s", namespace.name)
// 			}
// 		}
// 		time.Sleep(10 * time.Second)
// 	}
// }

// SetTargetVersion sets the target version
func (e *Engine) SetTargetVersion(namespaceName, entityName string, targetVersion EntityTargetVersion) error {
	namespace, err := e.getNamespace(namespaceName)
	if err != nil {
		return err
	}

	entity, err := namespace.findorCreateEntity(entityName)
	if err != nil {
		return err
	}

	if targetVersion.Version == "" {
		return ErrInvalidTargetVersion
	}

	return entity.setTargetVersion(targetVersion.Version, false)
}

// ForceTargetVersion sets the target version and marks current rolling version as bad
// this allows target version to be promoted to rolling version
func (e *Engine) ForceTargetVersion(namespaceName, entityName string, targetVersion EntityTargetVersion) error {
	namespace, err := e.getNamespace(namespaceName)
	if err != nil {
		return err
	}

	entity, err := namespace.findorCreateEntity(entityName)
	if err != nil {
		return err
	}

	if targetVersion.Version == "" {
		return ErrInvalidTargetVersion
	}

	return entity.setTargetVersion(targetVersion.Version, true)
}

// SetRolloutOptions sets rollout options for the entity
//
//	caller should typically set this initially before calling orchestrate
func (e *Engine) SetRolloutOptions(namespaceName string, entityName string, options *RolloutOptions) error {
	namespace, err := e.getNamespace(namespaceName)
	if err != nil {
		return err
	}

	entity, err := namespace.findorCreateEntity(entityName)
	if err != nil {
		return err
	}

	return entity.setRolloutOptions(options)
}

// SetEntityTargetController sets entity target controller for the entity
//
//	typically used for override
func (e *Engine) SetEntityTargetController(namespaceName string, entityName string, controller EntityTargetController) error {
	namespace, err := e.getNamespace(namespaceName)
	if err != nil {
		return err
	}

	entity, err := namespace.findorCreateEntity(entityName)
	if err != nil {
		return err
	}

	return entity.setTargetController(controller)
}

// SetEntityTargetController sets entity target controller for the entity
//
//	typically used for override
func (e *Engine) SetEntityMonitoringController(namespaceName string, entityName string, controller EntityMonitoringController) error {
	namespace, err := e.getNamespace(namespaceName)
	if err != nil {
		return err
	}

	entity, err := namespace.findorCreateEntity(entityName)
	if err != nil {
		return err
	}

	return entity.setMonitoringController(controller)
}

// Orchestrate list of input targets, modifies the state to record target state
func (e *Engine) Orchestrate(namespaceName, entityName string, targets []*ClientState) ([]*ClientState, error) {
	namespace, err := e.getNamespace(namespaceName)
	if err != nil {
		return nil, err
	}
	clientTargets, err := namespace.orchestrate(entityName, targets)

	if err != nil {
		return nil, err
	}

	// We should ideally just save the calling entity only
	return clientTargets, e.SaveNamespaceEntity(namespaceName, entityName)
}

// OrchestrateAsync records the input state of targets, responds nothing
// Clients call GetClientState to know the declarative state of targets
func (e *Engine) OrchestrateAsync(namespaceName, entityName string, targets []*ClientState) error {
	namespace, err := e.getNamespace(namespaceName)
	if err != nil {
		return err
	}

	if err := namespace.orchestrateasync(entityName, targets); err != nil {
		return err
	}

	// We should ideally just save the calling entity only
	return e.SaveNamespaceEntity(namespaceName, entityName)
}

// GetClientState Gets Expected Client State for all the client state for the namespace, entity
// This is an optional API where controller service reports partial status thought 1 API,
// gets the current expected client state with another API
func (e *Engine) GetClientState(namespaceName, entityName string) ([]*ClientState, error) {
	namespace, err := e.findNamespace(namespaceName)
	if err != nil {
		return nil, err
	}
	clientTargets, err := namespace.getClientState(entityName)

	if err != nil {
		return nil, err
	}

	return clientTargets, nil
}

// GetClientGroupState Gets Expected Client State for the client group state for the namespace, entity
// This is an optional API where controller service reports partial status thought 1 API,
// gets the current expected client group state with another API
func (e *Engine) GetClientGroupState(namespaceName, entityName, groupName string) ([]*ClientState, error) {
	namespace, err := e.findNamespace(namespaceName)
	if err != nil {
		return nil, err
	}
	clientTargets, err := namespace.getClientGroupState(entityName, groupName)

	if err != nil {
		return nil, err
	}

	return clientTargets, nil
}

// Below are mostly supporting cast for front end API

// GetNamespaces returns a list of namespaces owned by the engine
func (e *Engine) GetNamespaces() ([]string, error) {
	namespaces, err := e.store.LoadKeys(namespacePrefix)

	if err != nil {
		return nil, err
	}

	return namespaces, nil
}

// GetEntites returns a list of entities owned by the namespace
func (e *Engine) GetEntites(namespaceName string) ([]string, error) {
	namespace, err := e.findNamespace(namespaceName)
	if err != nil {
		return nil, err
	}

	return namespace.getEntities()
}

// GetRolloutInfo returns current rollout information
func (e *Engine) GetRolloutInfo(namespaceName, entityName string) (*RolloutState, error) {
	namespace, err := e.findNamespace(namespaceName)
	if err != nil {
		return nil, err
	}

	return namespace.getRolloutInfo(entityName)
}
