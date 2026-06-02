package handler

import (
	"encoding/json"
	"net/http"

	"github.com/andyss/code-seek/search"
)

func Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	workDir := q.Get("work_dir")
	query := q.Get("query")
	sessionID := q.Get("session_id")
	details := q.Get("details") == "true" || q.Get("details") == "1"

	if workDir == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"work_dir is required"}`))
		return
	}

	resp := search.DefaultManager.Search(r.Context(), sessionID, workDir, query, details)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
