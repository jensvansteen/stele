package stele

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"
)

// scanGoAnchorFile reads annotations only from Go comments and binds each one
// to the Go declaration that directly follows it. A file that does not parse
// is an error that names the file.
//
// @implements req.gosupport.4c353f2b171a
func scanGoAnchorFile(relative string, content []byte, kind string) ([]Anchor, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, relative, content, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	declarations := goDeclarationSelectors(fileSet, file, kind)
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	anchors := make([]Anchor, 0)
	for _, group := range file.Comments {
		for _, comment := range group.List {
			start := fileSet.Position(comment.Slash).Line
			for _, match := range anchorPattern.FindAllStringSubmatchIndex(comment.Text, -1) {
				line := start + strings.Count(comment.Text[:match[0]], "\n")
				selector, declarationLine := goAdjacentDeclaration(lines, line-1, declarations)
				anchors = append(anchors, Anchor{
					ID:              comment.Text[match[4]:match[5]],
					Annotation:      comment.Text[match[2]:match[3]],
					Kind:            kind,
					Path:            relative,
					Line:            line,
					Selector:        selector,
					DeclarationLine: declarationLine,
				})
			}
		}
	}
	return anchors, nil
}

// goDeclarationSelectors maps the first line of every top-level declaration to
// its selector, or to "" when the declaration cannot be a target.
func goDeclarationSelectors(fileSet *token.FileSet, file *ast.File, kind string) map[int]string {
	selectors := make(map[int]string, len(file.Decls))
	for _, declaration := range file.Decls {
		selectors[fileSet.Position(declaration.Pos()).Line] = goSelector(declaration, kind)
	}
	return selectors
}

func goSelector(declaration ast.Decl, kind string) string {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		if kind == "test" {
			if isGoTestFunction(typed) {
				return typed.Name.Name
			}
			return ""
		}
		if typed.Recv == nil {
			return typed.Name.Name
		}
		if receiver := goReceiverName(typed.Recv); receiver != "" {
			return receiver + "." + typed.Name.Name
		}
	case *ast.GenDecl:
		if kind == "code" && typed.Tok == token.TYPE && len(typed.Specs) == 1 {
			if spec, ok := typed.Specs[0].(*ast.TypeSpec); ok {
				return spec.Name.Name
			}
		}
	}
	return ""
}

func goReceiverName(receiver *ast.FieldList) string {
	if len(receiver.List) == 0 {
		return ""
	}
	expression := receiver.List[0].Type
	for {
		switch typed := expression.(type) {
		case *ast.StarExpr:
			expression = typed.X
		case *ast.ParenExpr:
			expression = typed.X
		case *ast.IndexExpr:
			expression = typed.X
		case *ast.IndexListExpr:
			expression = typed.X
		case *ast.Ident:
			return typed.Name
		default:
			return ""
		}
	}
}

// isGoTestFunction mirrors `go test`: a top-level TestXxx function whose only
// parameter is *testing.T.
func isGoTestFunction(function *ast.FuncDecl) bool {
	name := function.Name.Name
	if function.Recv != nil || function.Type.TypeParams != nil || !strings.HasPrefix(name, "Test") {
		return false
	}
	if next, _ := utf8.DecodeRuneInString(name[len("Test"):]); unicode.IsLower(next) {
		return false
	}
	parameters := function.Type.Params.List
	if len(parameters) != 1 || len(parameters[0].Names) > 1 {
		return false
	}
	pointer, ok := parameters[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "testing" && selector.Sel.Name == "T"
}

func goAdjacentDeclaration(lines []string, anchorIndex int, declarations map[int]string) (*string, *int) {
	limit := min(anchorIndex+declarationSearchLines+1, len(lines))
	for index := anchorIndex + 1; index < limit; index++ {
		if isIgnorableDeclarationLine(strings.TrimSpace(lines[index])) {
			continue
		}
		selector := declarations[index+1]
		if selector == "" {
			return nil, nil
		}
		declarationLine := index + 1
		return &selector, &declarationLine
	}
	return nil, nil
}
