package main

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var printerCfg = &printer.Config{
	Tabwidth: 4,
	Mode:     printer.SourcePos,
}

// TODO: make sure that "soft" is not used and handle case when "atomic" is imported under a different name
func addSoftImport(fset *token.FileSet, f *ast.File) {
	importSpecs := []ast.Spec{
		&ast.ImportSpec{
			Name: &ast.Ident{
				Name: "hot",
			},
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: `"github.com/YuriyNasretdinov/hotreload"`,
			},
		},
		&ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: `"sync/atomic"`,
			},
		},
	}

	alreadyImported := make(map[string]bool)

	for _, d := range f.Decls {
		if d, ok := d.(*ast.GenDecl); ok && d.Tok == token.IMPORT {
			for _, sp := range d.Specs {
				if sp, ok := sp.(*ast.ImportSpec); ok {
					alreadyImported[sp.Path.Value] = true
				}
			}
		}
	}

	var specs []ast.Spec

	for _, sp := range importSpecs {
		if alreadyImported[sp.(*ast.ImportSpec).Path.Value] {
			continue
		}
		specs = append(specs, sp)
	}

	if len(specs) == 0 {
		return
	}

	decls := make([]ast.Decl, 0, len(f.Decls)+len(specs))
	for _, sp := range specs {
		decls = append(decls, &ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: []ast.Spec{sp},
		})
	}

	decls = append(decls, f.Decls...)

	f.Decls = decls
}

func funcDeclFlagName(fset *token.FileSet, d *ast.FuncDecl) string {
	var parts []string
	if d.Body == nil {
		return "" // no body, so obviously cannot mock it
	}

	parts = append(parts, fset.Position(d.Body.Lbrace).String(), fset.Position(d.Body.Rbrace).String())
	h := md5.New()
	for _, p := range parts {
		h.Write([]byte(p))
	}
	return fmt.Sprintf("softMocksFlag_%x", h.Sum(nil))
}

// checks if we have situation like "func (file *file) close() error" in "os" package
// TODO: we can actually rename arguments when this happens so there is no ambiguity
func typesClashWithArgNames(decls []*ast.Field) bool {
	// there also are some predefined names, such as "soft" and "hot" that can't be
	// used as variables inside the function without causing problems.
	namesMap := map[string]bool{
		"hot":  true,
		"soft": true,
	}
	for _, d := range decls {
		for _, n := range d.Names {
			namesMap[n.Name] = true
		}
	}

	clash := false

	for _, d := range decls {
		ast.Inspect(d.Type, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				if namesMap[n.Name] {
					clash = true
				}
			}
			return true
		})
	}

	return clash
}

func funcDeclExpr(f *ast.FuncDecl) ast.Expr {
	if f.Recv == nil {
		return ast.NewIdent(f.Name.Name)
	}

	return &ast.SelectorExpr{
		X:   &ast.ParenExpr{X: f.Recv.List[0].Type},
		Sel: ast.NewIdent(f.Name.Name),
	}
}

var ErrNoNames = errors.New("No names in receiver")

func argNamesFromFuncDecl(f *ast.FuncDecl) ([]ast.Expr, bool, error) {
	var res []ast.Expr
	var haveEllipsis bool

	if f.Recv != nil {
		names := f.Recv.List[0].Names
		if len(names) == 0 {
			return nil, false, ErrNoNames
		}
		res = append(res, names[0])
	}

	for _, t := range f.Type.Params.List {
		if len(t.Names) == 0 {
			return nil, false, ErrNoNames
		}

		for _, n := range t.Names {
			if _, ok := t.Type.(*ast.Ellipsis); ok {
				haveEllipsis = true
			}
			res = append(res, n)
		}
	}

	return res, haveEllipsis, nil
}

func funcDeclType(f *ast.FuncDecl) ast.Expr {
	var in []*ast.Field

	if f.Recv != nil {
		in = append(in, f.Recv.List[0])
	}

	for _, t := range f.Type.Params.List {
		in = append(in, t)
	}

	if typesClashWithArgNames(in) {
		return nil
	}

	return &ast.FuncType{
		Params:  &ast.FieldList{List: in},
		Results: f.Type.Results,
	}
}

type funcMeta struct {
	flagName string // the flag name to be used in the interceptor
	funcName string // a unique name for the function that can be used to fully identify it
}

type funcFlags map[*ast.FuncDecl]funcMeta

func addInit(hashes funcFlags, initFunc *ast.FuncDecl, fset *token.FileSet, f *ast.File) {
	specs := &ast.ValueSpec{
		Type: ast.NewIdent("int32"),
	}

	for decl, flagMeta := range hashes {
		specs.Names = append(specs.Names, ast.NewIdent(flagMeta.flagName))

		initFunc.Body.List = append(initFunc.Body.List, &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent("hot"),
					Sel: ast.NewIdent("RegisterFunc"),
				},
				Args: []ast.Expr{
					funcDeclExpr(decl),
					&ast.BasicLit{
						Value: fmt.Sprintf("%q", flagMeta.funcName),
					},
					&ast.UnaryExpr{
						Op: token.AND,
						X:  ast.NewIdent(flagMeta.flagName),
					},
				},
			},
		})

	}

	f.Decls = append(f.Decls, &ast.GenDecl{
		Tok:   token.VAR,
		Specs: []ast.Spec{specs},
	})
}

