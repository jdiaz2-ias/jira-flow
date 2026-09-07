//go:build foundation

package foundation

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// This is a source-level regression check, not a macOS runtime certification.
func TestPinnedMacKeyringDoesNotPassSecretInArgv(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-json", "github.com/zalando/go-keyring").CombinedOutput()
	if err != nil {
		t.Fatalf("locating pinned dependency: %v\n%s", err, out)
	}
	var module struct{ Dir, Version string }
	if err := json.Unmarshal(out, &module); err != nil {
		t.Fatal(err)
	}
	if module.Version != "v0.2.8" {
		t.Fatal("dependency changed: review ADR-004 and update security audit")
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(module.Dir, "keyring_darwin.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var set *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "Set" {
			set = fn
		}
	}
	if set == nil {
		t.Fatal("Set implementation not found")
	}
	commands, stdinPipes, writes := 0, 0, 0
	ast.Inspect(set.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "StdinPipe" {
			stdinPipes++
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "io" && sel.Sel.Name == "WriteString" {
			writes++
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "exec" && sel.Sel.Name == "Command" {
			commands++
			if len(call.Args) != 2 {
				t.Error("unexpected process arguments")
				return true
			}
			path, pathOK := call.Args[0].(*ast.Ident)
			flag, flagOK := call.Args[1].(*ast.BasicLit)
			if !pathOK || path.Name != "execPathKeychain" || !flagOK {
				t.Error("unexpected process invocation")
				return true
			}
			value, err := strconv.Unquote(flag.Value)
			if err != nil || value != "-i" {
				t.Error("expected security interactive mode")
			}
		}
		return true
	})
	if commands != 1 || stdinPipes != 1 || writes != 1 {
		t.Fatalf("unexpected secret transport: command=%d stdin=%d write=%d", commands, stdinPipes, writes)
	}
}
