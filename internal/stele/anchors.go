package stele

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var (
	anchorPattern     = regexp.MustCompile(`@(implements|verifies)\s+((?:req|scn)\.[a-z0-9]+\.[a-f0-9]{12})`)
	jsTestPattern     = regexp.MustCompile(`^(?:test|it)\(\s*["'\x60]([^"'\x60]+)["'\x60]`)
	jsFunctionPattern = regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)`)
	jsClassPattern    = regexp.MustCompile(`^(?:export\s+)?class\s+([A-Za-z_$][\w$]*)`)
	jsMethodPattern   = regexp.MustCompile(`^(?:async\s+)?#?([A-Za-z_$][\w$]*)\s*\(`)
	goFunctionPattern = regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	goTypePattern     = regexp.MustCompile(`^type\s+([A-Za-z_][A-Za-z0-9_]*)\s+`)
)

func supportedSource(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".go" || ext == ".js" || ext == ".mjs" || ext == ".ts" || ext == ".tsx" || ext == ".jsx"
}

func isTestFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(name, "_test.go") || strings.Contains(name, ".test.") || strings.Contains(name, ".spec.")
}

func ScanAnchors(root string) ([]Anchor, error) {
	codeSet := make(map[string]bool)
	testSet := make(map[string]bool)
	for _, directory := range []string{"bin", "cmd", "internal", "src", "public", "tools", "scripts", "tests"} {
		for _, file := range walkFiles(filepath.Join(root, directory), supportedSource) {
			if isTestFile(file) {
				testSet[file] = true
			} else if directory != "tests" {
				codeSet[file] = true
			}
		}
	}
	for _, name := range []string{"server.mjs", "server.js"} {
		file := filepath.Join(root, name)
		if fileExists(file) {
			codeSet[file] = true
		}
	}
	anchors := make([]Anchor, 0)
	for _, group := range []struct {
		kind  string
		files map[string]bool
	}{{"code", codeSet}, {"test", testSet}} {
		files := make([]string, 0, len(group.files))
		for file := range group.files {
			files = append(files, file)
		}
		sort.Strings(files)
		for _, file := range files {
			content, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
			for lineIndex, line := range lines {
				matches := anchorPattern.FindAllStringSubmatch(line, -1)
				for _, match := range matches {
					selector, declarationLine := adjacentDeclaration(lines, lineIndex, group.kind, filepath.Ext(file))
					relative, err := relativePath(root, file)
					if err != nil {
						return nil, err
					}
					anchors = append(anchors, Anchor{ID: match[2], Annotation: match[1], Kind: group.kind, Path: filepath.ToSlash(relative), Line: lineIndex + 1, Selector: selector, DeclarationLine: declarationLine})
				}
			}
		}
	}
	sort.Slice(anchors, func(i, j int) bool {
		return fmt.Sprintf("%s:%s:%09d", anchors[i].ID, anchors[i].Path, anchors[i].Line) < fmt.Sprintf("%s:%s:%09d", anchors[j].ID, anchors[j].Path, anchors[j].Line)
	})
	return anchors, nil
}

func adjacentDeclaration(lines []string, anchorIndex int, kind, extension string) (*string, *int) {
	limit := min(anchorIndex+7, len(lines))
	for index := anchorIndex + 1; index < limit; index++ {
		line := strings.TrimSpace(lines[index])
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") {
			continue
		}
		var match []string
		switch {
		case kind == "test":
			if extension == ".go" {
				match = goFunctionPattern.FindStringSubmatch(line)
				if match == nil || !strings.HasPrefix(match[1], "Test") {
					return nil, nil
				}
			} else {
				match = jsTestPattern.FindStringSubmatch(line)
				if match == nil {
					return nil, nil
				}
			}
		case extension == ".go":
			match = goFunctionPattern.FindStringSubmatch(line)
			if match == nil {
				match = goTypePattern.FindStringSubmatch(line)
			}
			if match == nil {
				return nil, nil
			}
		default:
			for _, pattern := range []*regexp.Regexp{jsFunctionPattern, jsClassPattern, jsMethodPattern} {
				match = pattern.FindStringSubmatch(line)
				if match != nil {
					break
				}
			}
			if match == nil || slices.Contains([]string{"if", "for", "while", "switch", "catch"}, match[1]) {
				return nil, nil
			}
		}
		selector := match[1]
		declarationLine := index + 1
		return &selector, &declarationLine
	}
	return nil, nil
}

func contains(values []string, target string) bool {
	return slices.Contains(values, target)
}
