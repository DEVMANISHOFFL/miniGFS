package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(0)

	srv := setupServer()
	go sweeper()
	
	go func() {
		fmt.Printf("\033[31mmaster:\033[0m server starting on port: %s\n", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("\033[31mmaster:\033[0m error starting server: %v\n", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	fmt.Printf("\033[31mmaster:\033[0m received shutdown signal, shutting down...\n")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("\033[31mmaster:\033[0m graceful shutdown failed: %v\n", err)
	} else {
		fmt.Printf("\033[31mmaster:\033[0m shutdown complete\n")
	}
}
