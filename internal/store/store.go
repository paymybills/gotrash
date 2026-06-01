package store

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"sync"
	"time"
)

var (
	ErrNotFound     = errors.New("paste not found")
	ErrInvalidToken = errors.New("invalid delete token")
)

const base62Alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Store manages in-memory pastes and disk-based file uploads.
type Store struct {
	mu        sync.RWMutex
	pastes    map[string]*Paste
	uploadDir string
}

// NewStore initializes a new Store and ensures the upload directory exists.
func NewStore(uploadDir string) (*Store, error) {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	return &Store{
		pastes:    make(map[string]*Paste),
		uploadDir: uploadDir,
	}, nil
}

// Create generates a secure random ID and delete token, adds the paste to the store, and returns it.
func (s *Store) Create(content string, isFile bool, fileName string, fileType string, fileSize int64, filePath string, ttl time.Duration, burnOnRead bool) (*Paste, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate a unique ID (check collisions, though highly unlikely with 8 chars base62)
	var id string
	for {
		id = generateRandomString(8)
		if _, exists := s.pastes[id]; !exists {
			break
		}
	}

	deleteToken := generateRandomString(16)

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	paste := &Paste{
		ID:          id,
		Content:     content,
		IsFile:      isFile,
		FileName:    fileName,
		FileType:    fileType,
		FileSize:    fileSize,
		FilePath:    filePath,
		CreatedAt:   time.Now(),
		ExpiresAt:   expiresAt,
		BurnOnRead:  burnOnRead,
		DeleteToken: deleteToken,
	}

	s.pastes[id] = paste
	return paste, nil
}

// Get retrieves a paste by ID. It returns false if not found or expired.
func (s *Store) Get(id string) (*Paste, bool) {
	s.mu.RLock()
	paste, exists := s.pastes[id]
	s.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if paste.IsExpired() {
		// Asynchronously delete expired paste
		go s.Delete(id, "")
		return nil, false
	}

	return paste, true
}

// Delete removes a paste from the store and deletes its file on disk.
// If deleteToken is non-empty, it is checked against the paste's token.
func (s *Store) Delete(id string, deleteToken string) error {
	s.mu.Lock()
	paste, exists := s.pastes[id]
	if !exists {
		s.mu.Unlock()
		return ErrNotFound
	}

	// Validate token if one is provided
	if deleteToken != "" && paste.DeleteToken != deleteToken {
		s.mu.Unlock()
		return ErrInvalidToken
	}

	// Remove from map first
	delete(s.pastes, id)
	s.mu.Unlock()

	// Clean up file if this was a file upload
	if paste.IsFile && paste.FilePath != "" {
		if err := os.Remove(paste.FilePath); err != nil && !os.IsNotExist(err) {
			log.Printf("ERROR: failed to delete file %s from disk: %v", paste.FilePath, err)
		} else {
			log.Printf("DEBUG: successfully deleted file %s from disk", paste.FilePath)
		}
	}

	log.Printf("DEBUG: successfully removed paste %s from memory", id)
	return nil
}

// StartJanitor runs a background ticker to clean up expired items at regular intervals.
func (s *Store) StartJanitor(interval time.Duration, stopChan <-chan struct{}) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.cleanupExpired()
			case <-stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

// GetUploadDir returns the configured upload directory.
func (s *Store) GetUploadDir() string {
	return s.uploadDir
}

// cleanupExpired scans the store and removes all expired items.
func (s *Store) cleanupExpired() {
	s.mu.RLock()
	var expiredIDs []string
	for id, paste := range s.pastes {
		if paste.IsExpired() {
			expiredIDs = append(expiredIDs, id)
		}
	}
	s.mu.RUnlock()

	if len(expiredIDs) > 0 {
		log.Printf("DEBUG: Janitor found %d expired paste(s) to remove", len(expiredIDs))
		for _, id := range expiredIDs {
			// We pass empty token to bypass token check for system cleanup
			_ = s.Delete(id, "")
		}
	}
}

// generateRandomString returns a cryptographically secure random base62 string.
func generateRandomString(length int) string {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(base62Alphabet))))
		if err != nil {
			// Fallback in the extremely unlikely event of crypto/rand failure
			result[i] = base62Alphabet[0]
		} else {
			result[i] = base62Alphabet[num.Int64()]
		}
	}
	return string(result)
}
