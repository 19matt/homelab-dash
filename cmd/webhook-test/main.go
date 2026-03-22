package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		log.Printf("=== WEBHOOK RECEIVED ===")
		log.Printf("Method: %s", r.Method)
		log.Printf("Headers: %v", r.Header)
		log.Printf("Body: %s", string(body))
		log.Printf("========================")
		w.WriteHeader(200)
		fmt.Fprint(w, "ok")
	})

	log.Println("webhook receiver listening on :9090")
	log.Fatal(http.ListenAndServe(":9090", nil))
}
