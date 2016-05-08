package mobileproxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func expectEqualsStr(t *testing.T, actual string, expected string, message string) {
	if expected != actual {
		t.Fatalf("%s. Expected '%s', actual '%s'", message, expected, actual)
	}
}

func getProxy() *httptest.Server {
	return httptest.NewServer(ProxyHandler())
}

func getClient(proxy *url.URL) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: func(_ *http.Request) (*url.URL, error) {
				return proxy, nil
			},
		},
	}
}

func TestGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectEqualsStr(t, r.Method, http.MethodGet, "Bad method")
		expectEqualsStr(t, r.Proto, "HTTP/1.1", "Bad proto")
		expectEqualsStr(t, r.RequestURI, "/rofl", "Bad request URI")

		fmt.Fprint(w, "Hello, client")
	}))
	defer ts.Close()

	proxy := getProxy()
	proxyURL, _ := url.Parse(proxy.URL)
	client := getClient(proxyURL)

	requestURL, _ := url.Parse(ts.URL)
	requestURL.Path = "/rofl"

	resp, err := client.Get(requestURL.String())

	if err != nil {
		t.Error("Error attempting client.Get - " + err.Error())
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("Error reading client body - " + err.Error())
	}

	expected := "Hello, client"
	actual := string(body)

	expectEqualsStr(t, actual, expected, "Bad content")
	expectEqualsStr(t, resp.Header.Get("X-rob-proxy"), "0.1", "Bad X-Rob-Proxy header")
}

func getRequestServer(h http.Handler) (errChan chan error, req http.Request, closer func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	errChan = make(chan error)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errChan <- err
			return
		}
		defer conn.Close()
		clientReader := bufio.NewReader(conn)

		req, err := http.ReadRequest(clientReader)
		if err != nil {
			errChan <- err
			return
		}

		h.ServeHTTP(newRawResponseWriter(conn), req)
		errChan <- nil
	}()

	serverURL, err := url.Parse(fmt.Sprintf("http://%s/", ln.Addr().String()))
	if err != nil {
		panic(err)
	}
	req = http.Request{
		URL:    serverURL,
		Header: make(http.Header),
	}

	closer = func() {
		ln.Close()
	}

	return errChan, req, closer
}

// rawResponseWriter implements http.ResponseWriter in a very raw fashion
// It does not sniff content-type nor does it do chunked encoding and/or content-length for you
type rawResponseWriter struct {
	header         http.Header
	headersWritten bool
	writer         io.Writer
}

func newRawResponseWriter(writer io.Writer) *rawResponseWriter {
	return &rawResponseWriter{
		header: make(http.Header),
		writer: writer,
	}
}

func (w *rawResponseWriter) Header() http.Header {
	return w.header
}

func (w *rawResponseWriter) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return w.writer.Write(b)
}

func (w *rawResponseWriter) WriteHeader(statusCode int) {
	if !w.headersWritten {
		w.headersWritten = true

		statusText := http.StatusText(statusCode)
		if statusText == "" {
			statusText = "???"
		}
		w.writer.Write([]byte(fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, statusText)))
		for k, vs := range w.header {
			for _, v := range vs {
				w.writer.Write([]byte(fmt.Sprintf("%s: %s\r\n", k, v)))
			}
		}
		w.writer.Write([]byte("\r\n"))
	}
}

func hitProxy(req http.Request) (resp *http.Response, closer func()) {
	proxy := getProxy()
	proxyURL, _ := url.Parse(proxy.URL)
	proxyConn, err := net.Dial("tcp", proxyURL.Host)
	if err != nil {
		panic(err)
	}

	if err := req.WriteProxy(proxyConn); err != nil {
		panic(err)
	}

	reader := bufio.NewReader(proxyConn)
	resp, err = http.ReadResponse(reader, nil)
	if err != nil {
		panic(err)
	}
	closer = func() {
		proxyConn.Close()
	}

	return resp, closer
}

func TestManually(t *testing.T) {
	handler := func(w http.ResponseWriter, req *http.Request) {
		str := "OK"
		w.Header().Add("Content-type", "text/plain")
		w.Header().Add("Content-length", strconv.Itoa(len(str)))
		w.Write([]byte(str))
	}
	errChan, req, closer := getRequestServer(http.HandlerFunc(handler))
	defer closer()

	resp, closer := hitProxy(req)
	defer closer()

	if resp.StatusCode != 200 {
		t.Fatalf("Bad status code: %d", resp.StatusCode)
	}

	var err error
	select {
	case err = <-errChan:
		break
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for response from server")
	}

	if err != nil {
		t.Fatal("Error from server routine: " + err.Error())
	}
}

func TestGzipResponsesArePassedThrough(t *testing.T) {
	content := "This is example gzipped/compressed data"
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-type", "text/plain")
		w.Header().Add("Content-length", strconv.Itoa(len(content)))
		w.Header().Add("Content-encoding", "gzip")
		w.Write([]byte(content))
	}
	errChan, req, closer := getRequestServer(http.HandlerFunc(handler))
	defer closer()
	req.Header.Set("Accept-encoding", "gzip")

	resp, closer := hitProxy(req)
	defer closer()

	if resp.StatusCode != 200 {
		t.Fatalf("Bad status code: %d", resp.StatusCode)
	}
	expectEqualsStr(t, resp.Header.Get("Content-encoding"), "gzip", "Bad content-encoding header")
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	expectEqualsStr(t, string(body), content, "Bad body content")

	var err error
	select {
	case err = <-errChan:
		break
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for response from server")
	}

	if err != nil {
		t.Fatal("Error from server routine: " + err.Error())
	}
}

func TestNonGzippedResponsesGetGzipped(t *testing.T) {
	var content bytes.Buffer
	for content.Len() < 10240 {
		content.Write([]byte("hello "))
	}
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-type", "text/plain")
		w.Header().Add("Content-length", strconv.Itoa(content.Len()))
		w.Write(content.Bytes())
	}
	errChan, req, closer := getRequestServer(http.HandlerFunc(handler))
	defer closer()
	req.Header.Set("Accept-encoding", "gzip")

	resp, closer := hitProxy(req)
	defer closer()

	if resp.StatusCode != 200 {
		t.Fatalf("Bad status code: %d", resp.StatusCode)
	}
	expectEqualsStr(t, resp.Header.Get("Content-encoding"), "gzip", "Bad content-encoding header")
	// go stdlib strips the header off and puts it in its own field
	if len(resp.TransferEncoding) != 1 && resp.TransferEncoding[0] != "chunked" {
		t.Fatalf("Bad transfer encoding: %v", resp.TransferEncoding)
	}
	expectEqualsStr(t, resp.Header.Get("Content-length"), "", "Bad content-length header")
	reader, err := gzip.NewReader(httputil.NewChunkedReader(resp.Body))
	if err != nil {
		t.Fatal("Error creating gzip reader: " + err.Error())
	}
	body, err := ioutil.ReadAll(reader)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Error reading via chunked+gzip reader: " + err.Error())
	}
	expectEqualsStr(t, string(body), content.String(), "Bad body content")

	select {
	case err = <-errChan:
		break
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for response from server")
	}

	if err != nil {
		t.Fatal("Error from server routine: " + err.Error())
	}
}
