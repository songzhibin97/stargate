package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/proxy"
	"github.com/songzhibin97/stargate/internal/router"
)

var (
	configFile = flag.String("config", "config.yaml", "Configuration file path")
	version    = flag.Bool("version", false, "Show version information")
)

const (
	// Version information
	Version   = "v1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("Stargate Node %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration source settings
	if err := config.ValidateSourceConfig(cfg); err != nil {
		log.Fatalf("Invalid configuration source settings: %v", err)
	}

	// Create configuration source based on driver
	configSource, err := config.CreateConfigSource(cfg)
	if err != nil {
		log.Fatalf("Failed to create configuration source: %v", err)
	}
	defer func() {
		if err := configSource.Close(); err != nil {
			log.Printf("Error closing configuration source: %v", err)
		}
	}()

	// Create routing engine
	routingEngine := router.NewEngine(cfg)

	// Create configuration store with the routing engine
	configStore, err := router.NewStore(configSource, routingEngine)
	if err != nil {
		log.Fatalf("Failed to create configuration store: %v", err)
	}

	// Start configuration store
	ctx := context.Background()
	if err := configStore.Start(ctx); err != nil {
		log.Fatalf("Failed to start configuration store: %v", err)
	}
	defer func() {
		if err := configStore.Stop(); err != nil {
			log.Printf("Error stopping configuration store: %v", err)
		}
	}()

	log.Printf("Configuration source initialized: driver=%s", cfg.ConfigSource.Source.Driver)

	// Create proxy server
	server, err := proxy.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting Stargate Node on %s", cfg.Server.Address)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Stargate Node...")

	// Create a deadline to wait for
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	} else {
		log.Println("Server gracefully stopped")
	}
}
