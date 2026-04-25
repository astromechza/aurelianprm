package web

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// handleAPIBackup creates a consistent snapshot of the SQLite database via
// VACUUM INTO and streams it as a downloadable file. The snapshot is
// transactionally consistent regardless of ongoing writes.
func (s *Server) handleAPIBackup(w http.ResponseWriter, r *http.Request) {
	filename := "aurelianprm-" + time.Now().UTC().Format("20060102T150405Z") + ".db"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	if err := s.dal.Backup(r.Context(), w); err != nil {
		// Headers may already be partially written; log the error.
		log.Printf("backup error: %v", err)
		// Attempt to surface the error to the client if headers haven't flushed.
		s.serverError(w, r, fmt.Errorf("backup: %w", err))
	}
}
