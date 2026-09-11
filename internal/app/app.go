package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"sk5proxy/internal/api"
	listenermanager "sk5proxy/internal/listener"
	"sk5proxy/internal/upstream"
	"sk5proxy/internal/web"
)

type serviceResult struct {
	name string
	err  error
}

func Run(ctx context.Context, logger *slog.Logger) error {
	runtimeConfig, err := runtimeConfigFromEnv(os.LookupEnv)
	if err != nil {
		return err
	}
	defaults := defaultListeners(runtimeConfig.SOCKSAddr, runtimeConfig.HTTPAddr, "")
	store, storedConfig, err := loadConfig(ctx, runtimeConfig.ConfigPath, defaults)
	if err != nil {
		return fmt.Errorf("initialize config: %w", err)
	}
	selector, err := upstream.NewSelector(storedConfig.Upstreams, storedConfig.ActiveID)
	if err != nil {
		return fmt.Errorf("initialize upstream selector: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	listenerManager := listenermanager.NewManager(runCtx, selector)
	defer listenerManager.Close()
	prepared, err := listenerManager.Prepare(storedConfig)
	if err != nil {
		return fmt.Errorf("prepare downstream listeners: %w", err)
	}
	listenerManager.Commit(prepared)

	mux := http.NewServeMux()
	apiHandler := api.NewHandlerWithListeners(store, selector, listenerManager, logger)
	mux.Handle("/api/config", apiHandler)
	mux.Handle("/api/config/", apiHandler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("/", web.Handler())

	adminListener, err := net.Listen("tcp", runtimeConfig.WebAddr)
	if err != nil {
		return fmt.Errorf("listen admin on %s: %w", runtimeConfig.WebAddr, err)
	}
	results := make(chan serviceResult, 1)
	go serve(results, "admin", func() error { return serveAdmin(runCtx, adminListener, mux) })

	logger.Info("service.started",
		"web_addr", runtimeConfig.WebAddr,
		"config_path", runtimeConfig.ConfigPath,
		"listener_count", len(storedConfig.Listeners),
	)
	first := <-results
	cancel()
	if first.err != nil {
		return fmt.Errorf("%s service: %w", first.name, first.err)
	}
	logger.Info("service.stopped")
	return nil
}

func serve(results chan<- serviceResult, name string, run func() error) {
	results <- serviceResult{name: name, err: run()}
}

func serveAdmin(ctx context.Context, listener net.Listener, handler http.Handler) error {
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve admin HTTP: %w", err)
	}
	return nil
}
