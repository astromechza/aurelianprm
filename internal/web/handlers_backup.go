package web

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// handleBackup creates a consistent snapshot of the SQLite database via
// VACUUM INTO and streams it as a downloadable file. The snapshot is
// transactionally consistent regardless of ongoing writes.
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Reserve a unique path in the system temp dir.  We delete the empty
	// placeholder immediately so that VACUUM INTO (which requires the target
	// not to exist) can write to that exact path.
	f, err := os.CreateTemp("", "aurelianprm-backup-*.db")
	if err != nil {
		s.serverError(w, r, fmt.Errorf("create temp file: %w", err))
		return
	}
	tmpPath := f.Name()
	f.Close()
	os.Remove(tmpPath)       //nolint:errcheck
	defer os.Remove(tmpPath) //nolint:errcheck

	if err := s.dal.Backup(ctx, tmpPath); err != nil {
		s.serverError(w, r, fmt.Errorf("backup database: %w", err))
		return
	}

	backupFile, err := os.Open(tmpPath)
	if err != nil {
		s.serverError(w, r, fmt.Errorf("open backup file: %w", err))
		return
	}
	defer backupFile.Close()

	info, err := backupFile.Stat()
	if err != nil {
		s.serverError(w, r, fmt.Errorf("stat backup file: %w", err))
		return
	}

	filename := "aurelianprm-" + time.Now().UTC().Format("20060102T150405Z") + ".db"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))

	if _, err := io.Copy(w, backupFile); err != nil {
		log.Printf("backup stream error: %v", err)
	}
}
