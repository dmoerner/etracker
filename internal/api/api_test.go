package api

import (
	"etracker/internal/testutils"
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
	conf := testutils.BuildTestConfig(nil, "")
	defer testutils.TeardownTest(conf)

	data := []struct {
		name          string
		request       string
		authorization string
		expected      int
	}{
		{"normally good api key", "http://example.com:8080/api", testutils.DefaultAPIKey, http.StatusForbidden},
		{"bad api key", "http://example.com:8080/api", "badapikey", http.StatusForbidden},
		{"no api key", "http://example.com:8080/api", "", http.StatusBadRequest},
	}

	handler := APIHandler(conf)

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
}

func TestAuthorizationHeader(t *testing.T) {
	conf := testutils.BuildTestConfig(nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	data := []struct {
		name          string
		request       string
		authorization string
		expected      int
	}{
		// Although this is a good key, with no action the return is 400.
		{"good api key", "http://example.com:8080/api", testutils.DefaultAPIKey, http.StatusBadRequest},
		{"bad api key", "http://example.com:8080/api", "badapikey", http.StatusForbidden},
		{"no api key", "http://example.com:8080/api", "", http.StatusBadRequest},
	}

	handler := APIHandler(conf)

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
}

func TestInsertRemoveInfoHash(t *testing.T) {
	conf := testutils.BuildTestConfig(nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	data := []struct {
		name          string
		request       string
		authorization string
		expectedbody  string
		expectedcode  int
	}{
		// Inserting a duplicate key
		{"insert", "http://example.com:8080/api?action=insert_infohash&info_hash=ffffffffffffffffffffffffffffffffffffffff&note=hello", testutils.DefaultAPIKey, "", http.StatusOK},
		{"insert dupe", "http://example.com:8080/api?action=insert_infohash&info_hash=ffffffffffffffffffffffffffffffffffffffff&note=hello", testutils.DefaultAPIKey, "info_hash ffffffffffffffffffffffffffffffffffffffff already inserted", http.StatusBadRequest},
		{"remove", "http://example.com:8080/api?action=remove_infohash&info_hash=ffffffffffffffffffffffffffffffffffffffff", testutils.DefaultAPIKey, "", http.StatusOK},
		{"missing action", "http://example.com:8080/api?info_hash=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", testutils.DefaultAPIKey, "", http.StatusBadRequest},
		{"bad infohash", "http://example.com:8080/api?action=insert_infohash&info_hash=a", testutils.DefaultAPIKey, "", http.StatusBadRequest},
	}

	handler := APIHandler(conf)

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
}
