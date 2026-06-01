package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gotrash/internal/store"
)

func TestHandlers(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gotrash_server_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := store.NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	srv, err := NewServer(":8080", db)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 1. Test GET / renders index template
	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected GET / to return 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "<title>gotrash") {
		t.Errorf("expected home page to contain gotrash title")
	}

	// 2. Test POST /api/upload (Text Paste)
	form := url.Values{}
	form.Add("content", "lorem ipsum dolor sit amet")
	form.Add("ttl", "1h")
	form.Add("burn", "false")

	req, _ = http.NewRequest("POST", "/api/upload", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected POST /api/upload to return 201, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var res map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &res)
	if err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	pasteID, exists := res["id"].(string)
	if !exists || len(pasteID) != 8 {
		t.Errorf("expected valid paste ID in response, got: %v", res["id"])
	}

	deleteToken, exists := res["delete_token"].(string)
	if !exists || len(deleteToken) != 16 {
		t.Errorf("expected valid delete_token in response, got: %v", res["delete_token"])
	}

	// 3. Test GET /p/{id} (View Paste)
	// We must register path patterns properly. In httptest, srv.router matches Go 1.22 routing paths.
	req, _ = http.NewRequest("GET", "/p/"+pasteID, nil)
	rr = httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected GET /p/%s to return 200, got %d", pasteID, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "lorem ipsum dolor sit amet") {
		t.Errorf("expected paste view to contain content 'lorem ipsum dolor sit amet'")
	}

	// 4. Test GET /raw/{id} (Raw Content stream)
	req, _ = http.NewRequest("GET", "/raw/"+pasteID, nil)
	rr = httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected GET /raw/%s to return 200, got %d", pasteID, rr.Code)
	}
	if rr.Body.String() != "lorem ipsum dolor sit amet" {
		t.Errorf("expected raw stream to equal 'lorem ipsum dolor sit amet', got '%s'", rr.Body.String())
	}

	// 5. Test manual delete (DELETE /p/{id}?token=...)
	req, _ = http.NewRequest("DELETE", "/p/"+pasteID+"?token="+deleteToken, nil)
	rr = httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected DELETE /p/%s to return 200, got %d", pasteID, rr.Code)
	}

	// Verify paste is gone
	_, existsInDb := db.Get(pasteID)
	if existsInDb {
		t.Errorf("expected paste to be deleted from db")
	}
}

func TestBurnOnRead(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gotrash_server_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := store.NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	srv, err := NewServer(":8080", db)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Create a Burn-On-Read paste
	paste, err := db.Create("burn after reading", false, "", "text/plain", 18, "", 1*time.Hour, true)
	if err != nil {
		t.Fatalf("failed to create burn paste: %v", err)
	}

	// Request paste view once
	req, _ := http.NewRequest("GET", "/p/"+paste.ID, nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("failed to read paste initially: %d", rr.Code)
	}

	// Second request should return 404 (burned/expired)
	req, _ = http.NewRequest("GET", "/p/"+paste.ID, nil)
	rr = httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected second read to return 404, got %d", rr.Code)
	}
}
