package server

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/rs/zerolog"
)

type AppContext interface {
	Name() string
	Create(zerolog.Logger) error
	Delete() error
	Handler() http.Handler
}

// Context stores local and aggregate stores
type Context struct {
	srv    *http.Server
	logger zerolog.Logger
	app    AppContext
}

// Create App context creating router handling multiple REST API
func (ctx *Context) Create() error {
	if err := ctx.app.Create(ctx.logger); err != nil {
		return err
	}

	ctx.srv = &http.Server{
		Addr:         "127.0.0.1:8080",
		Handler:      ctx.app.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second}

	go func() {
		if err := ctx.srv.ListenAndServe(); err != nil {
			msg := fmt.Sprintf("%s", err)
			if flag.Lookup("test.v") == nil {
				ctx.logger.Fatal().Msg(msg)
			} else {
				ctx.logger.Info().Msg(msg)
			}
		}
	}()

	return nil
}

// Create creates and sets up context, stores and starts HTTP Server
func Create(app AppContext) (*Context, error) {
	appName := os.Getenv("APP_NAME")
	ctx := &Context{app: app}

	// register default and app routes
	level, err := zerolog.ParseLevel(strings.ToLower(os.Getenv("APP_LOG_LEVEL")))
	if err != nil {
		level = zerolog.FatalLevel
	}
	logger := zerolog.New(os.Stderr).With().Caller().Timestamp().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(level)

	// Use the right ID below
	ctx.logger = logger.With().Str("Application", appName).Logger()

	ctx.logger.Info().Msg("Creating Context")

	// Create context
	if err := ctx.Create(); err != nil {
		return nil, err
	}

	return ctx, nil
}

// DeleteContext app context and HTTP Server
func (ctx *Context) Delete() error {
	if ctx == nil {
		return nil
	}
	if err := ctx.app.Delete(); err != nil {
		return err
	}
	// Shutdown HTTP server
	// even if there is an error shutting down HTTP its ok to ignore
	return ctx.srv.Shutdown(context.TODO())
}

func DefaultRouter() *chi.Mux {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	//router.Use(NewStructuedLogger(logger))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)
	router.Use(render.SetContentType(render.ContentTypeJSON))

	// Seek, verify and validate JWT tokens
	// router.Use(jwtauth.Verifier(auth))

	// Handle valid / invalid tokens.
	// router.Use(jwtauth.Authenticator(auth))

	// Validating claims
	// Might require to validate different claims for dashboard and API
	//_, claims, _ := jwtauth.FromContext(router.Context())

	return router
}

func waitForCtrlC() {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	<-sigc
}

// Execute starts application and waits for ctrl+c
func Execute(app AppContext) error {
	ctx, err := Create(app)
	if err != nil {
		return err
	}
	waitForCtrlC()
	return ctx.Delete()
}
