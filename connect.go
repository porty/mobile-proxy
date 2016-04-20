package mobileproxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
)

func connect(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	host := r.RequestURI
	conn, err := net.Dial("tcp", host)
	if err != nil {
		message := fmt.Sprintf("Failed to connect to %s - %s", host, err.Error())
		log.Printf(message)
		http.Error(w, message, http.StatusBadGateway)
		return
	}
	defer conn.Close()

	hij, ok := w.(http.Hijacker)
	if !ok {
		panic("response does not support hijacking")
	}

	rawClient, _, err := hij.Hijack()
	if err != nil {
		message := fmt.Sprintf("Failed to get raw connection for client - %s", err.Error())
		log.Print(message)
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	defer rawClient.Close()

	rawClient.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
	log.Printf("Connected to %s", host)
	pipe(conn, rawClient)
	log.Printf("Finished with %s", host)
}

func pipe(a, b io.ReadWriter) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		io.Copy(a, b)
		wg.Done()
	}()
	go func() {
		io.Copy(b, a)
		wg.Done()
	}()
	wg.Wait()
}
