package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/handler"
	"github.com/dmoerner/etracker/internal/testutils"
	"github.com/google/go-cmp/cmp"
)

type APIRequest struct {
	name          string
	method        string
	request       string
	info_hash     []byte
	authorization string
	expectedbody  string
	expectedcode  int
}

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
		{"normally good api key", "https://example.com:8080/api/infohash", testutils.DefaultAPIKey, http.StatusForbidden},
		{"bad api key", "https://example.com:8080/api/infohash", "badapikey", http.StatusForbidden},
		{"no api key", "https://example.com:8080/api/infohash", "", http.StatusForbidden},
	}

	handler := PostInfohashHandler(conf)

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", d.request, nil)
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
		{"good api key", "https://example.com:8080/api/infohash", testutils.DefaultAPIKey, http.StatusBadRequest},
		{"bad api key", "https://example.com:8080/api/infohash", "badapikey", http.StatusForbidden},
		{"no api key", "https://example.com:8080/api/infohash", "", http.StatusBadRequest},
	}

	handler := PostInfohashHandler(conf)

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", d.request, nil)
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

func TestInsertDupeInfohash(t *testing.T) {
	conf := testutils.BuildTestConfig(nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	data := []APIRequest{
		// Inserting a duplicate key
		{"insert", "POST", "https://example.com:8080/api/infohash", []byte("ffffffffffffffffffff"), testutils.DefaultAPIKey, "success", http.StatusCreated},
		{"insert dupe", "POST", "https://example.com:8080/api/infohash", []byte("ffffffffffffffffffff"), testutils.DefaultAPIKey, "error: infohash already inserted", http.StatusBadRequest},
	}

	postHandler := PostInfohashHandler(conf)

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			body, err := json.Marshal(InfohashPost{d.info_hash, d.name})
			if err != nil {
				t.Errorf("error marshaling dummy request body: %v", err)
			}
			req := httptest.NewRequest(d.method, d.request, bytes.NewReader(body))
			if d.authorization != "" {
				req.Header.Add("Authorization", d.authorization)
			}
			w := httptest.NewRecorder()

			postHandler(w, req)
			resp := w.Result()
			if resp.StatusCode != d.expectedcode {
				t.Errorf("expected %d, got %d", d.expectedcode, resp.StatusCode)
			}

			expectedBody, err := json.Marshal(MessageJSON{d.expectedbody})
			if err != nil {
				t.Errorf("error marshaling expected response body: %v", err)
			}
			receivedBody, _ := io.ReadAll(resp.Body)
			if string(receivedBody) != string(expectedBody) {
				t.Errorf("expected %s, got %s", expectedBody, receivedBody)
			}
		})
	}
}

func TestInsertBadInfohash(t *testing.T) {
	conf := testutils.BuildTestConfig(nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	data := []APIRequest{
		{"insert", "POST", "https://example.com:8080/api/infohash", []byte("fffffffffffffffffffff"), testutils.DefaultAPIKey, "error: did not receive valid infohash", http.StatusBadRequest},
	}

	postHandler := PostInfohashHandler(conf)

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			body, err := json.Marshal(InfohashPost{d.info_hash, d.name})
			if err != nil {
				t.Errorf("error marshaling dummy request body: %v", err)
			}
			req := httptest.NewRequest(d.method, d.request, bytes.NewReader(body))
			if d.authorization != "" {
				req.Header.Add("Authorization", d.authorization)
			}
			w := httptest.NewRecorder()

			postHandler(w, req)
			resp := w.Result()
			if resp.StatusCode != d.expectedcode {
				t.Errorf("expected %d, got %d", d.expectedcode, resp.StatusCode)
			}

			expectedBody, err := json.Marshal(MessageJSON{d.expectedbody})
			if err != nil {
				t.Errorf("error marshaling expected response body: %v", err)
			}
			receivedBody, _ := io.ReadAll(resp.Body)
			if string(receivedBody) != string(expectedBody) {
				t.Errorf("expected %s, got %s", expectedBody, receivedBody)
			}
		})
	}
}

