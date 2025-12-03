package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	port := flag.String("port", "9001", "chunkserver port")
	flag.Parse()

	addr := ":" + *port

	srv := setupServer(addr)
	startHTTPServer(srv)

	// Register with master
	startRegistration(*port)

	// Heartbeats
	stopHeartbeat := make(chan struct{})
	startHeartbeats(*port, stopHeartbeat)

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("chunk-server shutting down…")
	close(stopHeartbeat)
	srv.Close()
}
