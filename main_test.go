// Test basic requests
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicRequests(t *testing.T) {
	router := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("GET /:\tUnexpected status code: %d, want: %d.", w.Code, 200)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/play?i=org.mozilla.firefox&l=Android&m=$version", nil)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("GET /play:\tUnexpected status code: %d, want: %d.", w.Code, 200)
	}
	if !json.Valid(w.Body.Bytes()) {
		t.Errorf("GET /play:\tResponse body from is not valid JSON: %s", w.Body.String())
	}
}