func TestInsertRemoveInfohash(t *testing.T) {
	conf := testutils.BuildTestConfig(nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	data := []APIRequest{
		{"insert", "POST", "https://example.com:8080/api/infohash", []byte("ffffffffffffffffffff"), testutils.DefaultAPIKey, "success", http.StatusCreated},
	}

	postHandler := PostInfohashHandler(conf)

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			body, err := json.Marshal(InfohashPost{d.info_hash, d.name})
			if err != nil {
				t.Errorf("error marshaling dummy request body: %v", err)
			}
			req := httptest.NewRequest(d.method, d.request, bytes.NewReader(body))
			if d.authorization != "" {
				req.Header.Add("Authorization", d.authorization)
			}
			w := httptest.NewRecorder()

			postHandler(w, req)
			resp := w.Result()
			if resp.StatusCode != d.expectedcode {
				t.Errorf("expected %d, got %d", d.expectedcode, resp.StatusCode)
			}

			expectedBody, err := json.Marshal(MessageJSON{d.expectedbody})
			if err != nil {
				t.Errorf("error marshaling expected response body: %v", err)
			}
			receivedBody, _ := io.ReadAll(resp.Body)
			if string(receivedBody) != string(expectedBody) {
				t.Errorf("expected %s, got %s", expectedBody, receivedBody)
			}
		})
	}

	data = []APIRequest{
		{"delete", "DELETE", "https://example.com:8080/api/infohash", []byte("ffffffffffffffffffff"), testutils.DefaultAPIKey, "success", http.StatusOK},
	}

	deleteHandler := DeleteInfohashHandler(conf)

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			body, err := json.Marshal(Infohash{d.info_hash})
			if err != nil {
				t.Errorf("error marshaling dummy request body: %v", err)
			}
			req := httptest.NewRequest(d.method, d.request, bytes.NewReader(body))
			if d.authorization != "" {
				req.Header.Add("Authorization", d.authorization)
			}
			w := httptest.NewRecorder()

			deleteHandler(w, req)
			resp := w.Result()
			if resp.StatusCode != d.expectedcode {
				t.Errorf("expected %d, got %d", d.expectedcode, resp.StatusCode)
			}

			expectedBody, err := json.Marshal(MessageJSON{d.expectedbody})
			if err != nil {
				t.Errorf("error marshaling expected response body: %v", err)
			}
			receivedBody, _ := io.ReadAll(resp.Body)
			if string(receivedBody) != string(expectedBody) {
				t.Errorf("expected %s, got %s", expectedBody, receivedBody)
			}
		})
	}
}

func TestInfohashes(t *testing.T) {
	conf := testutils.BuildTestConfig(handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	request := httptest.NewRequest("GET", testutils.FormatRequest(testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Event:       config.Completed,
		Left:        0,
	}), nil)
	w := httptest.NewRecorder()

	peerHandler := handler.PeerHandler(conf)
	peerHandler(w, request)

	request = httptest.NewRequest("GET", "http://example.com/frontendapi/infohashes", nil)
	w = httptest.NewRecorder()

	infohashesHandler := InfohashesHandler(conf)
	infohashesHandler(w, request)

	body, _ := io.ReadAll(w.Result().Body)

	expected := []InfohashStats{
		{
			Name:       testutils.AllowedInfoHashes["a"],
			Downloaded: 1,
			Seeders:    1,
			Leechers:   0,
			Info_hash:  []byte(testutils.AllowedInfoHashes["a"]),
		},
		{
			Name:       testutils.AllowedInfoHashes["b"],
			Downloaded: 0,
			Seeders:    0,
			Leechers:   0,
			Info_hash:  []byte(testutils.AllowedInfoHashes["b"]),
		},
		{
			Name:       testutils.AllowedInfoHashes["c"],
			Downloaded: 0,
			Seeders:    0,
			Leechers:   0,
			Info_hash:  []byte(testutils.AllowedInfoHashes["c"]),
		},
		{
			Name:       testutils.AllowedInfoHashes["d"],
			Downloaded: 0,
			Seeders:    0,
			Leechers:   0,
			Info_hash:  []byte(testutils.AllowedInfoHashes["d"]),
		},
	}

	var received []InfohashStats

	err := json.Unmarshal(body, &received)
	if err != nil {
		t.Errorf("error unmarshalling json response: %v", err)
	}

	// Use cmp.Diff for deep comparison of slices.
	if cmp.Diff(expected, received) != "" {
		t.Errorf("error in infohashes json, expected %v, got %v", expected, received)
	}
}

func TestStats(t *testing.T) {
	conf := testutils.BuildTestConfig(handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	request := httptest.NewRequest("GET", testutils.FormatRequest(testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Event:       config.Completed,
		Left:        0,
	}), nil)
	w := httptest.NewRecorder()

	peerHandler := handler.PeerHandler(conf)
	peerHandler(w, request)

	request = httptest.NewRequest("GET", "http://example.com/frontendapi/stats", nil)
	w = httptest.NewRecorder()

	statsHandler := StatsHandler(conf)
	statsHandler(w, request)

	body, _ := io.ReadAll(w.Result().Body)

	expected := GlobalStats{
		Hashcount: len(testutils.AllowedInfoHashes),
		Seeders:   1,
		Leechers:  0,
	}

	var received GlobalStats

	err := json.Unmarshal(body, &received)
	if err != nil {
		t.Errorf("error unmarshalling json response: %v", err)
	}

	if received != expected {
		t.Errorf("error in stats json, expected %v, got %v", expected, received)
	}
}

func TestGenerate(t *testing.T) {
	conf := testutils.BuildTestConfig(handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	request := httptest.NewRequest("GET", "http://example.com/frontendapi/generate", nil)
	w := httptest.NewRecorder()

	generateHandler := GenerateHandler(conf)
	generateHandler(w, request)

	body, _ := io.ReadAll(w.Result().Body)

	var received Key

	err := json.Unmarshal(body, &received)
	if err != nil {
		t.Errorf("error: did not receive key from generate endpoint: %v", err)
	}

	// Verify that the key was written to the db.
	var written bool
	err = conf.Dbpool.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT FROM peerids WHERE announce_key = $1)
		`,
		received.Announce_key).Scan(&written)
	if err != nil {
		t.Errorf("error: could not check database for written key: %v", err)
	}

	if !written {
		t.Errorf("key %s not written to database", received.Announce_key)
	}
}
