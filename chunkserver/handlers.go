package main

import (
	"fmt"
	"net/http"
)

func helloHandler(serverAddr string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "chunk-server %s online\n", serverAddr)
	}
}
