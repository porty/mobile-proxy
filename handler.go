package mobileproxy

import (
	"log"
	"net/http"
)

func ProxyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			connect(w, r)
		} else if r.Method == http.MethodGet {
			get(w, r)
		} else {
			log.Print("I don't know how to handle this")
		}
	})
}
