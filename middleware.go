package mobileproxy

import (
	"log"
	"net/http"
	"time"
)

func LogHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("Start %s %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		log.Printf("End %s %s, duration %s", r.Method, r.RequestURI, duration.String())
	})
}
