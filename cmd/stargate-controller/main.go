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
	"github.com/songzhibin97/stargate/internal/controller"
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
		fmt.Printf("Stargate Controller %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create controller server
	server, err := controller.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create controller server: %v", err)
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting Stargate Controller on %s", cfg.Controller.Address)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Start configuration synchronization
	go func() {
		log.Println("Starting configuration synchronization...")
		if err := server.StartSync(); err != nil {
			log.Printf("Failed to start sync: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Stargate Controller...")

	// Create a deadline to wait for
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop synchronization first
	server.StopSync()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	} else {
		log.Println("Server gracefully stopped")
	}
}
