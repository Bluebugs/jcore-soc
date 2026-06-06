package elaborate

import (
	"testing"

	"github.com/j-core/jcore-soc/tools/socgen/design"
)

func TestMatchPinRegexAndParametric(t *testing.T) {
	// regex full-match
	r := &design.PinRule{Match: &design.Match{Regex: "uart_tx"}}
	if env, ok := matchPin(r, "uart_tx"); !ok || len(env) != 0 {
		t.Errorf("regex match: ok=%v env=%v", ok, env)
	}
	if _, ok := matchPin(r, "uart_tx2"); ok {
		t.Error("regex should full-match (uart_tx2 must not match uart_tx)")
	}
	// parametric [mcb3_dram_a, n]
	p := &design.PinRule{Match: &design.Match{Parts: []design.SeqPart{{Lit: "mcb3_dram_a"}, {Sym: "n"}}}}
	env, ok := matchPin(p, "mcb3_dram_a12")
	if !ok || env["n"] != 12 {
		t.Errorf("parametric: ok=%v env=%v", ok, env)
	}
	// multi-part [io_p, n, "_", m]
	mp := &design.PinRule{Match: &design.Match{Parts: []design.SeqPart{{Lit: "io_p"}, {Sym: "n"}, {Lit: "_"}, {Sym: "m"}}}}
	env, ok = matchPin(mp, "io_p3_7")
	if !ok || env["n"] != 3 || env["m"] != 7 {
		t.Errorf("multi-part: ok=%v env=%v", ok, env)
	}
}

func TestExpandSig(t *testing.T) {
	env := map[string]int{"n": 5}
	tmpl := &design.SigSpec{Kind: design.SigTemplate, Parts: []design.SeqPart{{Lit: "dr_data_o.dqo("}, {Sym: "n"}, {Lit: ")"}}}
	ref, diff, kind := expandSig(tmpl, "thepin", env)
	if ref != "dr_data_o.dqo(5)" || diff != "" || kind != design.SigName {
		t.Errorf("template: ref=%q diff=%q kind=%v", ref, diff, kind)
	}
	if ref, _, _ := expandSig(&design.SigSpec{Kind: design.SigTrue}, "thepin", env); ref != "thepin" {
		t.Errorf("true: %q", ref)
	}
	if ref, diff, _ := expandSig(&design.SigSpec{Kind: design.SigMap, Name: "ddr_clk", Diff: "pos"}, "p", env); ref != "ddr_clk" || diff != "pos" {
		t.Errorf("map: ref=%q diff=%q", ref, diff)
	}
	if _, _, kind := expandSig(&design.SigSpec{Kind: design.SigConst, Int: 0}, "p", env); kind != design.SigConst {
		t.Errorf("const kind=%v", kind)
	}
}

func TestFoldRulesAttrsLastWins(t *testing.T) {
	rules := []*design.PinRule{
		{Match: &design.Match{Regex: ".*"}, Attrs: map[string]design.Value{"iostandard": {Kind: design.KindExpr, Text: "LVCMOS33"}}},
		{Match: &design.Match{Regex: "mcb3_dram_ck"}, Attrs: map[string]design.Value{"iostandard": {Kind: design.KindExpr, Text: "DIFF_MOBILE_DDR"}}},
		{Match: &design.Match{Regex: "mcb3_dram_ck"}, Signal: &design.SigSpec{Kind: design.SigMap, Name: "ddr_clk", Diff: "pos"}},
	}
	f := foldRules(rules, &design.Pin{Net: "mcb3_dram_ck", Pad: "E3"})
	if f.attrs["iostandard"].Text != "DIFF_MOBILE_DDR" {
		t.Errorf("attrs: %+v", f.attrs)
	}
	if f.signalRef != "ddr_clk" || f.signalDiff != "pos" {
		t.Errorf("signal: ref=%q diff=%q", f.signalRef, f.signalDiff)
	}
}
