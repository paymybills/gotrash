package store

import "time"

// Paste represents an ephemeral paste or file upload.
type Paste struct {
	ID          string    `json:"id"`
	Content     string    `json:"content,omitempty"`      // For text pastes
	IsFile      bool      `json:"is_file"`                // True if it is a file upload
	FileName    string    `json:"file_name,omitempty"`    // Original file name
	FileType    string    `json:"file_type,omitempty"`    // MIME type
	FileSize    int64     `json:"file_size,omitempty"`    // File size in bytes
	FilePath    string    `json:"-"`                      // Internal path on disk
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	BurnOnRead  bool      `json:"burn_on_read"`
	DeleteToken string    `json:"delete_token,omitempty"` // Key to manually delete the paste before expiry
}

// IsExpired returns true if the paste has passed its expiration time.
func (p *Paste) IsExpired() bool {
	if p.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(p.ExpiresAt)
}