func getInterceptor(decl *ast.FuncDecl, haveReturn bool) *ast.IfStmt {
	funcType := funcDeclType(decl)
	if funcType == nil {
		return nil
	}

	args, haveEllipsis, err := argNamesFromFuncDecl(decl)
	if err != nil {
		return nil
	}

	// if soft := hot.GetMockFor(<function pointer>); soft != nil {
	//   return soft.(func(...) ...)(<args>)
	//     -or-
	//   soft.(func(...) ...)(<args>)
	//   return
	// }

	getMockExpr := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   ast.NewIdent("hot"),
			Sel: ast.NewIdent("GetMockFor"),
		},
		Args: []ast.Expr{funcDeclExpr(decl)},
	}

	callExpr := &ast.CallExpr{
		Fun: &ast.TypeAssertExpr{
			X:    getMockExpr,
			Type: funcType,
		},
		Args: args,
	}

	if haveEllipsis {
		callExpr.Ellipsis = 1
	}

	var bodyStmts []ast.Stmt

	if haveReturn {
		bodyStmts = append(bodyStmts, &ast.ReturnStmt{Results: []ast.Expr{callExpr}})
	} else {
		bodyStmts = append(bodyStmts, &ast.ExprStmt{X: callExpr}, &ast.ReturnStmt{})
	}

	return &ast.IfStmt{
		Init: &ast.AssignStmt{
			Tok: token.DEFINE,
			Lhs: []ast.Expr{ast.NewIdent("soft")},
			Rhs: []ast.Expr{getMockExpr},
		},
		Cond: &ast.BinaryExpr{
			Op: token.NEQ,
			X:  ast.NewIdent("soft"),
			Y:  ast.NewIdent("nil"),
		},
		Body: &ast.BlockStmt{List: bodyStmts},
	}
}

func injectInterceptors(flags funcFlags) {
	for decl, flagMeta := range flags {
		interceptor := getInterceptor(decl, decl.Type.Results != nil)
		if interceptor == nil {
			delete(flags, decl)
			continue
		}

		newList := make([]ast.Stmt, 0, len(decl.Body.List)+1)
		newList = append(newList, &ast.IfStmt{
			Cond: &ast.BinaryExpr{
				Op: token.NEQ,
				X: &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   ast.NewIdent("atomic"),
						Sel: ast.NewIdent("LoadInt32"),
					},
					Args: []ast.Expr{&ast.UnaryExpr{
						Op: token.AND,
						X:  ast.NewIdent(flagMeta.flagName),
					}},
				},
				Y: &ast.BasicLit{
					Kind:  token.INT,
					Value: "0",
				},
			},
			Body: &ast.BlockStmt{List: []ast.Stmt{interceptor}},
		})
		newList = append(newList, decl.Body.List...)
		decl.Body.List = newList
	}
}

func transformAst(filename string, fset *token.FileSet, f *ast.File) {
	flags := make(funcFlags)
	var initFunc *ast.FuncDecl

	pkgPrefix := strings.TrimLeft(strings.TrimPrefix(filepath.Dir(filename), gopath), string(os.PathSeparator)+"src"+string(os.PathSeparator))

	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == "init" && d.Recv == nil {
				initFunc = d
			} else if flName := funcDeclFlagName(fset, d); flName != "" {
				var funcName string

				if d.Recv != nil {
					switch l := d.Recv.List[0].Type.(type) {
					case *ast.StarExpr:
						funcName = "*" + l.X.(*ast.Ident).Name
					case *ast.Ident:
						funcName = l.Name
					}

					funcName += "." + d.Name.Name
				} else {
					funcName = d.Name.Name
				}

				flags[d] = funcMeta{
					flagName: flName,
					funcName: pkgPrefix + "/" + funcName,
				}
			}
		}
	}

	injectInterceptors(flags)

	if len(flags) == 0 {
		return
	}

	addSoftImport(fset, f)

	if initFunc == nil {
		initFunc = &ast.FuncDecl{
			Name: ast.NewIdent("init"),
			Type: &ast.FuncType{},
			Body: &ast.BlockStmt{},
		}

		f.Decls = append(f.Decls, initFunc)
	}

	addInit(flags, initFunc, fset, f)
}

// checks only exact package, not subpackages (because examples and the soft util itself live there)
func isSoftPackage(filename string) bool {
	return filepath.Dir(filename) == filepath.Join(gopath, "src", "github.com", "YuriyNasretdinov", "hotreload")
}

func rewriteFile(filename string) (contents []byte, err error) {
	if !strings.HasSuffix(filename, ".go") || isSoftPackage(filename) {
		return ioutil.ReadFile(filename)
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	fset := token.NewFileSet() // positions are relative to fset
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	cmap := ast.NewCommentMap(fset, f, f.Comments)
	transformAst(filename, fset, f)
	f.Comments = cmap.Filter(f).Comments()

	var b bytes.Buffer
	if err := printerCfg.Fprint(&b, fset, f); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}
