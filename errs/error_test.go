package errs

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func Test_ConvertChecksAllConstsInSwitch(t *testing.T) {
	declaredNames := []string{}
	switchCaseNames := []string{}

	// First find all of the names of declared variables in error_codes.go.
	// This will obviously not work if we instantiate varibles of type
	// InternalErrorCode elsewhere. However, if we keep all of these variables
	// to that file (and no other variables) then this list will be comprehensive.
	fset := token.NewFileSet() // positions are relative to the file set.
	f, err := parser.ParseFile(fset, "error_codes.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if x.Obj != nil && x.Obj.Kind == ast.Con {
				obj := x.Obj
				d := obj.Decl
				if declAsValue, ok := d.(*ast.ValueSpec); ok {
					if len(declAsValue.Names) > 0 {
						declaredNames = append(declaredNames, declAsValue.Names[0].Name)
						return true
					}
				}
			}
		}
		return true
	})

	// Now find all of the case checks in the switch statement in `error.go`.
	// Keep in mind this will break if there is more than one switch statement
	// in that file. This is pretty hacky :-).
	fset = token.NewFileSet() 
	f, err = parser.ParseFile(fset, "error.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CaseClause:
			if len(x.List) > 0 {
				if listElemAsIdent, ok := x.List[0].(*ast.Ident); ok {
					switchCaseNames = append(switchCaseNames, listElemAsIdent.Name)
				}
			}
		}
		return true
	})

	Convey("Switch statement cases should be the same as declared names", t, func() {
		sort.Strings(declaredNames)
		sort.Strings(switchCaseNames)

		So(declaredNames, ShouldResemble, switchCaseNames)
	})
}