package main

import "net/http"

// APIHandler handles requests to the /api endpoint. It requires an appropriate
// authorization header, which is currently a single secret string managed by
// an environment variable.
func APIHandler(config Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization. An empty authorization value or no key
		// in the config means API access is forbidden.
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if config.authorization == "" || authorization != config.authorization {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}
}
