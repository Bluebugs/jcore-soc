package vhdl

import "testing"

func TestPrintReparseStable(t *testing.T) {
	srcs := []string{
		"package p is\n  constant C : integer := 5;\nend package;",
		"entity e is\n  port (clk : in std_logic;\n        d : out std_logic_vector(31 downto 0));\nend entity;",
		"package q is\n  type rec_t is record\n    a : std_logic;\n    b : std_logic_vector(7 downto 0);\n  end record;\nend package;",
	}
	for _, s := range srcs {
		p1 := newParser([]byte(s))
		f1 := p1.ParseFile()
		if len(p1.errs) != 0 {
			t.Fatalf("parse1 %q: %v", s, p1.errs)
		}
		out := Print(f1)
		p2 := newParser([]byte(out))
		f2 := p2.ParseFile()
		if len(p2.errs) != 0 {
			t.Fatalf("reparse %q -> %q: %v", s, out, p2.errs)
		}
		if !equalAST(f1, f2) {
			t.Fatalf("AST changed across round-trip:\nin:  %q\nout: %q", s, out)
		}
	}
}
