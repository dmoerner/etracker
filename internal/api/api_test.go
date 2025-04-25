package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
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
	ctx := context.Background()
	conf := testutils.BuildTestConfig(ctx, nil, "")
	defer testutils.TeardownTest(ctx, conf)

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

	handler := PostInfohashHandler(ctx, conf)

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
	ctx := context.Background()
	conf := testutils.BuildTestConfig(ctx, nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(ctx, conf)

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

	handler := PostInfohashHandler(ctx, conf)

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
	ctx := context.Background()
	conf := testutils.BuildTestConfig(ctx, nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(ctx, conf)

	data := []APIRequest{
		// Inserting a duplicate key
		{"insert", "POST", "https://example.com:8080/api/infohash", []byte("ffffffffffffffffffff"), testutils.DefaultAPIKey, "success", http.StatusCreated},
		{"insert dupe", "POST", "https://example.com:8080/api/infohash", []byte("ffffffffffffffffffff"), testutils.DefaultAPIKey, "error: infohash already inserted", http.StatusBadRequest},
	}

	postHandler := PostInfohashHandler(ctx, conf)

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
	ctx := context.Background()
	conf := testutils.BuildTestConfig(ctx, nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(ctx, conf)

	data := []APIRequest{
		{"insert", "POST", "https://example.com:8080/api/infohash", []byte("fffffffffffffffffffff"), testutils.DefaultAPIKey, "error: did not receive valid infohash", http.StatusBadRequest},
	}

	postHandler := PostInfohashHandler(ctx, conf)

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
	ctx := context.Background()
	conf := testutils.BuildTestConfig(ctx, nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(ctx, conf)

	data := []APIRequest{
		{"insert", "POST", "https://example.com:8080/api/infohash", []byte("ffffffffffffffffffff"), testutils.DefaultAPIKey, "success", http.StatusCreated},
	}

	postHandler := PostInfohashHandler(ctx, conf)

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

	deleteHandler := DeleteInfohashHandler(ctx, conf)

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
	ctx := context.Background()
	conf := testutils.BuildTestConfig(ctx, handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(ctx, conf)

	request := testutils.CreateTestAnnounce(testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Event:       config.Completed,
		Left:        0,
	})
	w := httptest.NewRecorder()

	peerHandler := handler.PeerHandler(ctx, conf)
	peerHandler(w, request)

	request = httptest.NewRequest("GET", "http://example.com/frontendapi/infohashes", nil)
	w = httptest.NewRecorder()

	infohashesHandler := InfohashesHandler(ctx, conf)
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
	ctx := context.Background()
	conf := testutils.BuildTestConfig(ctx, handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(ctx, conf)

	request := testutils.CreateTestAnnounce(testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Event:       config.Completed,
		Left:        0,
	})
	w := httptest.NewRecorder()

	peerHandler := handler.PeerHandler(ctx, conf)
	peerHandler(w, request)

	request = httptest.NewRequest("GET", "http://example.com/frontendapi/stats", nil)
	w = httptest.NewRecorder()

	statsHandler := StatsHandler(ctx, conf)
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
	ctx := context.Background()
	conf := testutils.BuildTestConfig(ctx, handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(ctx, conf)

	request := httptest.NewRequest("GET", "http://example.com/frontendapi/generate", nil)
	w := httptest.NewRecorder()

	generateHandler := GenerateHandler(ctx, conf)
	generateHandler(w, request)

	body, _ := io.ReadAll(w.Result().Body)

	var received Key

	err := json.Unmarshal(body, &received)
	if err != nil {
		t.Errorf("error: did not receive key from generate endpoint: %v", err)
	}

	// Verify that the key was written to the db.
	var written bool
	err = conf.Dbpool.QueryRow(ctx, `
		SELECT EXISTS (SELECT FROM peers WHERE announce_key = $1)
		`,
		received.Announce_key).Scan(&written)
	if err != nil {
		t.Errorf("error: could not check database for written key: %v", err)
	}

	if !written {
		t.Errorf("key %s not written to database", received.Announce_key)
	}
}

// The TorrentFile POST and GET endpoints are tested together: First POST samples,
// then verify that you can GET them with the announce keys and private flag
// rewritten.
func TestPostGetTorrentFile(t *testing.T) {
	ctx := context.Background()
	conf := testutils.BuildTestConfig(ctx, nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(ctx, conf)

	postHandler := PostTorrentFileHandler(ctx, conf)
	getHandler := GetTorrentFileHandler(ctx, conf)

	// These info_hashes are hard-coded, by manually constructing a stripped
	// torrent file and extracting the info_hash.
	data := []struct {
		name             string
		post_file        string
		announce_key     string
		stored_info_hash string
		get_file         string
	}{
		{"single file", "./test_files/post/singlefile.txt.torrent", testutils.AnnounceKeys[1], "07d3b124456aea33187e832e4c3c046fd94dde9a", "./test_files/get/singlefile.txt.torrent"},
		{"multi file", "./test_files/post/multifile.torrent", testutils.AnnounceKeys[1], "d77f2817a93fe9e98eff809202fc898d4d812f11", "./test_files/get/multifile.torrent"},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			info_hash, err := hex.DecodeString(d.stored_info_hash)
			if err != nil {
				t.Fatalf("could not convert hardcoded hex infohash: %v", err)
			}

			// Test POST method.

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			filePart, err := writer.CreateFormFile("file", d.post_file)
			if err != nil {
				t.Fatalf("could not create multipart writer from file: %v", err)
			}

			f, err := os.Open(d.post_file)
			if err != nil {
				t.Fatalf("could not open file: %v", err)
			}

			_, err = io.Copy(filePart, f)
			if err != nil {
				t.Fatalf("could not copy file content: %v", err)
			}

			err = writer.Close()
			if err != nil {
				t.Fatalf("failed to close multipart writer: %v", err)
			}

			request := httptest.NewRequest(http.MethodPost, "https://example.com/api/torrentfile/", body)
			request.Header.Add("Authorization", testutils.DefaultAPIKey)
			request.Header.Add("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()

			postHandler(w, request)

			var added bool
			err = conf.Dbpool.QueryRow(ctx, `
				SELECT EXISTS (SELECT FROM infohashes WHERE info_hash = $1)
				`,
				info_hash).Scan(&added)
			if err != nil {
				t.Errorf("error: could not check database for added hash: %v", err)
			}

			if !added {
				t.Errorf("info_hash %s was not added to database", info_hash)
			}

			// Test GET method.

			request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("https://example.com/api/torrentfile?announce_key=%s&info_hash=%s", d.announce_key, d.stored_info_hash), nil)
			w = httptest.NewRecorder()

			getHandler(w, request)

			received_file, _ := io.ReadAll(w.Result().Body)

			expected, err := os.ReadFile(d.get_file)
			if err != nil {
				t.Fatalf("could not read torrent file: %v", err)
			}

			if !bytes.Equal(expected, received_file) {
				t.Errorf("Did not receive expected torrent file. Expected: %s, Received: %s", expected, received_file)
			}
		})
	}
}

// func TestGetTorrentFile(t *testing.T) {
// 	conf := testutils.BuildTestConfig(nil, testutils.DefaultAPIKey)
// 	defer testutils.TeardownTest(conf)
//
// 	handler := GetTorrentFileHandler(conf)
//
// 	// These info_hashes are hard-coded, by manually constructing a stripped
// 	// torrent file and extracting the info_hash.
// 	data := []struct {
// 		name         string
// 		announce_key string
// 		info_hash    string
// 		file         string
// 	}{
// 		{"single file", testutils.AnnounceKeys[1], "07d3b124456aea33187e832e4c3c046fd94dde9a", "./test_files/get/singlefile.txt.torrent"},
// 	}
//
// 	for _, d := range data {
// 		t.Run(d.name, func(t *testing.T) {
// 			request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("https://example.com/api/torrentfile?announce_key=%s&info_hash=%s", d.announce_key, d.info_hash), nil)
// 			w := httptest.NewRecorder()
//
// 			handler(w, request)
//
// 			body, _ := io.ReadAll(w.Result().Body)
//
// 			expected, err := os.ReadFile(d.file)
// 			if err != nil {
// 				t.Fatalf("could not read torrent file: %v", err)
// 			}
//
// 			if !bytes.Equal(expected, body) {
// 				t.Errorf("Did not receive expected torrent file. Expected: %s, Received: %s", expected, body)
// 			}
// 		})
// 	}
// }
