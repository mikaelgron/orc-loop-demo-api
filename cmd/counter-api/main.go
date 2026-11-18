// Command counter-api serves the counter over HTTP. The admin credential
// comes from the environment — COUNTER_API_ADMIN_TOKEN — and never from
// the command line or the code; the store is a file; the service listens
// until it is stopped and then closes the store cleanly, so a stop is a
// restart and the steps survive both.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	counterapi "github.com/mikaelgron/orc-loop-demo-api"
)

func main() {
	addr := flag.String("addr", ":8080", "where the service listens")
	storePath := flag.String("store", "counter-api.boltdb", "the store's file")
	flag.Parse()

	// The admin credential is a secret: it comes from the environment,
	// and its absence is a configuration error the service refuses to
	// start under.
	// orc:ADR-0002#decision.c5
	token := os.Getenv("COUNTER_API_ADMIN_TOKEN")
	if token == "" {
		slog.Error("COUNTER_API_ADMIN_TOKEN is required")
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	store, err := counterapi.Open(*storePath)
	if err != nil {
		slog.Error("opening the store", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           counterapi.New(logger, store, token, time.Now),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutting down", "err", err)
		}
	}()

	slog.Info("serving the counter", "addr", *addr, "store", *storePath)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		// The store closes even on a serving error: whatever happened,
		// the file on disk is consistent for the next start.
		// orc:ADR-0002#decision.c9
		_ = store.Close()
		slog.Error("serving", "err", err)
		os.Exit(1)
	}
	if err := store.Close(); err != nil {
		slog.Error("closing the store", "err", err)
		os.Exit(1)
	}
}
