package mobileproxy

import (
	"io"
	"net/http"
	"net/url"
)

func Get(w http.ResponseWriter, r *http.Request) {
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
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.Header().Add("X-Rob-Proxy", "0.1")
	w.WriteHeader(srvResp.StatusCode)
	io.Copy(w, srvResp.Body)
}
