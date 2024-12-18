package main

import (
	"io"
	"net/http"
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
		{"normally good api key", "http://example.com:8080/api", defaultAPIKey, http.StatusForbidden},
		{"bad api key", "http://example.com:8080/api", "badapikey", http.StatusForbidden},
		{"no api key", "http://example.com:8080/api", "", http.StatusBadRequest},
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
		// Although this is a good key, with no action the return is 400.
		{"good api key", "http://example.com:8080/api", defaultAPIKey, http.StatusBadRequest},
		{"bad api key", "http://example.com:8080/api", "badapikey", http.StatusForbidden},
		{"no api key", "http://example.com:8080/api", "", http.StatusBadRequest},
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

func TestInsertRemoveInfoHash(t *testing.T) {
	config := buildTestConfig(defaultAlgorithm, defaultAPIKey)

	data := []struct {
		name          string
		request       string
		authorization string
		expectedbody  string
		expectedcode  int
	}{
		// Inserting a duplicate key
		{"insert dupe", "http://example.com:8080/api?action=insert_infohash&info_hash=aaaaaaaaaaaaaaaaaaaa&note=hello", defaultAPIKey, "info_hash aaaaaaaaaaaaaaaaaaaa already inserted", http.StatusBadRequest},
		{"insert", "http://example.com:8080/api?action=insert_infohash&info_hash=zzzzzzzzzzzzzzzzzzzz&note=hello", defaultAPIKey, "", http.StatusOK},
		{"remove", "http://example.com:8080/api?action=remove_infohash&info_hash=aaaaaaaaaaaaaaaaaaaa", defaultAPIKey, "", http.StatusOK},
		{"missing action", "http://example.com:8080/api?info_hash=aaaaaaaaaaaaaaaaaaaa", defaultAPIKey, "", http.StatusBadRequest},
		{"bad infohash", "http://example.com:8080/api?action=insert_infohash&info_hash=a", defaultAPIKey, "", http.StatusBadRequest},
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
			resp := w.Result()
			if resp.StatusCode != d.expectedcode {
				t.Errorf("expected %d, got %d", d.expectedcode, resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if string(body) != d.expectedbody {
				t.Errorf("expected %s, got %s", d.expectedbody, string(body))
			}
		})
	}

	teardownTest(config)
}
