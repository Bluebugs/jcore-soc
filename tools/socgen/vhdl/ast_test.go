package vhdl

import "testing"

func TestASTConstructs(t *testing.T) {
	f := &DesignFile{Units: []DesignUnit{
		&PackageDecl{Name: "p", Decls: []Decl{
			&ConstantDecl{Names: []string{"c"}, SubtypeMark: "integer", Default: &Lit{Text: "0"}},
		}},
	}}
	if len(f.Units) != 1 {
		t.Fatal("units")
	}
	pkg := f.Units[0].(*PackageDecl)
	if pkg.Name != "p" || len(pkg.Decls) != 1 {
		t.Fatal("pkg shape")
	}
}
