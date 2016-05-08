package mobileproxy

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func post(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
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
		Body:   r.Body,
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

	if clientCanGzip(r) && !responseAlreadyEncoded(srvResp) && isWorthGzipping(srvResp) {
		// gzip the content
		// seeing we don't know the output gzip length, we have to chunk it and throw away any existing content-length header
		w.Header().Set("Content-encoding", "gzip")
		w.Header().Set("Transfer-encoding", "chunked")
		w.Header().Del("Content-length")
		w.WriteHeader(srvResp.StatusCode)
		cw := httputil.NewChunkedWriter(w)
		gw := gzip.NewWriter(cw)
		io.Copy(gw, srvResp.Body)
		gw.Close()
		cw.Close()
	} else {
		w.WriteHeader(srvResp.StatusCode)
		io.Copy(w, srvResp.Body)
	}
}
