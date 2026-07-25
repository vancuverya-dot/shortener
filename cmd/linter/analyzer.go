package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

const doc = `check for panic usage and for log.Fatal/os.Exit calls outside main

noexit reports two kinds of issues:
  - any use of the built-in panic function;
  - calls to log.Fatal, log.Fatalf, log.Fatalln, or os.Exit made anywhere
    except directly inside the main function of package main.`

// Analyzer — статический анализатор, запускаемый через singlechecker.
var Analyzer = &analysis.Analyzer{
	Name: "noexit",
	Doc:  doc,
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	isMainPkg := pass.Pkg.Name() == "main"

	for _, file := range pass.Files {
		var stack []ast.Node

		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)

			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			if isPanicCall(pass, call) {
				pass.Reportf(call.Pos(), "use of panic is not allowed")
				return true
			}

			if name, ok := fatalCallName(pass, call); ok && !inMain(stack, isMainPkg) {
				pass.Reportf(call.Pos(), "call to %s is not allowed outside main function of package main", name)
			}

			return true
		})
	}

	return nil, nil
}

// isPanicCall сообщает, является ли call вызовом встроенной функции panic
// (а не пользовательской функции/переменной с тем же именем).
func isPanicCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	b, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	return ok && b.Name() == "panic"
}

// fatalCallName сообщает, является ли call вызовом log.Fatal/Fatalf/Fatalln
// или os.Exit из настоящих пакетов log/os, и возвращает строковое имя вызова
// вида "log.Fatal" для сообщения об ошибке.
func fatalCallName(pass *analysis.Pass, call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	pkgName, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	if !ok {
		return "", false
	}

	path := pkgName.Imported().Path()
	switch {
	case path == "log" && (sel.Sel.Name == "Fatal" || sel.Sel.Name == "Fatalf" || sel.Sel.Name == "Fatalln"):
		return "log." + sel.Sel.Name, true
	case path == "os" && sel.Sel.Name == "Exit":
		return "os.Exit", true
	}
	return "", false
}

// inMain сообщает, лежит ли текущий узел (верхушка стека предков) внутри
// объявления функции main пакета main. Поскольку в Go нет вложенных
// объявлений функций (только замыкания через ast.FuncLit), в стеке
// предков может быть не более одного *ast.FuncDecl — это и есть
// ближайшая именованная функция, объемлющая текущий вызов.
func inMain(stack []ast.Node, isMainPkg bool) bool {
	if !isMainPkg {
		return false
	}
	for _, n := range stack {
		if fd, ok := n.(*ast.FuncDecl); ok {
			return fd.Recv == nil && fd.Name.Name == "main"
		}
	}
	return false
}
