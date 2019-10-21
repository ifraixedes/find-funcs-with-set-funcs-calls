// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

func main() {
	cmdp, err := params(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	funcsFiles, err := find(cmdp)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(funcsFiles)
}

type cmdParams struct {
	pkgsPatterns []string
	funcCalls    []funcCall
}

type funcCall struct {
	pkg      string
	receiver string
	funcName string
}

type funcsByFile struct {
	Filename  string
	FuncNames []string
}

// params parses and maps the command line flags and arguments. inParams is the
// list of command line arguments without the program name.
func params(inParams []string) (cmdParams, error) {
	fset := flag.NewFlagSet("", flag.ExitOnError)
	funcs := fset.String("funcs", "",
		"the list of the functions to find where are all called inside of a function. It's a comma separated list of: pkg.[type.].func",
	)

	if err := fset.Parse(inParams); err != nil {
		return cmdParams{}, err
	}

	if *funcs == "" {
		return cmdParams{}, errors.New("funcs argument is required and it cannot be empty")
	}

	fcalls, err := parseFuncCalls(*funcs)
	if err != nil {
		return cmdParams{}, err
	}

	return cmdParams{
		pkgsPatterns: fset.Args(),
		funcCalls:    fcalls,
	}, nil
}

func parseFuncCalls(funcCallsFlagVal string) ([]funcCall, error) {
	funcCallsVals := strings.Split(funcCallsFlagVal, ",")

	funcCalls := make([]funcCall, len(funcCallsVals))
	for i, val := range funcCallsVals {
		var (
			fcv = strings.TrimSpace(val)
			pkg string
		)
		fpi := strings.LastIndex(fcv, "/")
		if fpi >= 0 {
			if fpi == (len(fcv) - 1) {
				return nil, fmt.Errorf(
					"Invalid function call reference, format is '<pkg path>.[<<type name>>.]<<func name>>'. Got: '%s'",
					val,
				)
			}

			pkg = fcv[:fpi+1]
			fcv = fcv[fpi+1:]
		}

		fpi = strings.Index(fcv, ".")
		if fpi < 0 || fpi == (len(fcv)-1) {
			return nil, fmt.Errorf(
				"Invalid function call reference, format is '<pkg path>.[<<type name>>.]<<func name>>'. Got: '%s'",
				val,
			)
		}

		pkg = fmt.Sprintf("%s%s", pkg, fcv[0:fpi])
		fcv = fcv[fpi+1:]

		var (
			receiver string
			funcName string
		)
		fpi = strings.Index(fcv, ".")
		switch {
		case fpi == 0:
			return nil, fmt.Errorf(
				"Invalid function call reference, format is '<pkg path>.[<<type name>>.]<<func name>>'. Got: '%s'",
				val,
			)

		case fpi > 0:
			if fpi == len(fcv)-1 {
				return nil, fmt.Errorf(
					"Invalid function call reference, format is '<pkg path>.[<<type name>>.]<<func name>>'. Got: '%s'",
					val,
				)
			}

			receiver = fcv[:fpi]
			funcName = fcv[fpi+1:]

		default:
			funcName = fcv
		}

		funcCalls[i] = funcCall{
			pkg:      pkg,
			receiver: receiver,
			funcName: funcName,
		}
	}

	return funcCalls, nil
}

func find(cmdp cmdParams) ([]funcsByFile, error) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedName |
			packages.NeedTypes | packages.NeedTypesInfo,
	}, cmdp.pkgsPatterns...)
	if err != nil {
		return nil, fmt.Errorf("error while loading packages: [%s]. %s",
			strings.Join(cmdp.pkgsPatterns, ", "), err,
		)
	}

	var funcsFiles []funcsByFile
	for _, p := range pkgs {
		ff, err := findFuncsNamesWhichCallFuncsSet(p, cmdp.funcCalls)
		if err != nil {
			return nil, err
		}

		funcsFiles = append(funcsFiles, ff...)
	}

	return funcsFiles, nil
}

// findFuncNamesWithCallsFuncsSet find the functions and methods declared in pkg
// which call all the funcCalls and return their name classified by Go source
// filepath.
//
// It returns an error if pkg doesn't contain the same number of compiled Go
// files than the files found in Syntax.
func findFuncsNamesWhichCallFuncsSet(pkg *packages.Package, funcCalls []funcCall) ([]funcsByFile, error) {
	if len(pkg.Syntax) != len(pkg.CompiledGoFiles) {
		return nil, fmt.Errorf(
			"Package with compiled Go files is reqired. Syntax files (%d) != Go files (%d)",
			len(pkg.Syntax), len(pkg.CompiledGoFiles),
		)
	}

	var funcsFiles []funcsByFile
	for i, f := range pkg.Syntax {

		var funcNames []string
		for _, fc := range funcCalls {
			// func call must belong to pkg or import it otherwise it cannot call fc
			if pkg.PkgPath != fc.pkg && !astutil.UsesImport(f, fc.pkg) {
				funcNames = []string{}
				break
			}

			if pkg.PkgPath == fc.pkg {
				// this func call belongs to this package so remove the pkg selector
				fc.pkg = ""
			}

			// TODO: could be more optimal when visiting a func defined in f, check
			// if has calls to all funcCalls
			fnames, err := funcsNamesWithCallFunc(f, fc, pkg.TypesInfo)
			if err != nil {
				fname := filepath.Join(pkg.PkgPath, filepath.Base(pkg.CompiledGoFiles[i]))
				return nil, fmt.Errorf("%v. Source file: %s", err, fname)
			}

			// File doesn't have any function which calls fc
			if fnames == nil {
				funcNames = []string{}
				break
			}

			if funcNames == nil {
				funcNames = fnames
			} else {
				funcNames = intersect(funcNames, fnames)
			}
		}

		if len(funcNames) > 0 {
			fname := filepath.Join(pkg.PkgPath, filepath.Base(pkg.CompiledGoFiles[i]))
			funcsFiles = append(funcsFiles, funcsByFile{
				Filename:  fname,
				FuncNames: funcNames,
			})
		}
	}

	return funcsFiles, nil
}

