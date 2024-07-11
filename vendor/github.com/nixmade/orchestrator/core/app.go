package core

import (
	"net/http"
	"os"

	"github.com/nixmade/orchestrator/store"
	"github.com/rs/zerolog"
)

// Context stores local and aggregate stores
type App struct {
	dbStore store.Store
	e       *Engine
	logger  zerolog.Logger
}

func NewApp() *App {
	return &App{}
}

func (app *App) Name() string {
	return "orchestrator"
}

// Create App context creating router handling multiple REST API
func (app *App) Create(logger zerolog.Logger) error {
	var err error

	app.logger = logger
	app.dbStore, err = store.NewBadgerDBStore(os.Getenv("APP_CONFIG_DIR"), os.Getenv("MASTER_KEY"))
	if err != nil {
		logger.Error().Err(err).Msg("failed to create store")
		return err
	}

	logger.Info().Msg("Starting the engine")
	app.e, err = NewOrchestratorEngineWithApp(app)
	if err != nil {
		app.logger.Error().Err(err).Msg("failed to create orchestrator engine")
		return err
	}
	return nil
}

// Delete app context and HTTP Server
func (app *App) Delete() error {
	if err := app.e.Shutdown(); err != nil {
		return err
	}
	if err := app.dbStore.Close(); err != nil {
		return err
	}
	return nil
}

func (app *App) Handler() http.Handler {
	return NewRouter(app)
}
