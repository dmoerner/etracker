package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmoerner/etracker/internal/testutils"
)

func TestTrackedInfohashes(t *testing.T) {
	conf := testutils.BuildTestConfig(nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	handler := WebHandler(conf)

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	defer w.Result().Body.Close()

	data, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatalf("error reading httptest recorder body: %v", err)
	}

	expected := fmt.Sprintf("Tracked infohashes: %d", len(testutils.AllowedInfoHashes))

	if !strings.Contains(string(data), expected) {
		t.Errorf("expected \"%s\", got something else", expected)
	}
}

func TestNotFound(t *testing.T) {
	conf := testutils.BuildTestConfig(nil, "")
	defer testutils.TeardownTest(conf)

	handler := WebHandler(conf)

	req := httptest.NewRequest("GET", "http://example.com/doesnotexist", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected %d, got %d", http.StatusNotFound, w.Result().StatusCode)
	}
}
