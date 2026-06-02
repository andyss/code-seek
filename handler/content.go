package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ContentRequest is the POST body for /content.
type ContentRequest struct {
	WorkDir string        `json:"work_dir"`
	Files   []FileRequest `json:"files"`
}

// FileRequest specifies a file and optional line ranges to return.
// If Ranges is empty the entire file is returned as a single segment.
type FileRequest struct {
	Path   string   `json:"path"`
	Ranges [][2]int `json:"ranges"` // [[startLine, endLine], ...], 1-based inclusive
}

// ContentResponse is the JSON response for /content.
type ContentResponse struct {
	Files []FileContent `json:"files"`
}

// FileContent holds metadata and content segments for one file.
type FileContent struct {
	Path       string        `json:"path"`
	TotalLines int           `json:"total_lines"`
	Size       int           `json:"size"`
	Segments   []LineSegment `json:"segments"`
	Error      string        `json:"error,omitempty"`
}

// LineSegment is a contiguous block of lines from a file.
type LineSegment struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
}

func Content(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(`{"error":"POST required"}`))
		return
	}

	var req ContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid JSON body"}`))
		return
	}

	resp := ContentResponse{Files: make([]FileContent, 0, len(req.Files))}
	for _, f := range req.Files {
		resp.Files = append(resp.Files, readFileContent(req.WorkDir, f))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func readFileContent(workDir string, req FileRequest) FileContent {
	fc := FileContent{Path: req.Path}

	absPath := req.Path
	if !filepath.IsAbs(absPath) {
		if workDir == "" {
			fc.Error = "relative path requires work_dir"
			return fc
		}
		absPath = filepath.Join(workDir, req.Path)
	}
	// Guard against directory traversal.
	absPath = filepath.Clean(absPath)
	if workDir != "" {
		cleanWork := filepath.Clean(workDir)
		if !strings.HasPrefix(absPath, cleanWork+string(filepath.Separator)) && absPath != cleanWork {
			fc.Error = "path outside work_dir"
			return fc
		}
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		fc.Error = err.Error()
		return fc
	}

	// Split into lines preserving original content.
	// strings.Split on "\n" gives an extra empty element for files ending in newline;
	// we keep that behaviour and let callers use 1-based line numbers naturally.
	lines := strings.Split(string(data), "\n")
	// Remove the trailing empty entry produced by a final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	fc.TotalLines = len(lines)
	fc.Size = len(data)

	if len(req.Ranges) == 0 {
		fc.Segments = []LineSegment{{
			StartLine: 1,
			EndLine:   len(lines),
			Content:   string(data),
		}}
		return fc
	}

	for _, r := range req.Ranges {
		start, end := r[0], r[1]
		if start < 1 {
			start = 1
		}
		if end > len(lines) {
			end = len(lines)
		}
		if start > end {
			continue
		}
		fc.Segments = append(fc.Segments, LineSegment{
			StartLine: start,
			EndLine:   end,
			Content:   strings.Join(lines[start-1:end], "\n"),
		})
	}

	return fc
}
