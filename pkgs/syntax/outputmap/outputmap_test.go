package outputmap_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/keskad/loco/pkgs/syntax/outputmap"
)

// fullSampleMap mirrors example_map_e1.txt (F0+F5+F6+F7+F8 all present,
// classic default function numbers).
const fullSampleMap = `
# Pc1, two front white lights (F0)
O4:F0<
O7:F0<
O1:F0>
O6:F0>

O12:F0<
O5:F0>

# Kabina (F8)
O11:F8<
O10:F8>

# Pc5 (F7)
O2:F7>
O9:F7>
O8:F7<
O3:F7<

# Pc2 (F5)
O12:F5<
O5:F5>
O4:F5<
O9:F5<
O1:F5>
O3:F5>

# Tb1 (F6)
O4:F6<
O1:F6<
O7:F6>
O6:F6>
`

// st44Map mirrors the real map_st44_kamilb.txt sample (non-default fn numbers,
// entry without direction suffix).
const st44Map = `
# Pc1, Biale, przednie, kierunkowe (F0)
O5:F0>
O1:F0>
O4:F0<
O2:F0<

# Pc5, Czerwone, tylnie kierunkowe (F7)
O8:F7<
O6:F7<
O3:F7>
O10:F7>

# Pc2, kierunek przeciwny do zasadniczego (F4)
O5:F4>
O6:F4>
O3:F4<
O2:F4<

# Tb1 (F16)
O5:F16>
O4:F16>
O1:F16<
O2:F16<

# Kabina (F8)
O9:F8>
O7:F8<

# Przedzial maszynowy (F5)
O11:F5
`

// ----- parser tests ----------------------------------------------------------

func TestParse_EntryCount(t *testing.T) {
	m, err := outputmap.Parse(strings.NewReader(fullSampleMap))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(m.Entries) == 0 {
		t.Fatal("expected at least one entry, got 0")
	}
}

func TestParse_InvalidDirection(t *testing.T) {
	// 'x' is not a valid direction character
	_, err := outputmap.Parse(strings.NewReader("O1:F0x\n"))
	if err == nil {
		t.Fatal("expected error for invalid direction char, got nil")
	}
}

func TestParse_InvalidOutput(t *testing.T) {
	_, err := outputmap.Parse(strings.NewReader("X1:F0>\n"))
	if err == nil {
		t.Fatal("expected error for invalid output token, got nil")
	}
}

func TestParse_MissingDirectionAccepted(t *testing.T) {
	// "O11:F5" has no direction suffix → must be accepted, not error
	m, err := outputmap.Parse(strings.NewReader("O11:F5\n"))
	if err != nil {
		t.Fatalf("expected no error for missing direction, got: %v", err)
	}
	if len(m.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(m.Entries))
	}
	if m.Entries[0].Direction != outputmap.DirNone {
		t.Errorf("expected DirNone, got %q", m.Entries[0].Direction)
	}
}

func TestParse_CommentAndBlankSkipped(t *testing.T) {
	input := "# this is a comment\n\nO1:F0>\n"
	m, err := outputmap.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(m.Entries))
	}
}

