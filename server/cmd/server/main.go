// Command server runs the Gde je Macker API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Drznedaj/gde-je-macker/server/internal/config"
	"github.com/Drznedaj/gde-je-macker/server/internal/db"
	"github.com/Drznedaj/gde-je-macker/server/internal/httpapi"
	"github.com/Drznedaj/gde-je-macker/server/internal/posts"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	api := posts.New(pool, cfg.PhotoDir, log)
	go api.RunExpiry(ctx, time.Minute)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(pool, log, api.Mount),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
	log.Info("listening", "port", cfg.Port)
	return serve(ctx, srv, nil, cfg.ShutdownTimeout, log)
}

// serve runs srv (on ln if given) until it fails or ctx is done, then shuts
// down gracefully within shutdownTimeout.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration, log *slog.Logger) error {
	errc := make(chan error, 1)
	go func() {
		if ln != nil {
			errc <- srv.Serve(ln)
			return
		}
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
