package search

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	langC "github.com/smacker/go-tree-sitter/c"
	langCPP "github.com/smacker/go-tree-sitter/cpp"
	langGo "github.com/smacker/go-tree-sitter/golang"
	langJava "github.com/smacker/go-tree-sitter/java"
	langJS "github.com/smacker/go-tree-sitter/javascript"
	langPython "github.com/smacker/go-tree-sitter/python"
	langRuby "github.com/smacker/go-tree-sitter/ruby"
	langRust "github.com/smacker/go-tree-sitter/rust"
	langTS "github.com/smacker/go-tree-sitter/typescript/typescript"
)

type langDef struct {
	exts  []string
	lang  func() *sitter.Language
	query string
}

// Each query uses two capture names per pattern:
//   @name  — the identifier node (gives symbol name and start line)
//   @<kind> — the full declaration node (gives end line and symbol kind)
//
// captureKind maps kind capture names → display kind strings.
// Swift is not included — it is not shipped in smacker/go-tree-sitter.
var langDefs = []langDef{
	{
		exts: []string{".go"},
		lang: langGo.GetLanguage,
		query: `
(function_declaration name: (identifier) @name) @func
(method_declaration name: (field_identifier) @name) @method
(type_spec name: (type_identifier) @name) @type
`,
	},
	{
		exts: []string{".py"},
		lang: langPython.GetLanguage,
		query: `
(function_definition name: (identifier) @name) @func
(class_definition name: (identifier) @name) @class
`,
	},
	{
		exts: []string{".js", ".jsx", ".mjs"},
		lang: langJS.GetLanguage,
		query: `
(function_declaration name: (identifier) @name) @func
(class_declaration name: (identifier) @name) @class
(method_definition name: (property_identifier) @name) @method
`,
	},
	{
		exts: []string{".ts", ".tsx"},
		lang: langTS.GetLanguage,
		query: `
(function_declaration name: (identifier) @name) @func
(class_declaration name: (identifier) @name) @class
(method_definition name: (property_identifier) @name) @method
(interface_declaration name: (type_identifier) @name) @type
`,
	},
	{
		exts: []string{".rb"},
		lang: langRuby.GetLanguage,
		query: `
(method name: (identifier) @name) @func
(singleton_method name: (identifier) @name) @func
(class name: (constant) @name) @class
(module name: (constant) @name) @module
`,
	},
	{
		exts: []string{".java"},
		lang: langJava.GetLanguage,
		query: `
(method_declaration name: (identifier) @name) @method
(class_declaration name: (identifier) @name) @class
(interface_declaration name: (identifier) @name) @type
(enum_declaration name: (identifier) @name) @type
`,
	},
	{
		exts: []string{".rs"},
		lang: langRust.GetLanguage,
		query: `
(function_item name: (identifier) @name) @func
(struct_item name: (type_identifier) @name) @type
(enum_item name: (type_identifier) @name) @type
(trait_item name: (type_identifier) @name) @type
`,
	},
	{
		exts: []string{".c", ".h"},
		lang: langC.GetLanguage,
		query: `
(function_definition declarator: (function_declarator declarator: (identifier) @name)) @func
`,
	},
	{
		exts: []string{".cc", ".cpp", ".cxx", ".hpp"},
		lang: langCPP.GetLanguage,
		query: `
(function_definition declarator: (function_declarator declarator: (identifier) @name)) @func
(class_specifier name: (type_identifier) @name) @class
`,
	},
}

// captureKind maps the outer capture name to the human-readable symbol kind.
var captureKind = map[string]string{
	"func":   "function",
	"method": "method",
	"class":  "class",
	"type":   "type",
	"module": "module",
}

var extToLang map[string]*langDef

func init() {
	extToLang = make(map[string]*langDef)
	for i := range langDefs {
		for _, ext := range langDefs[i].exts {
			extToLang[ext] = &langDefs[i]
		}
	}
}

func extractSymbols(ctx context.Context, path, relPath, symbolPattern string, baseScore int) []scoredResult {
	ext := strings.ToLower(filepath.Ext(path))
	ld, ok := extToLang[ext]
	if !ok {
		return nil
	}

	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return nil
	default:
	}

	lang := ld.lang()
	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(ctx, nil, src)
	if err != nil || tree == nil {
		return nil
	}
	defer tree.Close()

	q, err := sitter.NewQuery([]byte(ld.query), lang)
	if err != nil {
		return nil
	}

	// File-level metadata is free since we've already read the file.
	fileLines := countLines(src)
	fileSize := len(src)

	qc := sitter.NewQueryCursor()
	qc.Exec(q, tree.RootNode())

	seen := make(map[string]bool)
	var results []scoredResult

	for {
		select {
		case <-ctx.Done():
			return results
		default:
		}

		match, ok := qc.NextMatch()
		if !ok {
			break
		}

		// Each match has two captures: @name (identifier) and @<kind> (full declaration).
		var (
			symbolName string
			startLine  int
			endLine    int
			kind       string
		)

		for _, cap := range match.Captures {
			capName := q.CaptureNameForId(cap.Index)
			if capName == "name" {
				symbolName = cap.Node.Content(src)
				startLine = int(cap.Node.StartPoint().Row) + 1
			} else if k, ok := captureKind[capName]; ok {
				kind = k
				endLine = int(cap.Node.EndPoint().Row) + 1
			}
		}

		if symbolName == "" {
			continue
		}

		symOK, symScore := fuzzyScore(symbolName, symbolPattern)
		if !symOK {
			continue
		}

		key := symbolName + ":" + strconv.Itoa(startLine)
		if seen[key] {
			continue
		}
		seen[key] = true

		results = append(results, scoredResult{
			result: Result{
				File:    relPath,
				Lines:   fileLines,
				Size:    fileSize,
				Line:    startLine,
				EndLine: endLine,
				Symbol:  symbolName,
				Kind:    kind,
			},
			score: baseScore + symScore,
		})
	}

	return results
}
