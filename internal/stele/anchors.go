package stele

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var (
	anchorPattern = regexp.MustCompile(
		`@(implements|verifies)\s+((?:req|scn)\.[a-z0-9]+\.[a-f0-9]{12})` +
			`((?:\.(?:unit|integration|e2e)(?:\.[0-9]+)?\b)?)`,
	)
	typeScriptTestPattern = regexp.MustCompile(
		`^(?:(?:void|await)\s+)?(?:test|it)\(\s*(["'\x60])([^"'\x60]+)["'\x60]`,
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
func rootGoFiles(repo repoFiles, root string) []string {
	names, _ := repo.children(root)
	files := make([]string, 0)
	for _, name := range names {
		if strings.HasSuffix(name, ".go") {
			files = append(files, filepath.Join(root, name))
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

// anchorCache is repository files that keep each file's anchors until the
// file changes, so a rescan reads only changed files.
type anchorCache interface {
	cachedAnchors(file anchorFile, scan func() ([]Anchor, error)) ([]Anchor, error)
}

// ScanAnchors finds explicit Stele annotations in TypeScript and Go files and binds
// them to nearby implementation or test declarations. Unsupported file types
// are ignored; the scanner does not infer behavior from code.
// @implements req.verify.43d1d0d9f883
func ScanAnchors(root string) ([]Anchor, error) {
	return scanAnchors(diskFiles{}, root)
}

// scanAnchors finds the anchors of the repository files that repo gives.
func scanAnchors(repo repoFiles, root string) ([]Anchor, error) {
	files := discoverAnchorFiles(repo, root)
	anchors := make([]Anchor, 0)
	cache, cached := repo.(anchorCache)
	for _, file := range files {
		scan := func() ([]Anchor, error) { return scanAnchorFile(repo, root, file) }
		var found []Anchor
		var err error
		if cached {
			found, err = cache.cachedAnchors(file, scan)
		} else {
			found, err = scan()
		}
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

func discoverAnchorFiles(repo repoFiles, root string) []anchorFile {
	codeSet := make(map[string]struct{})
	testSet := make(map[string]struct{})
	for _, directory := range anchorScanRoots {
		for _, file := range repo.walk(filepath.Join(root, directory), supportedSource) {
			if isTestFile(file) {
				testSet[file] = struct{}{}
				continue
			}
			if directory != "tests" {
				codeSet[file] = struct{}{}
			}
		}
	}
	for _, file := range rootGoFiles(repo, root) {
		if isTestFile(file) {
			testSet[file] = struct{}{}
		} else {
			codeSet[file] = struct{}{}
		}
	}
	for _, name := range []string{"server.ts", "server.tsx", "server.mts"} {
		file := filepath.Join(root, name)
		if repo.isFile(file) {
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

func scanAnchorFile(repo repoFiles, root string, file anchorFile) ([]Anchor, error) {
	content, err := repo.readFile(file.path)
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
			evidence, level := anchorEvidence(match[2], match[3])
			anchors = append(anchors, Anchor{
				EvidenceID:      evidence,
				Level:           level,
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

// anchorEvidence splits an anchor's level suffix, such as ".e2e.2", into the
// full evidence ID and its level. A bare identity has neither.
func anchorEvidence(identity, suffix string) (string, string) {
	if suffix == "" {
		return "", ""
	}
	level, _, _ := strings.Cut(suffix[1:], ".")
	return identity + suffix, level
}

func anchorSortKey(anchor Anchor) string {
	return fmt.Sprintf("%s:%s:%09d:%s", anchor.ID, anchor.Path, anchor.Line, anchor.EvidenceID)
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

// testSelector returns the exact name of a test call. A template literal with
// an interpolation has no exact name, so it stays unresolved instead of
// selecting tests by a partial name.
//
// @implements req.tsanchors.3529ec7b6931
// @implements req.linkindex.860a4d91b9fe
func testSelector(line string) string {
	match := typeScriptTestPattern.FindStringSubmatch(line)
	if match == nil || (match[1] == "`" && strings.Contains(match[2], "${")) {
		return ""
	}
	return match[2]
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
