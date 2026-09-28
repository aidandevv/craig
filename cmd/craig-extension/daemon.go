package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aidandevv/craig-extension/internal/cache"
	"github.com/aidandevv/craig-extension/internal/config"
	"github.com/aidandevv/craig-extension/internal/daemon"
	"github.com/aidandevv/craig-extension/internal/signals"
)

const (
	daemonReadHeaderTimeout = 5 * time.Second
	daemonReadTimeout       = 15 * time.Second
	// The extension cancels an image analysis after 45 seconds. Leave the
	// daemon enough time to return its structured result without permitting an
	// unbounded local connection.
	daemonWriteTimeout  = 60 * time.Second
	daemonIdleTimeout   = 60 * time.Second
	daemonMaxHeaderSize = 8 << 10
)

func runDaemon(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	fs.SetOutput(stdout)
	path := fs.String("config", "", "configuration file path")
	verbose := fs.Bool("verbose", false, "print safe per-request execution traces to stdout")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return exitUsage
	}
	if *path == "" {
		defaultPath, err := config.DefaultPath()
		if err != nil {
			fmt.Fprintf(stdout, "error: %v\n", err)
			return exitRuntime
		}
		*path = defaultPath
	}
	cfg, err := config.Load(*path)
	if err != nil {
		fmt.Fprintf(stdout, "error: %v\n", err)
		return exitRuntime
	}
	if cfg.Daemon.ID == "" {
		id, err := config.EnsureDaemonID(*path)
		if err != nil {
			fmt.Fprintf(stdout, "error: create daemon ID: %v\n", err)
			return exitRuntime
		}
		cfg.Daemon.ID = id
	}
	store, err := cache.Open(cfg.Cache.Location)
	if err != nil {
		fmt.Fprintf(stdout, "error: open cache: %v\n", err)
		return exitRuntime
	}
	defer store.Close()
	if _, err := store.Prune(context.Background(), time.Duration(cfg.Cache.MaxAgeDays)*24*time.Hour); err != nil {
		fmt.Fprintf(stdout, "error: prune cache: %v\n", err)
		return exitRuntime
	}
	vision := signals.NewVision(store, signals.NopMeter{}, signals.Options{
		APIKey: cfg.APIKeys.GoogleVision, UseADC: cfg.Vision.UseADC,
		MonthlyCap: cfg.Vision.MonthlyUnitCap,
	})
	handler, err := daemon.New(daemon.Options{
		Port: cfg.Daemon.Port, Token: cfg.Daemon.Token, DaemonID: cfg.Daemon.ID, RulesPath: cfg.Rules.File,
		AutoReload: cfg.Rules.AutoReload, CacheLocation: cfg.Cache.Location, Vision: vision,
		SetVisionAPIKey: func(apiKey string) (*signals.Vision, error) {
			settings, err := config.SetGoogleVisionAPIKey(*path, apiKey)
			if err != nil {
				return nil, err
			}
			return signals.NewVision(store, signals.NopMeter{}, signals.Options{
				APIKey: apiKey, UseADC: settings.UseADC, MonthlyCap: settings.MonthlyUnitCap,
			}), nil
		},
		Verbose: *verbose, VerboseWriter: stdout,
	})
	if err != nil {
		fmt.Fprintf(stdout, "error: start daemon: %v\n", err)
		return exitRuntime
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Daemon.Port)))
	if err != nil {
		fmt.Fprintf(stdout, "error: bind 127.0.0.1:%d: %v\n", cfg.Daemon.Port, err)
		return exitRuntime
	}
	server := newDaemonHTTPServer(handler)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	fmt.Fprintf(stdout, "Craig helper is ready at http://127.0.0.1:%d (ID %s)\n", cfg.Daemon.Port, cfg.Daemon.ID)
	if *verbose {
		fmt.Fprintln(stdout, "verbose execution traces enabled; credentials, listing text, and image URLs are not logged")
	}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stdout, "error: serve daemon: %v\n", err)
		return exitRuntime
	}
	return exitOK
}

func newDaemonHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: daemonReadHeaderTimeout,
		ReadTimeout:       daemonReadTimeout,
		WriteTimeout:      daemonWriteTimeout,
		IdleTimeout:       daemonIdleTimeout,
		MaxHeaderBytes:    daemonMaxHeaderSize,
	}
}
