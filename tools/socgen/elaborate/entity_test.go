package elaborate

import (
	"testing"

	"github.com/j-core/jcore-soc/tools/socgen/design"
)

func TestChooseArchSingle(t *testing.T) {
	lib := buildLib(t,
		`entity e is port (clk : in std_logic); end entity;`,
		`architecture rtl of e is begin end architecture;`)
	ent, arch, cfg, hardErr, errs := chooseArch(`class "e"`, "e", "", "", lib, nil)
	if len(errs) != 0 {
		t.Fatalf("errs: %v", errs)
	}
	if ent == nil || arch != "rtl" || cfg != nil || hardErr {
		t.Fatalf("got ent=%v arch=%q cfg=%v hardErr=%v", ent, arch, cfg, hardErr)
	}
}

func TestChooseArchEntityNotFound(t *testing.T) {
	lib := buildLib(t, `entity e is port (clk : in std_logic); end entity;`,
		`architecture rtl of e is begin end architecture;`)
	ent, _, _, hardErr, errs := chooseArch(`class "ghost"`, "ghost", "", "", lib, nil)
	if ent != nil || !hardErr || len(errs) != 1 {
		t.Fatalf("expected hard error for missing entity: ent=%v hardErr=%v errs=%v", ent, hardErr, errs)
	}
}

func TestChooseArchAmbiguousIsSoft(t *testing.T) {
	// two architectures, none named -> error but hardErr=false (faithful: falls through)
	lib := buildLib(t,
		`entity e is port (clk : in std_logic); end entity;`,
		`architecture a1 of e is begin end architecture;`,
		`architecture a2 of e is begin end architecture;`)
	ent, arch, _, hardErr, errs := chooseArch(`class "e"`, "e", "", "", lib, nil)
	if ent == nil || arch != "" || hardErr || len(errs) != 1 {
		t.Fatalf("expected soft ambiguity: ent=%v arch=%q hardErr=%v errs=%v", ent, arch, hardErr, errs)
	}
	_ = design.KindExpr // keep design import used across the file
}
