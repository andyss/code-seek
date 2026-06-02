package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/andyss/code-seek/handler"
)

func main() {
	addr := flag.String("addr", ":15654", "HTTP server address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/search", handler.Search)
	mux.HandleFunc("/content", handler.Content)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("code-seek listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
