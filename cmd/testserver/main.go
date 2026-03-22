package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.Int("port", 9999, "port to listen on")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("test server listening on %s", addr)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "test service running")
	})

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("test server error: %v", err)
	}
}
