package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreCreateAndGet(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gotrash_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// 1. Test Text Paste creation
	paste, err := s.Create("hello world", false, "", "text/plain", 11, "", 1*time.Hour, false)
	if err != nil {
		t.Fatalf("failed to create paste: %v", err)
	}

	if len(paste.ID) != 8 {
		t.Errorf("expected ID length 8, got %d", len(paste.ID))
	}
	if len(paste.DeleteToken) != 16 {
		t.Errorf("expected Token length 16, got %d", len(paste.DeleteToken))
	}

	// 2. Test Get
	fetched, exists := s.Get(paste.ID)
	if !exists {
		t.Fatalf("expected paste %s to exist", paste.ID)
	}
	if fetched.Content != "hello world" {
		t.Errorf("expected content 'hello world', got '%s'", fetched.Content)
	}
	if fetched.IsFile {
		t.Errorf("expected IsFile to be false")
	}
}

func TestStoreExpiration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gotrash_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create with very short TTL (10 milliseconds)
	paste, err := s.Create("expired soon", false, "", "text/plain", 12, "", 10*time.Millisecond, false)
	if err != nil {
		t.Fatalf("failed to create paste: %v", err)
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Fetch should return false
	_, exists := s.Get(paste.ID)
	if exists {
		t.Errorf("expected paste %s to be expired and unavailable", paste.ID)
	}
}

func TestStoreDeleteToken(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gotrash_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	paste, err := s.Create("secret content", false, "", "text/plain", 14, "", 1*time.Hour, false)
	if err != nil {
		t.Fatalf("failed to create paste: %v", err)
	}

	// Try deleting with wrong token
	err = s.Delete(paste.ID, "wrong_token")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}

	// Verify paste still exists
	_, exists := s.Get(paste.ID)
	if !exists {
		t.Errorf("paste should still exist after failed deletion")
	}

	// Delete with correct token
	err = s.Delete(paste.ID, paste.DeleteToken)
	if err != nil {
		t.Errorf("expected successful deletion, got error: %v", err)
	}

	// Verify paste is gone
	_, exists = s.Get(paste.ID)
	if exists {
		t.Errorf("paste should be deleted from memory")
	}
}

func TestStoreFileDeletion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gotrash_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Create a dummy file on disk
	fileName := "test_upload.txt"
	filePath := filepath.Join(tempDir, "uploaded_file_abc")
	err = os.WriteFile(filePath, []byte("some file bytes"), 0644)
	if err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}

	// Register file paste
	paste, err := s.Create("", true, fileName, "text/plain", 15, filePath, 1*time.Hour, false)
	if err != nil {
		t.Fatalf("failed to create file paste: %v", err)
	}

	// Check file exists before delete
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("expected physical file to exist on disk")
	}

	// Perform delete
	err = s.Delete(paste.ID, paste.DeleteToken)
	if err != nil {
		t.Fatalf("failed to delete paste: %v", err)
	}

	// Verify physical file is shredded/deleted from disk
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expected physical file %s to be deleted from disk upon paste destruction", filePath)
	}
}
