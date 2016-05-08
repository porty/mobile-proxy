package mobileproxy

import (
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func clientCanGzip(req *http.Request) bool {
	return strings.Contains(req.Header.Get("Accept-encoding"), "gzip")
}

func responseAlreadyEncoded(resp *http.Response) bool {
	ce := resp.Header.Get("Content-encoding")
	if ce != "" {
		return true
	}
	return resp.Header.Get("Transfer-encoding") != ""
}

var compressedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"video/mp4":  true,
}

func isWorthGzipping(resp *http.Response) bool {
	if resp.ContentLength < 1024 {
		return false
	}

	if resp.Header.Get("Content-type") == "" {
		// putting content-type: gzip on something without a content-type doesn't work right
		return false
	}

	alreadyCompressed := compressedMimeTypes[resp.Header.Get("Content-type")]
	return !alreadyCompressed
}

func do(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		defer r.Body.Close()
	}
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

func unknownMethodHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Not sure how to do a %s request for %s", r.Method, r.URL.String())

	http.Error(w, "Unknown request method: "+r.Method, http.StatusBadRequest)
}
