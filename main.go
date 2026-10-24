// Command prometheus-inseego-exporter is a Prometheus exporter for Inseego MiFi
// devices (e.g. MiFi X PRO 5G / M3000). It reads GPS data from the "GPS over
// Wi-Fi" NMEA TCP stream and device status from the /srv/status JSON endpoint.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/hartfordfive/prometheus-inseego-exporter/internal/collector"
	"github.com/hartfordfive/prometheus-inseego-exporter/internal/config"
	"github.com/hartfordfive/prometheus-inseego-exporter/internal/status"
)

func main() {
	cfg, err := config.Parse(os.Args[0], os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if errors.Is(err, config.ErrShowVersion) {
		fmt.Println(versionString())
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(cfg.Log.Level)); err != nil {
		fmt.Fprintf(os.Stderr, "invalid log level: %v\n", err)
		os.Exit(2)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	c := collector.New(collector.Config{
		Address:     cfg.Mifi.Address,
		DialTimeout: cfg.Mifi.DialTimeout,
		ReadTimeout: cfg.Mifi.ReadTimeout,
		Interval:    cfg.GPS.Interval,
		StaleAfter:  cfg.GPS.StaleAfter,
	}, logger)
	go c.Run(ctx)

	reg := prometheus.NewRegistry()
	reg.MustRegister(c, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), buildInfo())
	if cfg.Status.Enabled {
		sc := status.New(status.Config{URL: cfg.Status.URL, Timeout: cfg.Status.Timeout, Interval: cfg.Status.Interval}, logger)
		go sc.Run(ctx)
		reg.MustRegister(sc)
	}

	mux := http.NewServeMux()
	mux.Handle(cfg.Web.TelemetryPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, "<html><head><title>Inseego Exporter</title></head><body><h1>Inseego Exporter</h1><p><a href=%q>Metrics</a></p></body></html>", cfg.Web.TelemetryPath)
	})

	srv := &http.Server{Addr: cfg.Web.ListenAddress, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.Info("starting exporter", "version", version, "commit", commit, "build_date", buildDate, "listen", cfg.Web.ListenAddress, "mifi", cfg.Mifi.Address)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server failed", "err", err)
		os.Exit(1)
	}
}
