package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

type request struct {
	File string `json:"file"`
}

type testDeclaration struct {
	Name      string `json:"name"`
	Parameter string `json:"parameter"`
	Exported  bool   `json:"exported"`
}

type response struct {
	Package string            `json:"package"`
	Tests   []testDeclaration `json:"tests"`
}

func main() {
	var input request
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fail(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), input.File, nil, parser.SkipObjectResolution)
	if err != nil {
		fail(err)
	}
	testingAliases := map[string]bool{}
	dotTestingImport := false
	for _, imported := range file.Imports {
		if imported.Path.Value != `"testing"` {
			continue
		}
		if imported.Name == nil {
			testingAliases["testing"] = true
		} else if imported.Name.Name == "." {
			dotTestingImport = true
		} else if imported.Name.Name != "_" {
			testingAliases[imported.Name.Name] = true
		}
	}

	result := response{Package: file.Name.Name, Tests: []testDeclaration{}}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Type.Results != nil || function.Type.Params == nil || len(function.Type.Params.List) != 1 {
			continue
		}
		parameter := function.Type.Params.List[0]
		pointer, ok := parameter.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		canonicalTestingT := false
		if testingType, ok := pointer.X.(*ast.SelectorExpr); ok && testingType.Sel.Name == "T" {
			packageName, ok := testingType.X.(*ast.Ident)
			canonicalTestingT = ok && testingAliases[packageName.Name]
		} else if testingType, ok := pointer.X.(*ast.Ident); ok {
			canonicalTestingT = dotTestingImport && testingType.Name == "T"
		}
		if !canonicalTestingT {
			continue
		}
		result.Tests = append(result.Tests, testDeclaration{
			Name:      function.Name.Name,
			Parameter: "*testing.T",
			Exported:  ast.IsExported(function.Name.Name),
		})
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
