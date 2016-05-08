package mobileproxy

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
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

func AuthorisationHandler(next http.Handler, users map[string]string) http.Handler {

	fail := func(w http.ResponseWriter) {
		w.Header().Set("Proxy-Authenticate", "basic realm=\"Shorty Mobile Proxy\"")
		http.Error(w, "Proxy authentication required", http.StatusProxyAuthRequired)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		v := r.Header.Get("Proxy-authorization")
		//Proxy-Authorization:[Basic cm9mbDpjb3B0ZXI=]
		if v == "" {
			log.Print("Failed to find proxy-authorization header")
			fail(w)
			return
		}
		parts := strings.Split(v, " ")
		if len(parts) != 2 {
			log.Print("Failed to split proxy-authorization in to 2 parts")
			fail(w)
			return
		}

		reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(parts[1]))
		b, err := ioutil.ReadAll(reader)
		if err != nil {
			log.Printf("Failed to interpret proxy-authorization header value: '%s'", v)
			fail(w)
			return
		}
		parts = strings.SplitN(string(b), ":", 2)
		if len(parts) != 2 {
			log.Print("Failed to split proxy base64-decoded string in to 2 parts")
			fail(w)
			return
		}
		if password, ok := users[parts[0]]; !ok {
			log.Printf("Failed to authenticate: User %s not found", parts[0])
			fail(w)
			return
		} else if password != parts[1] {
			log.Printf("Failed to authenticate: Password for user %s incorrect", parts[0])
			fail(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}
