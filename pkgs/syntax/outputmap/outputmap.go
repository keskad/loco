// Package outputmap parses the RB23xx AUX output mapping file format.
//
// Each non-comment line has the form:
//
//	O<num>:<Fx><dir>
//
// where <dir> is either ">" (driving direction A) or "<" (opposite direction B).
// A missing direction (e.g. "O11:F5") is accepted; the entry is stored with
// DirNone and is ignored during light classification.
//
// # Function role assignment
//
// The function numbers for Pc1/Pc2/Pc5/Tb1/cabin are NOT fixed – they differ
// between locomotive models.  The parser detects them from comment lines of the
// form:
//
//	# Pc1, … (F0)
//	# Pc5, Czerwone, tylnie kierunkowe (F7)
//	# Tb1 (F16)
//	# Kabina (F8)
//
// i.e. a parenthesised token "(F<n>)" in a comment that is preceded somewhere
// on the same line by a recognised role keyword.  When a role is not mentioned
// in any comment the classic defaults are used as a fallback:
//
//	Pc1 → F0, Pc2 → F5, Tb1 → F6, Pc5 → F7, cabin → F8
package outputmap

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// ErrMicrocontrollerBoard is returned by Parse when the mapping file indicates
// that the board uses an on-board microcontroller to handle lighting: the
// primary Pc1 function (F0) drives exactly one output per direction (1×DirA +
// 1×DirB).  In that case the individual AUX outputs are not independently
// configurable and printing a light summary makes no sense.
var ErrMicrocontrollerBoard = errors.New("lighting outputs are not independently configurable (microcontroller board detected)")

// Direction represents the driving direction context of an output mapping.
type Direction string

const (
	// DirA is the forward / driving direction (">").
	DirA Direction = "A"
	// DirB is the reverse / opposite direction ("<").
	DirB Direction = "B"
	// DirNone means the mapping line carried no direction suffix.
	DirNone Direction = ""
)

// OutputEntry describes a single Ox:Fy<dir> mapping line.
type OutputEntry struct {
	Output    uint8     // AUX output number
	Function  uint8     // function number
	Direction Direction // A, B, or "" (none)
}

// FunctionRoles maps semantic roles to the actual function numbers found in the
// mapping file.  A value of 255 means "not detected / not present".
type FunctionRoles struct {
	Pc1      uint8   // white front lights        (default F0)
	Pc2      uint8   // wrong-track mixed lights  (default F5)
	Tb1      uint8   // shunting white lights     (default F6)
	Pc5      uint8   // rear red tail lights      (default F7)
	Pc5Extra []uint8 // additional red tail functions (e.g. F27)
	Cabin    uint8   // driver's cabin light      (default F8)
}

const roleNotFound uint8 = 255

// defaults returns a FunctionRoles filled with the classic default values.
func defaults() FunctionRoles {
	return FunctionRoles{Pc1: 0, Pc2: 5, Tb1: 6, Pc5: 7, Cabin: 8}
}

// OutputMap is the result of parsing a full mapping file.
type OutputMap struct {
	Entries []OutputEntry
	Roles   FunctionRoles
}

// reRoleComment matches a comment line that associates a role keyword with a
// function number, e.g.:
//
//	# Pc1, Biale, przednie, kierunkowe (F0)
//	# Kabina (F8)
//	# Tb1 (F16)
//	# Pc5 (F7)(F27)
//
// All parenthesised "(F<n>)" tokens on the same line are captured; for roles
// that support multiple functions (Pc5) each token becomes a separate entry.
var reRoleComment = regexp.MustCompile(`(?i)(pc1|pc2|pc5|pc6|tb1|cabin|kabina)[^(]*\(F(\d+)\)`)
var reFnToken = regexp.MustCompile(`(?i)\(F(\d+)\)`)

