package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"

	_ "net/http/pprof"
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
		httpHandler(),
	)
	if err != nil {
		panic(err)
	}
}

func httpHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			connect(w, r)
		} else if r.Method == http.MethodGet {
			get(w, r)
			log.Print(r.RequestURI)
			log.Printf("%#v", *r)
		} else {
			log.Print(r.RequestURI)
			log.Printf("%#v", *r)
		}
	})
}

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

func get(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.RequestURI)
	if err != nil {
		http.Error(w, "Bad URL", 400)
		return
	}
	newReq := http.Request{
		Method: r.Method,
		URL:    u,
		Header: r.Header,
		Close:  r.Close,
	}
	c := http.Client{}
	srvResp, err := c.Do(&newReq)
	if err != nil {
		http.Error(w, "Proxy failed with backend request: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer srvResp.Body.Close()
	for k, vs := range srvResp.Header {
		if k != "Content-Encoding" && k != "Transfer-Encoding" {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
	}
	w.WriteHeader(srvResp.StatusCode)
	io.Copy(w, srvResp.Body)
}