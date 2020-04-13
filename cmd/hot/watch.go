package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/kylelemons/godebug/diff"
)

func watchChanges(stdin io.WriteCloser) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("watchChanges: fsnotify.NewWatcher(): %v", err)
	}

	err = filepath.Walk(*watchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return watcher.Add(path)
		}

		return nil
	})

	if err != nil {
		log.Fatalf("watchChanges: walk(%q): %v", *watchDir, err)
	}

	for ev := range watcher.Events {
		if ev.Op == fsnotify.Chmod {
			continue
		}

		if ev.Op != fsnotify.Write {
			log.Fatalf("Only WRITE changes are supported for live reload. Received %s event for %q", ev.Op, ev.Name)
		}

		time.Sleep(time.Millisecond * 25)

		handleEvent(ev, stdin)
	}
}

// computeChangedLines calculates which lines in the new file have changed and/or deleted.
func computeChangedLines(oldContents, newContents []byte) map[int]bool {
	chunks := diff.DiffChunks(strings.Split(string(oldContents), "\n"),
		strings.Split(string(newContents), "\n"))

	changedLines := make(map[int]bool)
	curNewLn := 1

	for _, ch := range chunks {
		if len(ch.Deleted) > 0 {
			changedLines[curNewLn] = true
		}

		if len(ch.Added) > 0 {
			for i := 0; i < len(ch.Added); i++ {
				changedLines[curNewLn+i] = true
			}
			curNewLn += len(ch.Added)
		}

		curNewLn += len(ch.Equal)
	}

	return changedLines
}

func getChangedDecls(fset *token.FileSet, f *ast.File, changedLines map[int]bool) ([]*ast.FuncDecl, error) {
	var changedDecls []*ast.FuncDecl
	changedLinesLeft := make(map[int]bool)
	for k, v := range changedLines {
		changedLinesLeft[k] = v
	}

	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			startLn := fset.Position(d.Pos()).Line
			endLn := fset.Position(d.End()).Line

			changed := false

			for i := startLn; i <= endLn; i++ {
				if changedLinesLeft[i] {
					changed = true
					delete(changedLinesLeft, i)
				}
			}

			if changed {
				changedDecls = append(changedDecls, d)
			}
		case *ast.GenDecl:
			// allow changes in imports
			if d.Tok != token.IMPORT {
				break
			}

			startLn := fset.Position(d.Pos()).Line
			endLn := fset.Position(d.End()).Line

			for i := startLn; i <= endLn; i++ {
				delete(changedLinesLeft, i)
			}
		}
	}

	if len(changedLinesLeft) > 0 {
		return nil, fmt.Errorf("Changed some lines that do not belong to the function implementations: %+v", changedLinesLeft)
	}

	return changedDecls, nil
}

func getFuncDeclName(d *ast.FuncDecl, origPkgName string) string {
	var name string

	if d.Recv != nil {
		switch r := d.Recv.List[0].Type.(type) {
		case *ast.StarExpr:
			name = "*" + r.X.(*ast.Ident).Name + "." + d.Name.Name
		case *ast.Ident:
			name = r.Name + "." + d.Name.Name
		}
	} else {
		name = d.Name.Name
	}

	return name
}

func rewriteFuncDecl(d *ast.FuncDecl, origPkgName string) *ast.FuncDecl {
	if d.Recv != nil {
		var l []*ast.Field
		rcv := d.Recv.List[0]
		switch r := rcv.Type.(type) {
		case *ast.StarExpr:
			rcv.Type = &ast.StarExpr{X: &ast.SelectorExpr{
				X:   ast.NewIdent(origPkgName),
				Sel: r.X.(*ast.Ident),
			}}
		case *ast.Ident:
			rcv.Type = &ast.SelectorExpr{
				X:   ast.NewIdent(origPkgName),
				Sel: r,
			}
		}

		l = append(l, rcv)
		l = append(l, d.Type.Params.List...)
		d.Type.Params.List = l
		d.Recv = nil
	}

	return d
}

