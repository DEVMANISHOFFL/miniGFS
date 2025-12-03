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

func Hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "master is here")
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/hi", Hello)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		fmt.Printf("master: starting server on port %v\n", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("master: ListenAndServe error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	fmt.Println("master: received shutdown signal, shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("master: graceful shutdown failed: %v", err)
	} else {
		log.Println("master: shutdown completed")
	}
}
