package search

import (
	"context"
	"strings"
	"sync"
)

type manager struct {
	mu       sync.Mutex
	sessions map[string]context.CancelFunc
}

// DefaultManager is the global search session manager.
var DefaultManager = &manager{
	sessions: make(map[string]context.CancelFunc),
}

// Response is the JSON response for a search request.
type Response struct {
	Results []Result `json:"results"`
}

// Result represents a single search result.
type Result struct {
	File    string `json:"file"`
	Lines   int    `json:"lines,omitempty"`    // total lines in file
	Size    int    `json:"size,omitempty"`     // file size in bytes
	Line    int    `json:"line,omitempty"`     // symbol start line
	EndLine int    `json:"end_line,omitempty"` // symbol end line (inclusive)
	Symbol  string `json:"symbol,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

// Search performs a search with optional session-based cancellation.
// details=true causes file-only results to also include line count and size.
// Symbol results always include end_line, lines, and size (file is already read for parsing).
func (m *manager) Search(httpCtx context.Context, sessionID, workDir, query string, details bool) *Response {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Also cancel when HTTP request ends (client disconnect).
	go func() {
		select {
		case <-httpCtx.Done():
			cancel()
		case <-ctx.Done():
		}
	}()

	if sessionID != "" {
		m.mu.Lock()
		if prev, ok := m.sessions[sessionID]; ok {
			prev() // Cancel the previous in-flight search for this session.
		}
		m.sessions[sessionID] = cancel
		m.mu.Unlock()

		defer func() {
			m.mu.Lock()
			delete(m.sessions, sessionID)
			m.mu.Unlock()
		}()
	}

	params := parseQuery(query)
	params.details = details
	results := walk(ctx, workDir, params)
	return &Response{Results: results}
}

type searchParams struct {
	filePattern   string
	symbolPattern string
	wantSymbols   bool // true if query contained ":"
	details       bool // include lines/size for file-only results
}

func parseQuery(query string) searchParams {
	if idx := strings.Index(query, ":"); idx >= 0 {
		return searchParams{
			filePattern:   query[:idx],
			symbolPattern: query[idx+1:],
			wantSymbols:   true,
		}
	}
	return searchParams{filePattern: query}
}
