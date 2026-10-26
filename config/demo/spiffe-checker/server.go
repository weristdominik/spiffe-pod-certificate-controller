package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

func (c *Checker) RunServer(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		peer := r.TLS.PeerCertificates[0]
		log.Printf("trusted client: %s", c.Config.ExpectedClient)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "Authenticated client: %s\n", peer.URIs)
	})

	server := &http.Server{
		Addr:              c.Config.ListenAddress,
		Handler:           mux,
		TLSConfig:         c.serverTLS(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("HTTPS server listening on %s", c.Config.ListenAddress)
	return server.ListenAndServeTLS("", "")
}
