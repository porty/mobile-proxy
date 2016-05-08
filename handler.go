package mobileproxy

import "net/http"

func ProxyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodConnect:
			connect(w, r)
		case http.MethodDelete, http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut:
			do(w, r)
		default:
			unknownMethodHandler(w, r)
		}
	})
}
