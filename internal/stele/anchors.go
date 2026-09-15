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
	anchorPattern = regexp.MustCompile(
		`@(implements|verifies)\s+((?:req|scn)\.[a-z0-9]+\.[a-f0-9]{12})`,
	)
	typeScriptTestPattern = regexp.MustCompile(
		`^(?:test|it)\(\s*["'\x60]([^"'\x60]+)["'\x60]`,
	)
	typeScriptFunctionPattern = regexp.MustCompile(
		`^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)`,
	)
	typeScriptClassPattern = regexp.MustCompile(
		`^(?:export\s+)?class\s+([A-Za-z_$][\w$]*)`,
	)
	typeScriptMethodPattern = regexp.MustCompile(
		`^(?:async\s+)?#?([A-Za-z_$][\w$]*)\s*\(`,
	)
)

const declarationSearchLines = 6

func supportedSource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".mts":
		return true
	default:
		return false
	}
}

func isTestFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.Contains(name, ".test.") || strings.Contains(name, ".spec.")
}

type anchorFile struct {
	path string
	kind string
}

// ScanAnchors finds explicit Stele annotations in TypeScript files and binds
// them to nearby implementation or test declarations. Unsupported file types
// are ignored; the scanner does not infer behavior from code.
func ScanAnchors(root string) ([]Anchor, error) {
	files := discoverAnchorFiles(root)
	anchors := make([]Anchor, 0)
	for _, file := range files {
		found, err := scanAnchorFile(root, file)
		if err != nil {
			return nil, err
		}
		anchors = append(anchors, found...)
	}

	sort.Slice(anchors, func(i, j int) bool {
		return anchorSortKey(anchors[i]) < anchorSortKey(anchors[j])
	})
	return anchors, nil
}

func discoverAnchorFiles(root string) []anchorFile {
	codeSet := make(map[string]struct{})
	testSet := make(map[string]struct{})
	for _, directory := range []string{"bin", "cmd", "internal", "src", "public", "tools", "scripts", "tests"} {
		for _, file := range walkFiles(filepath.Join(root, directory), supportedSource) {
			if isTestFile(file) {
				testSet[file] = struct{}{}
				continue
			}
			if directory != "tests" {
				codeSet[file] = struct{}{}
			}
		}
	}
	for _, name := range []string{"server.ts", "server.tsx", "server.mts"} {
		file := filepath.Join(root, name)
		if fileExists(file) {
			codeSet[file] = struct{}{}
		}
	}

	files := make([]anchorFile, 0, len(codeSet)+len(testSet))
	for _, group := range []struct {
		kind  string
		files map[string]struct{}
	}{
		{kind: "code", files: codeSet},
		{kind: "test", files: testSet},
	} {
		for path := range group.files {
			files = append(files, anchorFile{path: path, kind: group.kind})
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].kind+":"+files[i].path < files[j].kind+":"+files[j].path
	})
	return files
}

func scanAnchorFile(root string, file anchorFile) ([]Anchor, error) {
	content, err := os.ReadFile(file.path)
	if err != nil {
		return nil, err
	}
	relative, err := relativePath(root, file.path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	anchors := make([]Anchor, 0)
	for lineIndex, line := range lines {
		for _, match := range anchorPattern.FindAllStringSubmatch(line, -1) {
			selector, declarationLine := adjacentDeclaration(lines, lineIndex, file.kind)
			anchors = append(anchors, Anchor{
				ID:              match[2],
				Annotation:      match[1],
				Kind:            file.kind,
				Path:            filepath.ToSlash(relative),
				Line:            lineIndex + 1,
				Selector:        selector,
				DeclarationLine: declarationLine,
			})
		}
	}
	return anchors, nil
}

func anchorSortKey(anchor Anchor) string {
	return fmt.Sprintf("%s:%s:%09d", anchor.ID, anchor.Path, anchor.Line)
}

func adjacentDeclaration(lines []string, anchorIndex int, kind string) (*string, *int) {
	limit := min(anchorIndex+declarationSearchLines+1, len(lines))
	for index := anchorIndex + 1; index < limit; index++ {
		line := strings.TrimSpace(lines[index])
		if isIgnorableDeclarationLine(line) {
			continue
		}
		selector := declarationSelector(line, kind)
		if selector == "" {
			return nil, nil
		}
		declarationLine := index + 1
		return &selector, &declarationLine
	}
	return nil, nil
}

func isIgnorableDeclarationLine(line string) bool {
	if line == "" {
		return true
	}
	return strings.HasPrefix(line, "//") ||
		strings.HasPrefix(line, "/*") ||
		strings.HasPrefix(line, "*")
}

func declarationSelector(line, kind string) string {
	if kind == "test" {
		return testSelector(line)
	}
	return codeSelector(line)
}

func testSelector(line string) string {
	match := typeScriptTestPattern.FindStringSubmatch(line)
	if match == nil {
		return ""
	}
	return match[1]
}

func codeSelector(line string) string {
	patterns := []*regexp.Regexp{
		typeScriptFunctionPattern,
		typeScriptClassPattern,
		typeScriptMethodPattern,
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(line)
		if match == nil || slices.Contains([]string{"if", "for", "while", "switch", "catch"}, match[1]) {
			continue
		}
		return match[1]
	}
	return ""
}

func contains(values []string, target string) bool {
	return slices.Contains(values, target)
}
