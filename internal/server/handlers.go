package server

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gotrash/internal/store"
	"gotrash/web"
)

// IndexData is passed to render the main template.
type IndexData struct {
	Paste         *store.Paste
	FormattedSize string
	Error         string
}

// handleIndex serves the home page to create a new share.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.renderErrorPage(w, "The page you are looking for does not exist.", http.StatusNotFound)
		return
	}
	s.renderTemplate(w, IndexData{})
}

// handleUpload handles POST /api/upload for both text pastes and file uploads.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Restrict uploads to 50MB max
		err := r.ParseMultipartForm(50 << 20)
		if err != nil {
			http.Error(w, "Upload size exceeds maximum limit of 50MB", http.StatusRequestEntityTooLarge)
			return
		}
	} else {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Failed to parse form parameters: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	ttlStr := r.FormValue("ttl")
	if ttlStr == "" {
		ttlStr = "1h" // Default to 1 hour
	}
	ttlDuration, err := parseDuration(ttlStr)
	if err != nil {
		http.Error(w, "Invalid expiration time format: "+err.Error(), http.StatusBadRequest)
		return
	}

	burnOnRead := r.FormValue("burn") == "true"

	// Check if a file was uploaded
	file, header, err := r.FormFile("file")
	
	var paste *store.Paste
	if err == nil {
		// File upload case
		defer file.Close()

		// Generate a secure unique local filename
		diskFilename := fmt.Sprintf("upload_%s", generateRandomHex(16))
		filePath := filepath.Join(s.store.GetUploadDir(), diskFilename)

		// Create file on disk
		dst, err := os.Create(filePath)
		if err != nil {
			log.Printf("ERROR: Failed to create file on disk: %v", err)
			http.Error(w, "Internal server disk error", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		// Stream content to file
		written, err := io.Copy(dst, file)
		if err != nil {
			log.Printf("ERROR: Failed to save file contents: %v", err)
			os.Remove(filePath) // Clean up partial file
			http.Error(w, "Failed to stream upload contents", http.StatusInternalServerError)
			return
		}

		// Save metadata to store
		paste, err = s.store.Create(
			"",                 // content
			true,               // isFile
			header.Filename,    // fileName
			header.Header.Get("Content-Type"), // fileType
			written,            // fileSize
			filePath,           // filePath
			ttlDuration,
			burnOnRead,
		)
		if err != nil {
			os.Remove(filePath) // Clean up file
			log.Printf("ERROR: Failed to save file metadata: %v", err)
			http.Error(w, "Internal storage allocation error", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Uploaded file '%s' (%d bytes) with ID %s", header.Filename, written, paste.ID)

	} else {
		// Text paste case
		content := r.FormValue("content")
		if content == "" {
			http.Error(w, "Paste content or file upload is required", http.StatusBadRequest)
			return
		}

		paste, err = s.store.Create(
			content,
			false,      // isFile
			"",         // fileName
			"text/plain; charset=utf-8", // fileType
			int64(len(content)), // fileSize
			"",         // filePath
			ttlDuration,
			burnOnRead,
		)
		if err != nil {
			log.Printf("ERROR: Failed to save paste: %v", err)
			http.Error(w, "Internal storage allocation error", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Created text paste with ID %s", paste.ID)
	}

	// Respond with metadata JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(paste)
}

// handleView renders the paste display page (GET /p/{id}).
func (s *Server) handleView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.renderErrorPage(w, "Invalid share ID.", http.StatusBadRequest)
		return
	}

	// Check if this is a manual DELETE request embedded in a normal route
	// (some environments don't support custom HTTP verbs natively, so we check query token)
	if r.Method == http.MethodDelete {
		s.handleDelete(w, r)
		return
	}

	paste, exists := s.store.Get(id)
	if !exists {
		s.renderErrorPage(w, "This link has expired, been burned, or never existed.", http.StatusNotFound)
		return
	}

	data := IndexData{
		Paste:         paste,
		FormattedSize: formatBytes(paste.FileSize),
	}

	s.renderTemplate(w, data)

	// If Burn On Read is active, delete the paste from memory and disk immediately after rendering
	if paste.BurnOnRead {
		log.Printf("INFO: Paste %s read. Triggering automatic self-destruct (Burn-on-Read).", id)
		_ = s.store.Delete(id, "")
	}
}

// handleDelete removes a paste immediately (DELETE /p/{id}?token=...).
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := r.URL.Query().Get("token")

	if id == "" || token == "" {
		http.Error(w, "ID and delete token are required", http.StatusBadRequest)
		return
	}

	err := s.store.Delete(id, token)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "Share not found or already expired", http.StatusNotFound)
		} else if errors.Is(err, store.ErrInvalidToken) {
			http.Error(w, "Invalid delete token provided", http.StatusForbidden)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	log.Printf("INFO: Paste %s deleted manually using deletion token", id)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Wiped successfully"))
}

// handleRaw outputs the raw text body or streams the raw binary file directly.
func (s *Server) handleRaw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	paste, exists := s.store.Get(id)
	if !exists {
		http.Error(w, "Paste not found or expired", http.StatusNotFound)
		return
	}

	if paste.IsFile {
		// Verify file still exists on disk
		if _, err := os.Stat(paste.FilePath); os.IsNotExist(err) {
			http.Error(w, "Associated file was missing from disk", http.StatusGone)
			// Cleanup storage metadata
			_ = s.store.Delete(id, "")
			return
		}

		w.Header().Set("Content-Type", paste.FileType)
		w.Header().Set("Content-Length", strconv.FormatInt(paste.FileSize, 10))
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", paste.FileName))

		// Stream file from disk
		file, err := os.Open(paste.FilePath)
		if err != nil {
			log.Printf("ERROR: Failed to open disk file for streaming: %v", err)
			http.Error(w, "Disk read failure", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		_, _ = io.Copy(w, file)
	} else {
		// Output raw text paste
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(paste.Content)))
		_, _ = w.Write([]byte(paste.Content))
	}

	// Burn on read deletion trigger
	if paste.BurnOnRead {
		log.Printf("INFO: Raw stream for %s accessed. Triggering automatic self-destruct (Burn-on-Read).", id)
		_ = s.store.Delete(id, "")
	}
}

// renderTemplate is a helper to compile and write the index.html template.
func (s *Server) renderTemplate(w http.ResponseWriter, data IndexData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	tmpl, err := template.ParseFS(web.Assets, "templates/index.html")
	if err != nil {
		log.Printf("ERROR: Template parsing failed: %v", err)
		http.Error(w, "Internal server template error", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Printf("ERROR: Template execution failed: %v", err)
	}
}

// renderErrorPage is a helper to serve standard glassmorphic error views.
func (s *Server) renderErrorPage(w http.ResponseWriter, errMsg string, code int) {
	w.WriteHeader(code)
	s.renderTemplate(w, IndexData{Error: errMsg})
}

// parseDuration decodes custom durations including 'd' (days).
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 1 * time.Hour, nil
	}
	
	// Support 'd' mapping (time.ParseDuration doesn't natively support days)
	lastChar := s[len(s)-1]
	if lastChar == 'd' {
		daysStr := s[:len(s)-1]
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, fmt.Errorf("invalid day format: %w", err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)
}

// formatBytes outputs human-readable binary sizes.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGT"[exp])
}

// generateRandomHex makes a secure hex token for disk file separation.
func generateRandomHex(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback timestamp
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", bytes)
}
