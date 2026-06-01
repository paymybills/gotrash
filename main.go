package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gotrash/internal/server"
	"gotrash/internal/store"
)

func main() {
	// Define CLI flags
	portFlag := flag.Int("port", 8080, "Port to listen on")
	uploadDirFlag := flag.String("dir", "./data/uploads", "Directory to save uploaded files")
	cleanIntervalFlag := flag.Duration("clean", 10*time.Second, "Interval at which to run the expired items janitor")
	flag.Parse()

	log.Println("Initializing gotrash server...")

	// 1. Initialize Storage Backend
	db, err := store.NewStore(*uploadDirFlag)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize store: %v", err)
	}

	// 2. Start Expired Items Janitor Loop
	stopJanitorChan := make(chan struct{})
	db.StartJanitor(*cleanIntervalFlag, stopJanitorChan)
	log.Printf("INFO: Background Janitor started (sweeping every %v)", *cleanIntervalFlag)

	// 3. Set Up Graceful Shutdown Context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 4. Initialize HTTP Server
	addr := fmt.Sprintf(":%d", *portFlag)
	srv, err := server.NewServer(addr, db)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize server: %v", err)
	}

	// 5. Start Server
	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("FATAL: Server crashed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down gotrash daemon gracefully...")

	// Stop background Janitor
	close(stopJanitorChan)
	log.Println("Background Janitor stopped.")

	// Allow brief moment for final logging
	time.Sleep(500 * time.Millisecond)
	log.Println("gotrash server stopped. Goodbye!")
}
