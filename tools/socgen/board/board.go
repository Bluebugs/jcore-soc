package board

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/j-core/jcore-soc/tools/socgen/design"
	"github.com/j-core/jcore-soc/tools/socgen/iface"
)

// Board is a loaded + validated board: its parsed spec and the interface
// Library extracted from its full VHDL file set.
type Board struct {
	Name    string
	Design  *design.Design
	Library *iface.Library
}

var vhdlExt = regexp.MustCompile(`\.vh[hd]$`)

// readFileList reads a vhdl_list.txt (one path per line) and returns the lines
// naming a .vhd/.vhh file. Separated from Files so it is unit-testable.
func readFileList(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file list %s: %w", path, err)
	}
	var out []string
	for _, ln := range strings.Split(string(data), "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" && vhdlExt.MatchString(ln) {
			out = append(out, ln)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty file list %s", path)
	}
	return out, nil
}

// loadFrom composes spec-load + library-build + validate for a board whose VHDL
// file set is already known (no make). Load (Task 2) wraps it with Files.
func loadFrom(root, name string, files []string) (*Board, []error) {
	d, derrs := design.Load(filepath.Join(root, "targets", "boards", name, "design.yaml"))
	lib, lerrs := Library(files)
	errs := append(append([]error{}, derrs...), lerrs...)
	if d != nil {
		errs = append(errs, design.Validate(d, lib)...)
	}
	return &Board{Name: name, Design: d, Library: lib}, errs
}
