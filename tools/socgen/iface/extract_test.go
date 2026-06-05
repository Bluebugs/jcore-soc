package iface

import (
	"testing"

	"github.com/j-core/jcore-soc/tools/socgen/vhdl"
)

func parse(t *testing.T, src string) *vhdl.DesignFile {
	t.Helper()
	df, errs := vhdl.ParseFile(vhdl.NewFileSet(), "t.vhd", []byte(src))
	if len(errs) != 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	return df
}

func TestExtractEntity(t *testing.T) {
	df := parse(t, `entity uart is
  generic (width : integer := 8; fast : boolean);
  port (clk, rst : in std_logic;
        data : out std_logic_vector(15 downto 0));
end entity;`)
	lib, errs := Extract([]*vhdl.DesignFile{df})
	if len(errs) != 0 {
		t.Fatalf("extract errors: %v", errs)
	}
	e, ok := lib.Entity("uart")
	if !ok {
		t.Fatal("entity uart not found")
	}
	if len(e.Generics) != 2 {
		t.Fatalf("generics: got %d want 2", len(e.Generics))
	}
	if e.Generics[0].Name != "width" || e.Generics[0].Type.String() != "integer" {
		t.Errorf("generic0 = %q %q", e.Generics[0].Name, e.Generics[0].Type.String())
	}
	if e.Generics[0].Default == nil {
		t.Error("generic0 default should be non-nil")
	}
	if e.Generics[1].Name != "fast" || e.Generics[1].Default != nil {
		t.Errorf("generic1 = %q default=%v", e.Generics[1].Name, e.Generics[1].Default)
	}
	if len(e.Ports) != 3 {
		t.Fatalf("ports: got %d want 3", len(e.Ports))
	}
	if e.Ports[0].Name != "clk" || e.Ports[0].Dir != "in" || e.Ports[0].Type.String() != "std_logic" {
		t.Errorf("port0 = %+v (%s)", e.Ports[0], e.Ports[0].Type.String())
	}
	if e.Ports[1].Name != "rst" || e.Ports[1].Dir != "in" {
		t.Errorf("port1 = %+v", e.Ports[1])
	}
	if e.Ports[2].Name != "data" || e.Ports[2].Dir != "out" ||
		e.Ports[2].Type.String() != "std_logic_vector(15 downto 0)" {
		t.Errorf("port2 = %+v (%s)", e.Ports[2], e.Ports[2].Type.String())
	}
}

func TestExtractArchitecture(t *testing.T) {
	df := parse(t, `architecture rtl of uart is begin end architecture;`)
	lib, errs := Extract([]*vhdl.DesignFile{df})
	if len(errs) != 0 {
		t.Fatalf("extract errors: %v", errs)
	}
	archs := lib.ArchitecturesOf("uart")
	if len(archs) != 1 {
		t.Fatalf("architectures of uart: got %d want 1", len(archs))
	}
	if archs[0].Name != "rtl" || archs[0].Entity != "uart" || archs[0].Node == nil {
		t.Errorf("arch = %+v", archs[0])
	}
}

func TestExtractPackage(t *testing.T) {
	df := parse(t, `package bus_pkg is
  constant WIDTH : integer := 32;
  constant DEFERRED : integer;
  subtype byte is std_logic_vector(7 downto 0);
  type state_t is (idle, run, stop);
  component fifo is
    generic (depth : integer);
    port (clk : in std_logic; full : out std_logic);
  end component;
end package;`)
	lib, errs := Extract([]*vhdl.DesignFile{df})
	if len(errs) != 0 {
		t.Fatalf("extract errors: %v", errs)
	}
	p, ok := lib.Package("bus_pkg")
	if !ok {
		t.Fatal("package bus_pkg not found")
	}
	if len(p.Constants) != 2 {
		t.Fatalf("constants: got %d want 2", len(p.Constants))
	}
	if p.Constants[0].Name != "WIDTH" || p.Constants[0].Type.String() != "integer" || p.Constants[0].Value == nil {
		t.Errorf("const0 = %+v", p.Constants[0])
	}
	if p.Constants[1].Name != "DEFERRED" || p.Constants[1].Value != nil {
		t.Errorf("const1 should be deferred: %+v", p.Constants[1])
	}
	if len(p.Types) != 2 { // subtype byte + type state_t
		t.Fatalf("types: got %d want 2", len(p.Types))
	}
	if len(p.Components) != 1 || p.Components[0].Name != "fifo" {
		t.Fatalf("components: %+v", p.Components)
	}
	if len(p.Components[0].Ports) != 2 || p.Components[0].Ports[1].Name != "full" || p.Components[0].Ports[1].Dir != "out" {
		t.Errorf("component ports: %+v", p.Components[0].Ports)
	}
	if te, ok := lib.ResolveType("byte"); !ok || te.Name != "byte" {
		t.Errorf("ResolveType(byte) = %v %v", te, ok)
	}
	if te, ok := lib.ResolveType("state_t"); !ok || te.Name != "state_t" {
		t.Errorf("ResolveType(state_t) = %v %v", te, ok)
	}
}
