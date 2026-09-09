#!/usr/bin/env bash
# Fail when a board's hand-maintained targets/boards/<board>/filelist.sh has
# fallen behind the file lists that live inside the components/cpu submodule.
#
# WHY THIS EXISTS
# ---------------
# components/cpu owns the authoritative list of the VHDL that makes up the CPU:
# one `$(VHDLS) += <path>` fragment per directory (components/cpu/build_core.mk
# for the core+decode, components/cpu/cache/build_core.mk for the caches). Every
# Make-driven board consumes those fragments directly, via tools/mk_utils.mk.
#
# The two ghdl-script boards (ulx3s, icesugar) do NOT. They re-state the same
# paths by hand in filelist.sh, because they need a real *analyze order* and the
# Make fragments are an unordered SET, not a sequence: this Makefile's
# VHDS_FPGA/VHDS_ASIC `$(sort)` the paths mk_utils.mk collects, and
# components/cpu's own sim hands them to `ghdl -i` (tools/ghdl.mk), which
# imports rather than analyzes and does not care about order. Their `must
# precede` comments are documentation, not a guarantee -- build_core.mk itself
# lists core/cpu.vhd before core/datapath_pkg.vhd, which `ghdl -a` rejects.
# That hand copy is a standing tax, and it has already been paid twice:
#
#   * core/tlb.vhd -- somebody reasoned out that cpu.vhd instantiates work.tlb
#     unconditionally and added it to both filelists, with a comment.
#   * core/perf_pkg.vhd + core/perf.vhd (the PMU) -- nobody repeated that
#     reasoning. cpu.vhd's `use work.perf_pack.all;` is an unconditional context
#     clause, so the moment components/cpu is bumped past the PMU merge, every
#     ULX3S variant stops analyzing. Neither board filelist mentions the PMU.
#     Six more core files and two cache files are in the same state; run this
#     check against a components/cpu bumped to jcore-cpu master to see them all.
#
# This script compares the two and prints the difference. Like
# components/cpu/scripts/guard_list_drift.sh, it does NOT try to merge them --
# a board filelist legitimately carries board VHDL no CPU list knows about, and
# legitimately drops CPU units it never binds -- it only insists that where a
# board draws from a CPU list at all, it draws the whole list, or says in
# writing why not.
#
# THE RULES
# ---------
#   1. Scope is opt-in per list. A board is measured against a given
#      build_core.mk only if its filelist already names at least one file from
#      that list. (iCESugar is EBR-only and analyzes no cache VHDL at all, so
#      cache/build_core.mk simply does not apply to it -- that is a design
#      choice, not drift.)
#   2. Within a list that does apply, every literal path must appear in the
#      board's FILES array -- either written in filelist.sh, or spliced in from
#      the board's soc_gen-generated cpu_synth_files.list.
#   3. `$(VARIABLE)` entries (CPU_EXTRA_FILES, CPU_CONFIG_FILE,
#      CPU_DECODE_GENERATED) are variant-selected and are exactly what
#      cpu_synth_files.list supplies, so they are reported and skipped.
#   4. An exception needs a written reason, in
#      targets/boards/<board>/filelist_waivers. A waiver that names a file the
#      CPU no longer lists, or one the board now analyzes anyway, is itself an
#      error -- so waivers cannot rot in place.
#
# Usage: tools/check_filelist_drift.sh [board ...]     (default: every board
#        under targets/boards/*/ that has a filelist.sh)
#
# Side effect: for boards that stage their synth sources with
# gen_synth_sources.sh (ulx3s), that script is run first, because filelist.sh
# refuses to be sourced without the cpu_synth_files.list it stages. It writes
# only into the board's gitignored generated/ directory.
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT=$PWD
CPU=components/cpu

die () { echo "check_filelist_drift: $*" >&2; exit 2; }

[ -f "$CPU/build_core.mk" ] || die "$CPU/build_core.mk not found -- run: git submodule update --init $CPU"