// Parse reads a mapping file from r and returns an OutputMap.
// Lines starting with "#" are inspected for role declarations before being
// skipped as comments.  Blank lines are silently ignored.
func Parse(r io.Reader) (*OutputMap, error) {
	m := &OutputMap{
		Roles: defaults(),
	}
	detected := map[string][]uint8{} // role keyword (lower) → list of fn numbers

	scanner := bufio.NewScanner(r)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		if line == "" {
			continue
		}

		// ---- comment lines: scan for role declarations, then skip ----------
		if strings.HasPrefix(line, "#") {
			if roleMatch := reRoleComment.FindStringSubmatch(line); roleMatch != nil {
				role := strings.ToLower(roleMatch[1])
				// collect ALL (Fxx) tokens from this line for this role
				for _, fnMatch := range reFnToken.FindAllStringSubmatch(line, -1) {
					fn, _ := strconv.ParseUint(fnMatch[1], 10, 8)
					detected[role] = appendUniqUint8(detected[role], uint8(fn))
				}
			}
			continue
		}

		// strip inline comment
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		entry, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		m.Entries = append(m.Entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// ---- apply detected roles (override defaults where found) ---------------
	applyDetected(&m.Roles, detected)

	// ---- reject boards where F0 is driven by a microcontroller -------------
	if err := checkMicrocontrollerBoard(m); err != nil {
		return nil, err
	}

	// ---- auto-detect additional Pc5 functions (no-comment files) -----------
	autoDetectPc5Extra(m)

	return m, nil
}

// checkMicrocontrollerBoard returns ErrMicrocontrollerBoard when the mapping
// file indicates a board with an on-board lighting microcontroller.  The
// heuristic: the Pc1 function (F0 by default, or whatever was detected from
// comments) has exactly 1 DirA entry and exactly 1 DirB entry – which means
// the decoder simply switches the whole front/rear group together and the
// individual outputs are not independently addressable.
func checkMicrocontrollerBoard(m *OutputMap) error {
	pc1Entries := entriesForFn(m.Entries, m.Roles.Pc1)
	countA := len(entriesWithDir(pc1Entries, DirA))
	countB := len(entriesWithDir(pc1Entries, DirB))
	if countA == 1 && countB == 1 {
		return ErrMicrocontrollerBoard
	}
	return nil
}

// autoDetectPc5Extra finds function numbers not yet assigned to any role whose
// directional structure matches the primary Pc5 function: same count of DirA
// entries and same count of DirB entries.  This handles mapping files with no
// comments that list additional red-light functions (e.g. F27 alongside F7).
func autoDetectPc5Extra(m *OutputMap) {
	r := &m.Roles

	// build the set of already-known function numbers so we don't re-classify them
	known := map[uint8]bool{
		r.Pc1:   true,
		r.Pc2:   true,
		r.Tb1:   true,
		r.Pc5:   true,
		r.Cabin: true,
	}
	for _, fn := range r.Pc5Extra {
		known[fn] = true
	}

	// count directional entries for primary Pc5
	pc5CountA := len(entriesWithDir(entriesForFn(m.Entries, r.Pc5), DirA))
	pc5CountB := len(entriesWithDir(entriesForFn(m.Entries, r.Pc5), DirB))

	if pc5CountA == 0 && pc5CountB == 0 {
		return // primary Pc5 has no directional entries → nothing to match against
	}

	// index all entries by function
	byFn := make(map[uint8][]OutputEntry)
	for _, e := range m.Entries {
		byFn[e.Function] = append(byFn[e.Function], e)
	}

	for fn, entries := range byFn {
		if known[fn] {
			continue
		}
		candA := len(entriesWithDir(entries, DirA))
		candB := len(entriesWithDir(entries, DirB))

		// A function is a Pc5Extra candidate when it has the same number of
		// DirA and DirB entries as the primary Pc5 function and no DirNone entries.
		dirNoneCount := len(entriesWithDir(entries, DirNone))
		if dirNoneCount > 0 {
			continue // entries without direction are not red-tail indicators
		}
		if candA == pc5CountA && candB == pc5CountB {
			r.Pc5Extra = appendUniqUint8(r.Pc5Extra, fn)
		}
	}
}

// entriesForFn returns all entries matching the given function number.
func entriesForFn(entries []OutputEntry, fn uint8) []OutputEntry {
	var out []OutputEntry
	for _, e := range entries {
		if e.Function == fn {
			out = append(out, e)
		}
	}
	return out
}

// isSubset returns true when every key in sub is also present in super.
func isSubset(sub, super map[uint8]bool) bool {
	for k := range sub {
		if !super[k] {
			return false
		}
	}
	return true
}

// applyDetected copies detected role→fn mappings into roles, overriding defaults.
func applyDetected(roles *FunctionRoles, detected map[string][]uint8) {
	if fns, ok := detected["pc1"]; ok && len(fns) > 0 {
		roles.Pc1 = fns[0]
	}
	if fns, ok := detected["pc2"]; ok && len(fns) > 0 {
		roles.Pc2 = fns[0]
	}
	if fns, ok := detected["pc5"]; ok && len(fns) > 0 {
		roles.Pc5 = fns[0]
		roles.Pc5Extra = fns[1:] // any additional (F27), (F28), … on the same line
	}
	if fns, ok := detected["pc6"]; ok && len(fns) > 0 {
		// Pc6 is treated as an additional red-indicator; map it onto Pc5 slot
		// only when Pc5 was not explicitly found in comments.
		if _, hasPc5 := detected["pc5"]; !hasPc5 {
			roles.Pc5 = fns[0]
			roles.Pc5Extra = fns[1:]
		}
	}
	if fns, ok := detected["tb1"]; ok && len(fns) > 0 {
		roles.Tb1 = fns[0]
	}
	if fns, ok := detected["cabin"]; ok && len(fns) > 0 {
		roles.Cabin = fns[0]
	}
	if fns, ok := detected["kabina"]; ok && len(fns) > 0 {
		roles.Cabin = fns[0]
	}
}

// appendUniqUint8 appends v to s only if s does not already contain v.
func appendUniqUint8(s []uint8, v uint8) []uint8 {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

// parseLine parses a single "O<n>:F<m>[<dir>]" token.
// A missing direction suffix is accepted (stored as DirNone).
func parseLine(line string) (OutputEntry, error) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return OutputEntry{}, fmt.Errorf("expected 'O<n>:F<m>[><]', got %q", line)
	}

	// --- output number ---
	outStr := strings.TrimSpace(parts[0])
	if !strings.HasPrefix(strings.ToUpper(outStr), "O") {
		return OutputEntry{}, fmt.Errorf("output token must start with 'O', got %q", outStr)
	}
	outNum, err := strconv.ParseUint(outStr[1:], 10, 8)
	if err != nil {
		return OutputEntry{}, fmt.Errorf("invalid output number in %q: %w", outStr, err)
	}

	// --- function + optional direction ---
	fnStr := strings.TrimSpace(parts[1])
	if len(fnStr) < 2 {
		return OutputEntry{}, fmt.Errorf("function token too short: %q", fnStr)
	}
	if !strings.HasPrefix(strings.ToUpper(fnStr), "F") {
		return OutputEntry{}, fmt.Errorf("function token must start with 'F', got %q", fnStr)
	}

	// detect optional direction suffix
	var dir Direction
	fnBody := fnStr[1:] // strip leading 'F'
	switch fnBody[len(fnBody)-1] {
	case '>':
		dir = DirA
		fnBody = fnBody[:len(fnBody)-1]
	case '<':
		dir = DirB
		fnBody = fnBody[:len(fnBody)-1]
	default:
		dir = DirNone
	}

	fnNum, err := strconv.ParseUint(fnBody, 10, 8)
	if err != nil {
		return OutputEntry{}, fmt.Errorf("invalid function number in %q: %w", fnStr, err)
	}

	return OutputEntry{
		Output:    uint8(outNum),
		Function:  uint8(fnNum),
		Direction: dir,
	}, nil
}

