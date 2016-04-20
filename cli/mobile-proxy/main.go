package main

import (
	"log"
	"net/http"

	_ "net/http/pprof"

	"github.com/porty/mobile-proxy"
)

func main() {
	go func() {
		// for pprof
		http.ListenAndServe("localhost:8081", nil)
	}()

	log.Print("Listening on http://localhost:8081/debug/ for pprof")
	log.Print("Listening on http://localhost:8080/ for proxy")
	err := http.ListenAndServe(
		":8080",
		mobileproxy.LogHandler(mobileproxy.ProxyHandler()),
	)
	if err != nil {
		panic(err)
	}
}
