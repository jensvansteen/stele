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
		`^(?:(?:void|await)\s+)?(?:test|it)\(\s*["'\x60]([^"'\x60]+)["'\x60]`,
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

var anchorScanRoots = []string{"bin", "cmd", "internal", "pkg", "src", "public", "tools", "scripts", "tests"}

// rootGoFiles lists Go files directly in the repository root, without recursing.
func rootGoFiles(root string) []string {
	entries, _ := os.ReadDir(root)
	files := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			files = append(files, filepath.Join(root, entry.Name()))
		}
	}
	return files
}

func supportedSource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".mts", ".go":
		return true
	default:
		return false
	}
}

func isTestFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(name, ".go") {
		return strings.HasSuffix(name, "_test.go")
	}
	return strings.Contains(name, ".test.") || strings.Contains(name, ".spec.")
}

type anchorFile struct {
	path string
	kind string
}

// ScanAnchors finds explicit Stele annotations in TypeScript and Go files and binds
// them to nearby implementation or test declarations. Unsupported file types
// are ignored; the scanner does not infer behavior from code.
// @implements req.verify.43d1d0d9f883
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
	for _, directory := range anchorScanRoots {
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
	for _, file := range rootGoFiles(root) {
		if isTestFile(file) {
			testSet[file] = struct{}{}
		} else {
			codeSet[file] = struct{}{}
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

	if strings.EqualFold(filepath.Ext(file.path), ".go") {
		return scanGoAnchorFile(filepath.ToSlash(relative), content, file.kind)
	}
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	anchors := make([]Anchor, 0)
	for lineIndex, comment := range typeScriptCommentText(lines) {
		for _, match := range anchorPattern.FindAllStringSubmatch(comment, -1) {
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

type typeScriptLexState uint8

const (
	lexCode typeScriptLexState = iota
	lexLineComment
	lexBlockComment
	lexSingleQuote
	lexDoubleQuote
	lexTemplate
)

// typeScriptLexer separates comment text from code, strings, and template
// literals. It deliberately does not recognize regular-expression literals.
type typeScriptLexer struct {
	state     typeScriptLexState
	templates []int
	comment   strings.Builder
}

// typeScriptCommentText returns, for every line, only the text inside comments,
// so annotations spelled in string or template literals are never matched.
//
// @implements req.tsanchors.c02ad141ee6a
func typeScriptCommentText(lines []string) []string {
	lexer := typeScriptLexer{state: lexCode}
	comments := make([]string, len(lines))
	for index, line := range lines {
		lexer.beginLine()
		for position := 0; position < len(line); {
			position += lexer.step(line, position)
		}
		comments[index] = lexer.comment.String()
	}
	return comments
}

func (lexer *typeScriptLexer) beginLine() {
	lexer.comment.Reset()
	switch lexer.state {
	case lexLineComment, lexSingleQuote, lexDoubleQuote:
		lexer.state = lexCode
	}
}

// step consumes the character at position and returns how many bytes it used.
func (lexer *typeScriptLexer) step(line string, position int) int {
	character := line[position]
	pair := line[position:min(position+2, len(line))]
	switch lexer.state {
	case lexLineComment:
		lexer.comment.WriteByte(character)
	case lexBlockComment:
		if pair == "*/" {
			lexer.state = lexCode
			lexer.comment.WriteByte(' ')
			return 2
		}
		lexer.comment.WriteByte(character)
	case lexSingleQuote, lexDoubleQuote:
		return lexer.stepString(character)
	case lexTemplate:
		return lexer.stepTemplate(character, pair)
	default:
		return lexer.stepCode(character, pair)
	}
	return 1
}

func (lexer *typeScriptLexer) stepString(character byte) int {
	if character == '\\' {
		return 2
	}
	closing := byte('"')
	if lexer.state == lexSingleQuote {
		closing = '\''
	}
	if character == closing {
		lexer.state = lexCode
	}
	return 1
}

func (lexer *typeScriptLexer) stepTemplate(character byte, pair string) int {
	switch {
	case character == '\\':
		return 2
	case character == '`':
		lexer.state = lexCode
	case pair == "${":
		lexer.templates = append(lexer.templates, 1)
		lexer.state = lexCode
		return 2
	}
	return 1
}

func (lexer *typeScriptLexer) stepCode(character byte, pair string) int {
	switch {
	case pair == "//":
		lexer.state = lexLineComment
		lexer.comment.WriteByte(' ')
		return 2
	case pair == "/*":
		lexer.state = lexBlockComment
		lexer.comment.WriteByte(' ')
		return 2
	case character == '\'':
		lexer.state = lexSingleQuote
	case character == '"':
		lexer.state = lexDoubleQuote
	case character == '`':
		lexer.state = lexTemplate
	case character == '{' && len(lexer.templates) > 0:
		lexer.templates[len(lexer.templates)-1]++
	case character == '}' && len(lexer.templates) > 0:
		lexer.closeTemplateBrace()
	}
	return 1
}

func (lexer *typeScriptLexer) closeTemplateBrace() {
	last := len(lexer.templates) - 1
	lexer.templates[last]--
	if lexer.templates[last] == 0 {
		lexer.templates = lexer.templates[:last]
		lexer.state = lexTemplate
	}
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

// @implements req.tsanchors.3529ec7b6931
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