func compileNewFile(pkgPath, filename string, contents []byte, changedLines map[int]bool) (string, error) {
	fset := token.NewFileSet() // positions are relative to fset

	f, err := parser.ParseFile(fset, filename, contents, 0)
	if err != nil {
		return "", err
	}

	origPkgName := f.Name.Name // name of the package originally (not to be confused with it's path)

	decls, err := getChangedDecls(fset, f, changedLines)
	if err != nil {
		return "", err
	}

	var mockBody []ast.Stmt
	var imports []ast.Decl
	var haveHot bool

	for idx, d := range f.Decls {
		if d, ok := d.(*ast.FuncDecl); ok {
			// hot.ResetByName(package)
			mockBody = append(mockBody, &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						Sel: ast.NewIdent("ResetByName"),
						X:   ast.NewIdent("hot"),
					},
					Args: []ast.Expr{
						&ast.BasicLit{
							Kind:  token.STRING,
							Value: fmt.Sprintf("%q", pkgPath+"/"+getFuncDeclName(d, origPkgName)),
						},
					},
				},
			})
			continue
		}

		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}

		for _, sp := range gen.Specs {
			imp := sp.(*ast.ImportSpec)
			if imp.Path.Value == `"github.com/YuriyNasretdinov/hotreload"` {
				haveHot = true
			}
		}

		// add import for current package to somewhere
		if idx == 0 {
			gen.Specs = append(gen.Specs, &ast.ImportSpec{
				Name: ast.NewIdent(origPkgName),
				Path: &ast.BasicLit{Value: fmt.Sprintf("%q", pkgPath)},
			})
		}

		imports = append(imports, gen)
	}

	if !haveHot {
		d := imports[0].(*ast.GenDecl)
		d.Specs = append(d.Specs, &ast.ImportSpec{
			Name: ast.NewIdent("hot"),
			Path: &ast.BasicLit{Value: `"github.com/YuriyNasretdinov/hotreload"`},
		})
	}

	// plugin package must be called main
	f.Name = ast.NewIdent("main")

	f.Decls = nil
	f.Decls = append(f.Decls, imports...)

	for _, d := range decls {
		name := getFuncDeclName(d, origPkgName)
		fun := rewriteFuncDecl(d, origPkgName)
		f.Decls = append(f.Decls, fun)

		// hot.MockByName(package, function)
		mockBody = append(mockBody, &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					Sel: ast.NewIdent("MockByName"),
					X:   ast.NewIdent("hot"),
				},
				Args: []ast.Expr{
					&ast.BasicLit{
						Kind:  token.STRING,
						Value: fmt.Sprintf("%q", pkgPath+"/"+name),
					},
					ast.NewIdent(fun.Name.Name),
				},
			},
		})
	}

	mockBody = append(mockBody, &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: ast.NewIdent("println"),
			Args: []ast.Expr{
				&ast.BasicLit{
					Kind:  token.STRING,
					Value: fmt.Sprintf("%q", fmt.Sprintf("new plugin loaded at %s (%d)", time.Now(), time.Now().UnixNano())),
				},
			},
		},
	})

	f.Decls = append(f.Decls, &ast.FuncDecl{
		Name: ast.NewIdent("Mock"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
		},
		Body: &ast.BlockStmt{
			List: mockBody,
		},
	})

	f.Decls = append(f.Decls, &ast.FuncDecl{
		Name: ast.NewIdent("main"),
		Type: &ast.FuncType{
			Func:   token.NoPos,
			Params: &ast.FieldList{},
		},
		Body: &ast.BlockStmt{},
	})

	pr := (&printer.Config{Tabwidth: 4})

	var b bytes.Buffer
	if err := pr.Fprint(&b, fset, f); err != nil {
		return "", err
	}

	liveDir := softGopath + "/src/live"
	liveFile := liveDir + "/main.go"
	plugPath := liveDir + "/plug" + fmt.Sprint(time.Now().UnixNano()+rand.Int63()) + ".so"

	os.RemoveAll(liveDir)
	if err := os.MkdirAll(liveDir, 0777); err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(liveFile, b.Bytes(), 0666); err != nil {
		return "", err
	}

	start := time.Now()
	goimports := exec.Command("goimports", "-w", liveFile)
	goimports.Stderr = os.Stderr
	if err := goimports.Run(); err != nil {
		return "", fmt.Errorf("Goimports for %q failed: %v", liveFile, err)
	}
	log.Printf("goimports finished in %s", time.Since(start))

	start = time.Now()
	gobuild := exec.Command("go", "build", "-buildmode=plugin", "-o", plugPath, liveFile)
	gobuild.Stderr = os.Stderr
	if err := gobuild.Run(); err != nil {
		return "", fmt.Errorf("Go build for plugin for %q failed: %v", liveFile, err)
	}
	log.Printf("go build -buildmode=plugin finished in %s", time.Since(start))

	// newContents, err := ioutil.ReadFile(liveFile)
	// if err != nil {
	// return "", err
	// }

	// log.Printf("New contents: %s", newContents)

	return plugPath, nil
}

func handleEvent(ev fsnotify.Event, stdin io.WriteCloser) {
	goPkg := strings.TrimPrefix(ev.Name, gopath+"/src/")
	origPath := softGopath + "/src/" + goPkg + ".orig"

	newContents, err := ioutil.ReadFile(ev.Name)
	if err != nil {
		log.Fatalf("Couldn't read %q: %v", ev.Name, err)
	}

	origContents, err := ioutil.ReadFile(origPath)
	if err != nil {
		log.Fatalf("Couldn't read %q: %v", origPath, err)
	}

	changedLines := computeChangedLines(origContents, newContents)

	plugPath, err := compileNewFile(filepath.Dir(goPkg), ev.Name, newContents, changedLines)
	if err != nil {
		log.Fatalf("Compilation of %q failed: %v", ev.Name, err)
	}

	log.Printf("Compiled new plugin: %s", plugPath)
	fmt.Fprintf(stdin, "%s\n", plugPath)
}
