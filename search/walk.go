package search

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxResults = 500

var skipDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	".git":         true,
	".svn":         true,
	"__pycache__":  true,
	"dist":         true,
	"build":        true,
	"target":       true,
	".build":       true,
	".gradle":      true,
}

type scoredResult struct {
	result Result
	score  int
}

func walk(ctx context.Context, workDir string, params searchParams) []Result {
	var scored []scoredResult

	filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return filepath.SkipAll
		default:
		}

		if len(scored) >= maxResults {
			return filepath.SkipAll
		}

		if d.IsDir() {
			name := d.Name()
			if skipDirs[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(workDir, path)
		if err != nil {
			relPath = path
		}
		relPath = filepath.ToSlash(relPath)

		ok, fileScore := fuzzyScore(relPath, params.filePattern)
		if !ok {
			return nil
		}

		if params.wantSymbols {
			syms := extractSymbols(ctx, path, relPath, params.symbolPattern, fileScore)
			scored = append(scored, syms...)
		} else {
			r := Result{File: relPath}
			if params.details {
				if data, err := os.ReadFile(path); err == nil {
					r.Lines = countLines(data)
					r.Size = len(data)
				}
			}
			scored = append(scored, scoredResult{result: r, score: fileScore})
		}

		return nil
	})

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	results := make([]Result, len(scored))
	for i, s := range scored {
		results[i] = s.result
	}
	return results
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte("\n"))
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}

// fuzzyScore returns whether pattern matches s (all chars in order, case-insensitive)
// and a score reflecting match quality. Higher is better.
//
// Scoring:
//   - +10 for each char matched at a word boundary (after /, _, -, . or at position 0)
//   - +consecutive*5 bonus that grows with consecutive run length
//   - -len(s)/5 length penalty (shorter paths rank higher for equal pattern density)
func fuzzyScore(s, pattern string) (matched bool, score int) {
	if pattern == "" {
		return true, 0
	}

	sLow := strings.ToLower(s)
	patLow := strings.ToLower(pattern)

	pi := 0
	consecutive := 0

	for i := 0; i < len(sLow) && pi < len(patLow); i++ {
		if sLow[i] == patLow[pi] {
			if i == 0 || isWordBoundary(sLow[i-1]) {
				score += 10
			}
			consecutive++
			if consecutive > 1 {
				score += consecutive * 5
			}
			pi++
		} else {
			consecutive = 0
		}
	}

	if pi < len(patLow) {
		return false, 0
	}

	score -= len(s) / 5
	return true, score
}

func isWordBoundary(c byte) bool {
	return c == '/' || c == '\\' || c == '_' || c == '-' || c == '.'
}

// fuzzyMatch is a convenience wrapper for callers that only need pass/fail.
func fuzzyMatch(s, pattern string) bool {
	ok, _ := fuzzyScore(s, pattern)
	return ok
}
