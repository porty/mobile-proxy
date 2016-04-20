package mobileproxy

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func getProxy() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(Get))
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

	if actual != expected {
		t.Errorf("Content mismatch, expected '%s', received '%s'", expected, actual)
	}

	if resp.Header.Get("X-rob-proxy") != "0.1" {
		t.Errorf("Expected X-Rob-Proxy header value of 0.1, receieved %s", resp.Header.Get("X-rob-proxy"))
	}

}