// ---- classification ----------------------------------------------------------------

// OutputSummary is the human-readable result of Classify.
type OutputSummary struct {
	WhiteA []uint8 // white lights visible from side A
	WhiteB []uint8 // white lights visible from side B
	RedA   []uint8 // red lights visible from side A
	RedB   []uint8 // red lights visible from side B
	// UnknownA / UnknownB hold outputs whose colour could not be determined
	// (e.g. only F0 is present in the file without F5/F6/F7).
	UnknownA []uint8
	UnknownB []uint8
	// CabinEntries keeps the raw entries for cabin outputs so that direction
	// information is preserved in the printed output.
	CabinEntries []OutputEntry

	// Strategy records which detection path was taken, for diagnostic output.
	Strategy string
}

// Classify analyses the parsed map and returns an OutputSummary.
//
// Detection strategy (tried in order, first that yields white outputs wins):
//
//  1. Tb1 (shunting, default F6) present → fully defines white outputs.
//     Red candidates come from Pc2 outputs absent from the Tb1 set.
//
//  2. Tb1 absent, Pc1 + Pc2 present → Pc1 defines whites, Pc2 identifies reds
//     (outputs in Pc2 that also appear in Pc1 are moved to the red list).
//
//  3. Tb1 absent, Pc2 absent, only Pc1 present → outputs listed as
//     "unknown colour" (side is still known).
//
// Pc5 (tail red) and cabin are always extracted regardless of strategy.
func (m *OutputMap) Classify() OutputSummary {
	r := m.Roles

	// ---- index entries by function number -----------------------------------
	byFn := make(map[uint8][]OutputEntry)
	for _, e := range m.Entries {
		byFn[e.Function] = append(byFn[e.Function], e)
	}

	sum := OutputSummary{}

	// ---- cabin – always extracted first ------------------------------------
	for _, e := range byFn[r.Cabin] {
		sum.CabinEntries = appendUniqEntry(sum.CabinEntries, e)
	}

	// ---- red lights from Pc5 (primary + extra functions) -------------------
	// Pc5>  → loco heading A, tail at B → red B
	// Pc5<  → loco heading B, tail at A → red A
	redA := setOf(entriesWithDir(byFn[r.Pc5], DirB)) // Pc5< → red A
	redB := setOf(entriesWithDir(byFn[r.Pc5], DirA)) // Pc5> → red B
	for _, extraFn := range r.Pc5Extra {
		for o := range setOf(entriesWithDir(byFn[extraFn], DirB)) {
			redA[o] = true
		}
		for o := range setOf(entriesWithDir(byFn[extraFn], DirA)) {
			redB[o] = true
		}
	}

	// ---- red candidates from Pc2 -------------------------------------------
	// Pc2> → red candidate A,  Pc2< → red candidate B
	pc2RedCandA := setOf(entriesWithDir(byFn[r.Pc2], DirA))
	pc2RedCandB := setOf(entriesWithDir(byFn[r.Pc2], DirB))

	hasTb1 := len(byFn[r.Tb1]) > 0
	hasPc1 := len(byFn[r.Pc1]) > 0
	hasPc2 := len(byFn[r.Pc2]) > 0

	switch {
	// ------------------------------------------------------------------ case 1
	case hasTb1:
		// Tb1 (shunting) fully defines the white set.
		// O:Tb1<  → shunting light on A side → white A
		// O:Tb1>  → shunting light on B side → white B
		sum.Strategy = fmt.Sprintf("Tb1/F%d (shunting)", r.Tb1)
		tb1Set := setOf(byFn[r.Tb1])

		whiteA := setOf(entriesWithDir(byFn[r.Tb1], DirB)) // Tb1< → white A
		whiteB := setOf(entriesWithDir(byFn[r.Tb1], DirA)) // Tb1> → white B

		// Red from Pc2: outputs in Pc2 absent from Tb1 white set.
		if hasPc2 {
			for o := range pc2RedCandA {
				if !tb1Set[o] {
					redA[o] = true
				}
			}
			for o := range pc2RedCandB {
				if !tb1Set[o] {
					redB[o] = true
				}
			}
		}

		sum.WhiteA = sortedKeys(whiteA)
		sum.WhiteB = sortedKeys(whiteB)

	// ------------------------------------------------------------------ case 2
	case hasPc1 && hasPc2:
		sum.Strategy = fmt.Sprintf("Pc1/F%d minus Pc2/F%d reds", r.Pc1, r.Pc2)

		pc1A := setOf(entriesWithDir(byFn[r.Pc1], DirA))
		pc1B := setOf(entriesWithDir(byFn[r.Pc1], DirB))

		whiteA := make(map[uint8]bool)
		whiteB := make(map[uint8]bool)

		for o := range pc1A {
			if !pc2RedCandA[o] {
				whiteA[o] = true
			} else {
				redA[o] = true
			}
		}
		for o := range pc1B {
			if !pc2RedCandB[o] {
				whiteB[o] = true
			} else {
				redB[o] = true
			}
		}

		sum.WhiteA = sortedKeys(whiteA)
		sum.WhiteB = sortedKeys(whiteB)

	// ------------------------------------------------------------------ case 3
	case hasPc1:
		sum.Strategy = fmt.Sprintf("Pc1/F%d only (colour unknown)", r.Pc1)
		sum.UnknownA = sortedKeys(setOf(entriesWithDir(byFn[r.Pc1], DirA)))
		sum.UnknownB = sortedKeys(setOf(entriesWithDir(byFn[r.Pc1], DirB)))

	default:
		sum.Strategy = "unknown (no Pc1/Pc2/Tb1 found)"
	}

	sum.RedA = sortedKeys(redA)
	sum.RedB = sortedKeys(redB)

	return sum
}

// ---- helpers --------------------------------------------------------------------

// entriesWithDir filters entries keeping only those with the given direction.
func entriesWithDir(entries []OutputEntry, dir Direction) []OutputEntry {
	var out []OutputEntry
	for _, e := range entries {
		if e.Direction == dir {
			out = append(out, e)
		}
	}
	return out
}

// setOf returns a set (map[uint8]bool) of output numbers from a slice of entries.
func setOf(entries []OutputEntry) map[uint8]bool {
	s := make(map[uint8]bool, len(entries))
	for _, e := range entries {
		s[e.Output] = true
	}
	return s
}

// sortedKeys returns the keys of a set sorted ascending.
func sortedKeys(s map[uint8]bool) []uint8 {
	out := make([]uint8, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sortOutputs(out)
	return out
}

// appendUniqEntry appends e to dst only if dst does not yet contain an entry
// with the same output number.
func appendUniqEntry(dst []OutputEntry, e OutputEntry) []OutputEntry {
	for _, x := range dst {
		if x.Output == e.Output {
			return dst
		}
	}
	return append(dst, e)
}

func sortOutputs(s []uint8) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