# ---------------------------------------------------------------------------
# Expand one build_core.mk with GNU make itself, rather than grepping it.
# build_core.mk is a Make fragment using mk_utils.mk's indirect-append idiom
# ($(VHDLS) names the variable to append to), and it already contains a
# $(patsubst ...) entry -- a regex reader would have to re-implement Make to
# stay correct as that file evolves. Undefined variables (CPU_EXTRA_FILES,
# CPU_CONFIG_FILE, CPU_DECODE_GENERATED, CPU_INC_DIR) expand to empty, which is
# what rule 3 wants. Including build_core.mk directly, not build.mk, is
# deliberate: build.mk pulls in Makefile.inc, which needs Go, variants.toml and
# GNU Make 4.3+, and which defines rules that regenerate the decoder.
# ---------------------------------------------------------------------------
mk_entries () {  # <path to build_core.mk> -> one raw entry per line
  local mk=$1 out
  out=$(make --no-print-directory -s -f - <<MK
VHDLS := DRIFT_LIST
DRIFT_LIST :=
include $mk
\$(info \$(DRIFT_LIST))
.PHONY: drift-noop
drift-noop: ; @:
MK
  ) || die "GNU make could not read $mk"
  printf '%s\n' $out
}

# cpu-relative prefix for a build_core.mk: "" for components/cpu/build_core.mk,
# "cache/" for components/cpu/cache/build_core.mk, and so on for any list added
# later -- the discovery below is a find, not a hardcoded pair.
mk_prefix () {
  local d; d=$(dirname "$1")
  [ "$d" = "$CPU" ] && { printf ''; return; }
  printf '%s/' "${d#"$CPU"/}"
}

# ---------------------------------------------------------------------------
# The board's list: source filelist.sh and read the FILES array it defines.
# Sourcing rather than grepping means the check measures the exact argv ghdl is
# handed -- including the cpu_synth_files.list entries filelist.sh splices in at
# run time, which a text scan of filelist.sh would miss entirely.
# ---------------------------------------------------------------------------
board_files () {  # <board> -> one path per line, as passed to ghdl
  local b=$1
  (
    set -euo pipefail
    cd "$ROOT"
    if [ -f "targets/boards/$b/gen_synth_sources.sh" ]; then
      bash "targets/boards/$b/gen_synth_sources.sh" >/dev/null
    fi
    # shellcheck disable=SC1090
    source "targets/boards/$b/filelist.sh"
    printf '%s\n' "${FILES[@]}"
  ) || die "could not source targets/boards/$b/filelist.sh"
}

mapfile -t MK_FILES < <(find "$CPU" -name build_core.mk | sort)
[ "${#MK_FILES[@]}" -gt 0 ] || die "no build_core.mk found under $CPU"

if [ "$#" -gt 0 ]; then
  BOARDS=("$@")