func funcsNamesWithCallFunc(file *ast.File, fnCall funcCall, typesInfo *types.Info) ([]string, error) {
	var funcNames []string
	for _, d := range file.Decls {
		fdecl, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}

		ok, err := hasFuncSetCalls(fdecl.Body, fnCall, file.Imports, typesInfo)
		if err != nil {
			return nil, err
		}

		if ok {
			funcNames = append(funcNames, functionIdentifier(fdecl))
		}
	}

	return funcNames, nil
}

// hasFuncSetCalls return true if fnCall is found in function body. imports are
// the packages imported by the file where fnCall is defined and typesInfo holds
// the type information of the package where fnCall is defined.
//
// fnCall.pkg must be set to empty string if the function is defined in the
// same package that the function to inspect.
func hasFuncSetCalls(
	body *ast.BlockStmt, fnCall funcCall, imports []*ast.ImportSpec, typesInfo *types.Info,
) (bool, error) {
	var (
		found    bool
		errToRet error
	)
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil || found || errToRet != nil {
			return false
		}

		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// it's a function defined in the same package
		if ident, ok := callExpr.Fun.(*ast.Ident); ok {
			if fnCall.pkg == "" && fnCall.receiver == "" && fnCall.funcName == ident.Name {
				found = true
			}

			return false
		}

		sel := callExpr.Fun.(*ast.SelectorExpr)

		// receiver is a field of a struct type
		if selx, ok := sel.X.(*ast.SelectorExpr); ok {
			typ := typesInfo.TypeOf(selx.X)
			if ptyp, ok := typ.(*types.Pointer); ok {
				typ = ptyp.Elem()
			}

			var typeRef string
			structType := typ.Underlying().(*types.Struct)
			for i := 0; i < structType.NumFields(); i++ {
				field := structType.Field(i)
				if field.Name() == selx.Sel.Name {
					typeRef = field.Type().String()
					break
				}
			}

			typeRef = removeStartingStar(typeRef)
			pkg, typName, err := splitPackageAndType(typeRef)
			if err != nil {
				if errToRet != nil {
					errToRet = err
				}

				return false
			}

			if fnCall.pkg == pkg && fnCall.receiver == typName &&
				fnCall.funcName == sel.Sel.Name {
				found = true
			}

			return false
		}

		// receiver is a package or a var
		if ident, ok := sel.X.(*ast.Ident); ok {
			// ident represents local package name
			if ident.Obj == nil {
				for _, imp := range imports {
					if fnCall.pkg == strings.Trim(imp.Path.Value, `"`) &&
						fnCall.funcName == sel.Sel.Name {
						found = true
						return false
					}
				}

				return false
			}

			typeRef := typesInfo.ObjectOf(ident).Type().String()
			typeRef = removeStartingStar(typeRef)
			pkg, typ, err := splitPackageAndType(typeRef)
			if err != nil {
				if errToRet != nil {
					errToRet = err
				}

				return false
			}

			if fnCall.pkg == pkg && fnCall.receiver == typ &&
				fnCall.funcName == sel.Sel.Name {
				found = true
			}

			return false
		}

		return false
	})

	return found, errToRet
}

func functionIdentifier(fdecl *ast.FuncDecl) string {
	id := ""
	if fdecl.Recv != nil {
		t := fdecl.Recv.List[0].Type
		if st, ok := t.(*ast.StarExpr); ok {
			id = "*"
			t = st.X
		}

		id = fmt.Sprintf("%s%s.", id, t.(*ast.Ident).Name)
	}

	return fmt.Sprintf("%s%s", id, fdecl.Name.Name)
}

func intersect(a []string, b []string) []string {
	sort.Strings(a)
	sort.Strings(b)

	shorter, longer := a, b
	if len(a) > len(b) {
		shorter, longer = b, a
	}

	var (
		li           = 0
		intersection = []string{}
	)
	for _, s := range shorter {
		if s < longer[li] {
			continue
		}

		for li < len(longer) && s > longer[li] {
			li++
		}

		if li >= len(longer) {
			break
		}

		if s == longer[li] {
			li++
			intersection = append(intersection, s)
			continue
		}
	}

	return intersection
}

// removeStaringStar removes "*"  from the beginning of val, otherwise returns
// val.
func removeStartingStar(val string) string {
	if len(val) == 0 {
		return val
	}

	if val[0] == '*' {
		return val[1:]
	}

	return val
}

// splitPackateAndType return the package and type parts of the pkgPathType
// value.
//
// pkgPathType cannot be empty and must be of appropriated format, e.g.
// net/http/cookiejar.Jar. The format isn't thoroughly checked, so only a some
// invalid formatting is detected.
func splitPackageAndType(pkgPathType string) (pkgPath, typ string, _ error) {
	if len(pkgPathType) == 0 {
		return "", "", fmt.Errorf(
			"Invalid 'package_path.type' format. Got %s", pkgPathType,
		)
	}

	i := strings.LastIndex(pkgPathType, ".")
	if i <= 0 {
		return "", "", fmt.Errorf(
			"Invalid 'package_path.type' format. Got %s", pkgPathType,
		)
	}

	return pkgPathType[:i], pkgPathType[i+1:], nil
}
