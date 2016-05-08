package mobileproxy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TesPost(t *testing.T) {
	postContent := "cat=meow&dog=woof"
	responseContent := "Hello, client"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectEqualsStr(t, r.Method, http.MethodPost, "Bad method")
		expectEqualsStr(t, r.Proto, "HTTP/1.1", "Bad proto")
		expectEqualsStr(t, r.RequestURI, "/rofl", "Bad request URI")
		expectEqualsStr(t, r.Header.Get("Content-length"), strconv.Itoa(len(postContent)), "Bad content-length header")
		expectEqualsStr(t, r.Header.Get("Content-type"), "application/x-www-form-urlencoded", "Bad content-type header")
		body, _ := ioutil.ReadAll(r.Body)

		expectEqualsStr(t, string(body), postContent, "Bad post content")
		fmt.Fprint(w, responseContent)
	}))
	defer ts.Close()

	proxy := getProxy()
	proxyURL, _ := url.Parse(proxy.URL)
	client := getClient(proxyURL)

	requestURL, _ := url.Parse(ts.URL)
	requestURL.Path = "/rofl"

	resp, err := client.Post(requestURL.String(), "application/x-www-form-urlencoded", bytes.NewReader([]byte(postContent)))

	if err != nil {
		t.Error("Error attempting client.Post - " + err.Error())
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error("Error reading client body - " + err.Error())
	}

	expectEqualsStr(t, string(body), responseContent, "Bad response content")
	expectEqualsStr(t, resp.Header.Get("X-rob-proxy"), "0.1", "Bad X-Rob-Proxy header")
}