else
  mapfile -t BOARDS < <(for f in targets/boards/*/filelist.sh; do
                          [ -e "$f" ] && basename "$(dirname "$f")"
                        done | sort)
fi
[ "${#BOARDS[@]}" -gt 0 ] || die "no boards with a filelist.sh"

rc=0
for board in "${BOARDS[@]}"; do
  [ -f "targets/boards/$board/filelist.sh" ] || die "no such board filelist: targets/boards/$board/filelist.sh"

  # Everything the board hands ghdl that lives inside components/cpu, made
  # cpu-relative. Board VHDL (targets/, lib/, components/uartlite/, ...) is out
  # of scope by construction: no CPU list claims it.
  # Captured into a variable first, not straight into a process substitution:
  # board_files' own die() would exit only the substituted subshell, and this
  # check would then silently measure an empty board list as "no drift".
  board_raw=$(board_files "$board")
  declare -A board_cpu=()
  while read -r f; do
    if [ -n "$f" ]; then board_cpu["$f"]=1; fi
  done < <(printf '%s\n' "$board_raw" | sed -n "s|^$CPU/||p")
  [ "${#board_cpu[@]}" -gt 0 ] || die "targets/boards/$board/filelist.sh names no $CPU/ file at all"
  in_board () { [ -n "${board_cpu[$1]+x}" ]; }

  # Waivers: "<cpu-relative path><whitespace><reason>". A reason is mandatory --
  # the whole failure this guards against is a missing piece of reasoning.
  waiver_file="targets/boards/$board/filelist_waivers"
  declare -A waived=()
  if [ -f "$waiver_file" ]; then
    lineno=0
    while IFS= read -r line || [ -n "$line" ]; do
      lineno=$((lineno + 1))
      case "$line" in ''|'#'*) continue ;; esac
      path=${line%%[[:space:]]*}
      reason=${line#"$path"}
      reason=${reason#"${reason%%[![:space:]]*}"}
      if [ -z "$reason" ]; then
        echo "ERROR: $waiver_file:$lineno: '$path' is waived with no reason." >&2
        echo "       Write why this board does not analyze it." >&2
        rc=1
        continue
      fi
      waived["$path"]=$reason
    done < "$waiver_file"
  fi

  echo "== $board =="
  declare -A cpu_lists=()
  for mk in "${MK_FILES[@]}"; do
    prefix=$(mk_prefix "$mk")

    literals=(); variables=()
    while read -r e; do
      [ -n "$e" ] || continue
      case "$e" in
        *'$('*) variables+=("$e") ;;
        *)      literals+=("$prefix$e"); cpu_lists["$prefix$e"]=$mk ;;
      esac
    done < <(mk_entries "$mk")

    # Rule 1: opt-in scope.
    applies=no
    for f in "${literals[@]}"; do in_board "$f" && { applies=yes; break; }; done
    if [ "$applies" = no ]; then
      echo "   $mk: not drawn from by this board (${#literals[@]} files) -- skipped"
      continue
    fi

    missing=()
    for f in "${literals[@]}"; do
      in_board "$f" && continue
      [ -n "${waived[$f]+x}" ] && continue
      missing+=("$f")
    done

    if [ "${#missing[@]}" -gt 0 ]; then
      echo "" >&2
      echo "DRIFT: targets/boards/$board/filelist.sh does not analyze ${#missing[@]} file(s)" >&2
      echo "       that $mk lists as part of the CPU:" >&2
      for f in "${missing[@]}"; do echo "         $CPU/$f" >&2; done
      rc=1
    else
      note=""
      [ "${#variables[@]}" -gt 0 ] && note=" (+${#variables[@]} variant-selected \$(VAR) entries, supplied by cpu_synth_files.list -- not compared)"
      echo "   $mk: ${#literals[@]} file(s), all analyzed$note"
    fi
  done

  # Rule 4: waivers may not rot.
  for f in "${!waived[@]}"; do
    if [ -z "${cpu_lists[$f]+x}" ]; then
      echo "ERROR: $waiver_file waives '$f', which no components/cpu build_core.mk lists any more." >&2
      echo "       Delete the waiver." >&2
      rc=1
    elif in_board "$f"; then
      echo "ERROR: $waiver_file waives '$f', but filelist.sh analyzes it anyway." >&2
      echo "       Delete the waiver." >&2
      rc=1
    fi
  done
  unset waived cpu_lists board_cpu
done

if [ "$rc" != 0 ]; then
  cat >&2 <<'EOF'

A components/cpu file list names VHDL that a board's filelist.sh does not
analyze. That is the bug this check exists to catch, and how it announces
itself depends on how the CPU refers to the file -- measured against
jcore-cpu e8a5a4e, by analyzing the ULX3S core block with one file removed:

  * removing core/perf_pkg.vhd -> "unit perf_pack not found in library work"
    at core/datapath_pkg.vhd:8 (a package behind an unconditional use clause);
  * removing core/perf.vhd     -> "unit perf not found in library work" at
    core/cpu.vhd:810 (a DIRECT entity instantiation, `entity work.perf`,
    still fatal even though it sits inside the PRIV_ARCH generate);
  * removing core/divider.vhd  -> NOTHING. ghdl -a exits 0. `u_div` is a
    COMPONENT instantiation, so the component declaration in divider_pkg.vhd
    satisfies the analyzer and --syn-binding then leaves the divider an
    unbound black box -- silently absent hardware, the same failure mode
    synth/cpu_synth_config.vhd's u_mult binding exists to prevent.

The first two are loud, but only after a full soc_gen + boot-rom + v2p run has
paid for the privilege. The third is not loud at all. Hence this check.

Fix it in one of two ways:
  * add the file to targets/boards/<board>/filelist.sh, in an order that
    satisfies its `use` clauses (the CPU lists are an unordered set -- they
    cannot tell you where); or
  * if this board genuinely never binds it, add a line to
    targets/boards/<board>/filelist_waivers saying so, and why.
EOF
  exit 1
fi

echo "OK: no drift between components/cpu's file lists and the board filelists"
