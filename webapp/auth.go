package main

import (
	"crypto/subtle"
	"net/http"
)

// BasicAuth wraps h with HTTP basic auth. When both user and pass are empty,
// authentication is disabled entirely (the default for a LAN/Umbrel install).
func BasicAuth(h http.Handler, user, pass string) http.Handler {
	if user == "" && pass == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || !secureEqual(u, user) || !secureEqual(p, pass) {
			w.Header().Set("WWW-Authenticate", `Basic realm="OrdexCoin Web"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func secureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
