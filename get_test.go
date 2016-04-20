package mobileproxy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func expectEqualsStr(t *testing.T, actual string, expected string, message string) {
	if expected != actual {
		t.Errorf("%s. Expected '%s', actual '%s'", message, expected, actual)
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
