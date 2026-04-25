package web

import (
	"bytes"
	"net/http"
	"strconv"
	"time"
)

// handleAPIBackup creates a consistent snapshot of the SQLite database via
// VACUUM INTO and streams it as a downloadable file. The snapshot is
// transactionally consistent regardless of ongoing writes. The backup is
// fully buffered before any response headers are written so that errors can
// still be reported as proper HTTP 500 responses.
func (s *Server) handleAPIBackup(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	if err := s.dal.Backup(r.Context(), &buf); err != nil {
		s.serverError(w, r, err)
		return
	}

	filename := "aurelianprm-" + time.Now().UTC().Format("20060102T150405Z") + ".db"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, _ = buf.WriteTo(w)
}
