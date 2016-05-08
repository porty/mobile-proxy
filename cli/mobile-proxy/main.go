package main

import (
	"log"
	"net/http"

	_ "net/http/pprof"

	"github.com/porty/mobile-proxy"
)

var users = map[string]string{
	"user": "password",
}

func main() {
	go func() {
		// for pprof
		http.ListenAndServe("localhost:8081", nil)
	}()

	log.Print("Listening on http://localhost:8081/debug/ for pprof")
	log.Print("Listening on http://localhost:8080/ for proxy")
	handler := mobileproxy.ProxyHandler()
	handler = mobileproxy.AuthorisationMiddleware(handler, users)
	handler = mobileproxy.LogMiddleware(handler)
	err := http.ListenAndServe(
		":8080",
		handler,
	)
	if err != nil {
		panic(err)
	}
}
