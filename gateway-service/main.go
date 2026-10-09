package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	handler, err := newGatewayHandler(os.Getenv("RESERVATION_SERVICE_URL"))
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
	}
	log.Println("Gateway Service listening on :8080")
	return server.ListenAndServe()
}

func newGatewayHandler(reservationURL string) (http.Handler, error) {
	target, err := url.Parse(reservationURL)
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return nil, fmt.Errorf("RESERVATION_SERVICE_URL must be a valid HTTP or HTTPS URL")
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(target)
			request.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("reservation service request failed: %v", err)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			if err := json.NewEncoder(w).Encode(map[string]string{"message": "Reservation Service unavailable"}); err != nil {
				log.Printf("write error response: %v", err)
			}
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /manage/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /api/v1/hotels", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})
	return mux, nil
}