func TestParse_RoleDetection_DefaultsWhenNoComments(t *testing.T) {
	m, err := outputmap.Parse(strings.NewReader("O1:F0>\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No role comments → classic defaults
	if m.Roles.Pc1 != 0 {
		t.Errorf("expected Pc1=0 (default), got %d", m.Roles.Pc1)
	}
	if m.Roles.Tb1 != 6 {
		t.Errorf("expected Tb1=6 (default), got %d", m.Roles.Tb1)
	}
}

func TestParse_RoleDetection_FromComments(t *testing.T) {
	input := "# Pc1, Biale (F0)\n# Tb1 (F16)\n# Pc5, czerwone (F7)\n# Kabina (F8)\nO1:F0>\n"
	m, err := outputmap.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Roles.Pc1 != 0 {
		t.Errorf("expected Pc1=0, got %d", m.Roles.Pc1)
	}
	if m.Roles.Tb1 != 16 {
		t.Errorf("expected Tb1=16 (from comment), got %d", m.Roles.Tb1)
	}
	if m.Roles.Pc5 != 7 {
		t.Errorf("expected Pc5=7, got %d", m.Roles.Pc5)
	}
	if m.Roles.Cabin != 8 {
		t.Errorf("expected Cabin=8, got %d", m.Roles.Cabin)
	}
}

func TestParse_St44_NoParsError(t *testing.T) {
	// The ST44 map has "O11:F5" (no direction) and non-default fn numbers.
	// Must not return a parse error.
	_, err := outputmap.Parse(strings.NewReader(st44Map))
	if err != nil {
		t.Fatalf("unexpected parse error on st44 map: %v", err)
	}
}

func TestParse_St44_RolesDetected(t *testing.T) {
	m, _ := outputmap.Parse(strings.NewReader(st44Map))
	// # Pc2 … (F4)  and  # Tb1 (F16)
	if m.Roles.Pc2 != 4 {
		t.Errorf("expected Pc2=4 (from comment), got %d", m.Roles.Pc2)
	}
	if m.Roles.Tb1 != 16 {
		t.Errorf("expected Tb1=16 (from comment), got %d", m.Roles.Tb1)
	}
}

// ----- strategy 1: Tb1 present -----------------------------------------------

func TestClassify_Strategy1_Tb1_UsedForWhite(t *testing.T) {
	// Tb1< → white A,  Tb1> → white B
	input := "# Tb1 (F6)\nO4:F6<\nO1:F6<\nO7:F6>\nO6:F6>\n"
	m, _ := outputmap.Parse(strings.NewReader(input))
	sum := m.Classify()

	if !strings.Contains(sum.Strategy, "Tb1") {
		t.Errorf("expected strategy to mention 'Tb1', got %q", sum.Strategy)
	}
	mustContain(t, "WhiteA", sum.WhiteA, 4)
	mustContain(t, "WhiteA", sum.WhiteA, 1)
	mustContain(t, "WhiteB", sum.WhiteB, 7)
	mustContain(t, "WhiteB", sum.WhiteB, 6)
}

func TestClassify_Strategy1_Tb1_RedFromPc2NotInTb1(t *testing.T) {
	// Tb1 defines whites; O9 in Pc2 but not in Tb1 → red.
	input := "# Tb1 (F6)\n# Pc2 (F5)\nO4:F6<\nO7:F6>\nO9:F5<\n"
	m, _ := outputmap.Parse(strings.NewReader(input))
	sum := m.Classify()

	mustContain(t, "RedB", sum.RedB, 9)
	mustNotContain(t, "WhiteA", sum.WhiteA, 9)
	mustNotContain(t, "WhiteB", sum.WhiteB, 9)
}

func TestClassify_Strategy1_Tb1_Pc2OutputInTb1NotRed(t *testing.T) {
	// O4 in both Tb1 and Pc2 → must NOT appear in red.
	input := "# Tb1 (F6)\n# Pc2 (F5)\nO4:F6<\nO4:F5<\n"
	m, _ := outputmap.Parse(strings.NewReader(input))
	sum := m.Classify()

	mustNotContain(t, "RedA", sum.RedA, 4)
	mustNotContain(t, "RedB", sum.RedB, 4)
	mustContain(t, "WhiteA", sum.WhiteA, 4)
}

func TestClassify_Strategy1_FullSample(t *testing.T) {
	m, err := outputmap.Parse(strings.NewReader(fullSampleMap))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	sum := m.Classify()

	if !strings.Contains(sum.Strategy, "Tb1") {
		t.Errorf("expected strategy to mention 'Tb1', got %q", sum.Strategy)
	}

	// F6< → white A: O4, O1
	mustContain(t, "WhiteA", sum.WhiteA, 4)
	mustContain(t, "WhiteA", sum.WhiteA, 1)
	// F6> → white B: O7, O6
	mustContain(t, "WhiteB", sum.WhiteB, 7)
	mustContain(t, "WhiteB", sum.WhiteB, 6)

	// F7< → red A: O8, O3
	mustContain(t, "RedA", sum.RedA, 8)
	mustContain(t, "RedA", sum.RedA, 3)
	// F7> → red B: O2, O9
	mustContain(t, "RedB", sum.RedB, 2)
	mustContain(t, "RedB", sum.RedB, 9)

	// Tb1 outputs must not appear in red
	mustNotContain(t, "RedA", sum.RedA, 4)
	mustNotContain(t, "RedA", sum.RedA, 1)
	mustNotContain(t, "RedB", sum.RedB, 7)
	mustNotContain(t, "RedB", sum.RedB, 6)
}

func TestClassify_Strategy1_St44(t *testing.T) {
	// ST44 uses F16 for Tb1 and F4 for Pc2, F7 for Pc5.
	m, err := outputmap.Parse(strings.NewReader(st44Map))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	sum := m.Classify()

	if !strings.Contains(sum.Strategy, "Tb1") {
		t.Errorf("expected Tb1 strategy, got %q", sum.Strategy)
	}

	// F16< → white A: O1, O2
	mustContain(t, "WhiteA", sum.WhiteA, 1)
	mustContain(t, "WhiteA", sum.WhiteA, 2)
	// F16> → white B: O5, O4
	mustContain(t, "WhiteB", sum.WhiteB, 5)
	mustContain(t, "WhiteB", sum.WhiteB, 4)

	// F7< → red A: O8, O6
	mustContain(t, "RedA", sum.RedA, 8)
	mustContain(t, "RedA", sum.RedA, 6)
	// F7> → red B: O3, O10
	mustContain(t, "RedB", sum.RedB, 3)
	mustContain(t, "RedB", sum.RedB, 10)
}

// ----- strategy 2: Pc1 + Pc2, no Tb1 ----------------------------------------

func TestClassify_Strategy2_Pc1AndPc2_NoTb1(t *testing.T) {
	// O1:F0> → white A candidate (not in Pc2 → stays white)
	// O4:F0< → white B candidate (not in Pc2 → stays white)
	// O3 in F0> AND F5> → shared → moves to red A
	// O9 in F0< AND F5< → shared → moves to red B
	input := "# Pc1 (F0)\n# Pc2 (F5)\nO1:F0>\nO3:F0>\nO4:F0<\nO9:F0<\nO3:F5>\nO9:F5<\n"
	m, _ := outputmap.Parse(strings.NewReader(input))
	sum := m.Classify()

	if !strings.Contains(sum.Strategy, "Pc1") {
		t.Errorf("expected strategy to mention 'Pc1', got %q", sum.Strategy)
	}
	mustContain(t, "WhiteA", sum.WhiteA, 1)
	mustContain(t, "WhiteB", sum.WhiteB, 4)
	mustContain(t, "RedA", sum.RedA, 3)
	mustContain(t, "RedB", sum.RedB, 9)
	mustNotContain(t, "WhiteA", sum.WhiteA, 3)
	mustNotContain(t, "WhiteB", sum.WhiteB, 9)
}

func TestClassify_Strategy2_SharedOutputMovedToRed(t *testing.T) {
	// O1 in both F0> and F5> → moves to red A.
	input := "# Pc1 (F0)\n# Pc2 (F5)\nO1:F0>\nO1:F5>\n"
	m, _ := outputmap.Parse(strings.NewReader(input))
	sum := m.Classify()

	mustNotContain(t, "WhiteA", sum.WhiteA, 1)
	mustContain(t, "RedA", sum.RedA, 1)
}

// ----- strategy 3: Pc1 only --------------------------------------------------

func TestClassify_Strategy3_Pc1Only(t *testing.T) {
	input := "# Pc1 (F0)\nO1:F0>\nO6:F0>\nO4:F0<\nO7:F0<\n"
	m, _ := outputmap.Parse(strings.NewReader(input))
	sum := m.Classify()

	if !strings.Contains(sum.Strategy, "colour unknown") {
		t.Errorf("expected 'colour unknown' strategy, got %q", sum.Strategy)
	}
	if len(sum.WhiteA) != 0 {
		t.Errorf("expected no WhiteA, got %v", sum.WhiteA)
	}
	mustContain(t, "UnknownA", sum.UnknownA, 1)
	mustContain(t, "UnknownA", sum.UnknownA, 6)
	mustContain(t, "UnknownB", sum.UnknownB, 4)
	mustContain(t, "UnknownB", sum.UnknownB, 7)
}

func TestClassify_Strategy3_Pc1WithPc5(t *testing.T) {
	// Pc5 reds extracted even when strategy 3 is used.
	input := "# Pc1 (F0)\n# Pc5 (F7)\nO1:F0>\nO3:F7<\nO2:F7>\n"
	m, _ := outputmap.Parse(strings.NewReader(input))
	sum := m.Classify()

	if !strings.Contains(sum.Strategy, "colour unknown") {
		t.Errorf("expected 'colour unknown' strategy, got %q", sum.Strategy)
	}
	mustContain(t, "RedA", sum.RedA, 3)
	mustContain(t, "RedB", sum.RedB, 2)
}

// ----- cabin -----------------------------------------------------------------

func TestClassify_Cabin(t *testing.T) {
	input := "# Kabina (F8)\nO11:F8<\nO10:F8>\n"
	m, _ := outputmap.Parse(strings.NewReader(input))
	sum := m.Classify()
	if len(sum.CabinEntries) != 2 {
		t.Fatalf("expected 2 cabin entries, got %d: %v", len(sum.CabinEntries), sum.CabinEntries)
	}
}

// ----- Pc5Extra: multiple red functions --------------------------------------

// sm42Map mirrors map_rb2400_sm42.txt exactly: no comments, F0 has only
// 1 output per direction → microcontroller board → Parse must return
// ErrMicrocontrollerBoard.
const sm42Map = `O1:F0>
O2:F0<
O3:F7<
O4:F7>
O5:F6>
O6:F6<
O7:F8
O8:F27>
O9:F27<
`

// sm42MapAutoDetect is a synthetic variant of SM42 that gives F0 two outputs
// per direction so the microcontroller guard does NOT fire, letting us test
// Pc5Extra auto-detection in isolation.
const sm42MapAutoDetect = `O1:F0>
O2:F0<
O11:F0>
O12:F0<
O3:F7<
O4:F7>
O5:F6>
O6:F6<
O7:F8
O8:F27>
O9:F27<
`

func TestParse_Pc5Extra_FromComment_SingleLine(t *testing.T) {
	// "# Pc5 (F7)(F27)" — both on the same comment line
	input := "# Pc5 (F7)(F27)\nO3:F7<\nO4:F7>\nO8:F27>\nO9:F27<\n"
	m, err := outputmap.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if m.Roles.Pc5 != 7 {
		t.Errorf("expected Pc5=7, got %d", m.Roles.Pc5)
	}
	if len(m.Roles.Pc5Extra) != 1 || m.Roles.Pc5Extra[0] != 27 {
		t.Errorf("expected Pc5Extra=[27], got %v", m.Roles.Pc5Extra)
	}
}

func TestClassify_Pc5Extra_FromComment_RedOutputs(t *testing.T) {
	input := "# Pc5 (F7)(F27)\nO3:F7<\nO4:F7>\nO8:F27>\nO9:F27<\n"
	m, _ := outputmap.Parse(strings.NewReader(input))
	sum := m.Classify()

	// F7< → red A: O3;  F7> → red B: O4
	mustContain(t, "RedA", sum.RedA, 3)
	mustContain(t, "RedB", sum.RedB, 4)
	// F27> → red B: O8;  F27< → red A: O9
	mustContain(t, "RedB", sum.RedB, 8)
	mustContain(t, "RedA", sum.RedA, 9)
}

func TestParse_MicrocontrollerBoard_SM42(t *testing.T) {
	// sm42Map: F0 has exactly O1:F0> and O2:F0< → microcontroller board.
	_, err := outputmap.Parse(strings.NewReader(sm42Map))
	if err == nil {
		t.Fatal("expected ErrMicrocontrollerBoard, got nil")
	}
	if !errors.Is(err, outputmap.ErrMicrocontrollerBoard) {
		t.Errorf("expected ErrMicrocontrollerBoard, got: %v", err)
	}
}

func TestParse_Pc5Extra_AutoDetect_NoComments(t *testing.T) {
	// sm42MapAutoDetect: F0 has 2 outputs per direction → no microcontroller guard.
	// F27 has the same directional pattern as F7 → auto-detected as Pc5Extra.
	m, err := outputmap.Parse(strings.NewReader(sm42MapAutoDetect))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if m.Roles.Pc5 != 7 {
		t.Errorf("expected Pc5=7 (default), got %d", m.Roles.Pc5)
	}
	found := false
	for _, fn := range m.Roles.Pc5Extra {
		if fn == 27 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected F27 to be auto-detected as Pc5Extra, Pc5Extra=%v", m.Roles.Pc5Extra)
	}
}

func TestClassify_Pc5Extra_AutoDetect_SM42(t *testing.T) {
	m, err := outputmap.Parse(strings.NewReader(sm42MapAutoDetect))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	sum := m.Classify()

	// F6< → white A: O6;  F6> → white B: O5
	mustContain(t, "WhiteA", sum.WhiteA, 6)
	mustContain(t, "WhiteB", sum.WhiteB, 5)

	// F7< → red A: O3;  F7> → red B: O4
	mustContain(t, "RedA", sum.RedA, 3)
	mustContain(t, "RedB", sum.RedB, 4)

	// F27> → red B: O8;  F27< → red A: O9
	mustContain(t, "RedB", sum.RedB, 8)
	mustContain(t, "RedA", sum.RedA, 9)
}

func TestParse_Pc5Extra_AutoDetect_NotTriggeredForUnrelatedFn(t *testing.T) {
	// O5:F5> shares no outputs with F7 → must NOT be added to Pc5Extra.
	input := "O3:F7<\nO4:F7>\nO5:F5>\n"
	m, _ := outputmap.Parse(strings.NewReader(input))
	for _, fn := range m.Roles.Pc5Extra {
		if fn == 5 {
			t.Errorf("F5 should NOT be auto-detected as Pc5Extra")
		}
	}
}

// ----- helpers ---------------------------------------------------------------

func mustContain(t *testing.T, name string, s []uint8, v uint8) {
	t.Helper()
	for _, x := range s {
		if x == v {
			return
		}
	}
	t.Errorf("%s: expected to contain O%d, got %v", name, v, s)
}

func mustNotContain(t *testing.T, name string, s []uint8, v uint8) {
	t.Helper()
	for _, x := range s {
		if x == v {
			t.Errorf("%s: must NOT contain O%d, but it does (full list: %v)", name, v, s)
			return
		}
	}
}
