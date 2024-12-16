package main

import (
	"net/http/httptest"
	"testing"
)

func TestAuthorization(t *testing.T) {
	config := buildTestConfig(defaultAlgorithm, defaultAPIKey)

	data := []struct {
		name          string
		request       string
		authorization string
		expected      int
	}{
		{"good api key", "http://example.com:8080/api", defaultAPIKey, 200},
		{"bad api key", "http://example.com:8080/api", "badapikey", 403},
		{"no api key", "http://example.com:8080/api", "", 400},
	}

	handler := APIHandler(config)

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", d.request, nil)
			if d.authorization != "" {
				req.Header.Add("Authorization", d.authorization)
			}
			w := httptest.NewRecorder()

			handler(w, req)
			if w.Result().StatusCode != d.expected {
				t.Errorf("expected %d, got %d", d.expected, w.Result().StatusCode)
			}
		})
	}

	teardownTest(config)
}
