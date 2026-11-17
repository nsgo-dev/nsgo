package moduletest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"testing"
)

// blockBridge is the one package that may build Objective-C blocks itself: purego's objc.NewBlock hands _Block_copy a Go stack address as a uintptr, which a stack move between the conversion and the copy leaves stale, so the copy reads garbage or fails and Foundation receives a nil block (#428).
const blockBridge = "internal/platform/objcblock"

// TestObjectiveCBlocksUseTheBridge fails when a package outside blockBridge calls purego's objc.NewBlock, under whatever name the file imports objc as.
func TestObjectiveCBlocksUseTheBridge(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); rel != "." && (name[0] == '.' || name == "testdata" || rel == filepath.FromSlash(blockBridge)) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		names := map[string]bool{}
		for _, spec := range file.Imports {
			if p, _ := strconv.Unquote(spec.Path.Value); p == "github.com/ebitengine/purego/objc" {
				name := "objc"
				if spec.Name != nil {
					name = spec.Name.Name
				}
				names[name] = true
			}
		}
		if len(names) == 0 {
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "NewBlock" {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && names[pkg.Name] {
				t.Errorf("%s: objc.NewBlock; create blocks with %s.New", fset.Position(sel.Pos()), blockBridge)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
