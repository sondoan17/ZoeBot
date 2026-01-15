// ZoeBot - Discord Bot for League of Legends Match Analysis
// Optimized for minimal resource usage.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/zoebot/internal/bot"
	"github.com/zoebot/internal/config"
	"github.com/zoebot/pkg/healthcheck"
)

func init() {
	// Optimize garbage collector for low memory
	// GOGC=50 means GC runs more frequently, using less memory
	debug.SetGCPercent(50)

	// Limit max memory usage (soft limit)
	debug.SetMemoryLimit(50 * 1024 * 1024) // 50MB

	// Use minimal number of OS threads
	runtime.GOMAXPROCS(1)
}

func main() {
	// Health check flag for Docker
	healthFlag := flag.Bool("health", false, "Run health check")
	flag.Parse()

	if *healthFlag {
		if err := runHealthCheck(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Minimal logging - write directly to stdout for Docker
	log.SetFlags(log.Ltime)
	log.SetOutput(os.Stdout)
	log.Println("Starting ZoeBot...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Config invalid: %v", err)
	}

	// Create bot
	discordBot, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("Bot error: %v", err)
	}

	// Start health check server (lightweight)
	healthServer := healthcheck.New(":8080")
	go func() {
		if err := healthServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health server error: %v", err)
		}
	}()

	// Start bot
	if err := discordBot.Start(); err != nil {
		log.Fatalf("Start error: %v", err)
	}

	log.Println("ZoeBot running")

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")

	// Graceful shutdown with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	healthServer.Stop(ctx)
	discordBot.Stop()

	log.Println("Stopped")
}

// runHealthCheck performs a quick health check
func runHealthCheck() error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://localhost:8080/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: %d", resp.StatusCode)
	}
	return nil
}
