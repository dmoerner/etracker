package main

import (
	"net/http/httptest"
	"testing"
)

// TestUnsetAuthorization is a critical test. Due to the lack of optional
// types, an empty authorization string in the config struct is required by the
// program to reject all API attempts, including with an empty string in
// the Authorization Header.
func TestUnsetAuthorization(t *testing.T) {
	config := buildTestConfig(defaultAlgorithm, "")

	data := []struct {
		name          string
		request       string
		authorization string
		expected      int
	}{
		{"normally good api key", "http://example.com:8080/api", defaultAPIKey, 403},
		{"bad api key", "http://example.com:8080/api", "badapikey", 403},
		{"no api key", "http://example.com:8080/api", "", 403},
	}

	handler := APIHandler(config)

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", d.request, nil)
			req.Header.Add("Authorization", d.authorization)
			w := httptest.NewRecorder()

			handler(w, req)
			if w.Result().StatusCode != d.expected {
				t.Errorf("expected %d, got %d", d.expected, w.Result().StatusCode)
			}
		})
	}

	teardownTest(config)
}

func TestAuthorizationHeader(t *testing.T) {
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
