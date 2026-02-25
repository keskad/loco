package app

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/keskad/loco/pkgs/syntax/outputmap"
)

// PrintOutputsAction reads the AUX output mapping file at mapFile, classifies
// every output as white/red side-A, white/red side-B or cabin, and prints a
// human-readable summary.
func (app *LocoApp) PrintOutputsAction(mapFile string) error {
	f, err := os.Open(mapFile)
	if err != nil {
		return fmt.Errorf("cannot open map file %q: %w", mapFile, err)
	}
	defer f.Close()

	m, err := outputmap.Parse(f)
	if err != nil {
		if errors.Is(err, outputmap.ErrMicrocontrollerBoard) {
			_, _ = app.P.Printf("Lighting outputs are not independently configurable.\n")
			_, _ = app.P.Printf("This board uses an on-board microcontroller to control lights.\n")
			return nil
		}
		return fmt.Errorf("cannot parse map file %q: %w", mapFile, err)
	}

	sum := m.Classify()

	_, _ = app.P.Printf("Detection strategy    : %s\n", sum.Strategy)
	_, _ = app.P.Printf("\n")

	if len(sum.UnknownA) > 0 || len(sum.UnknownB) > 0 {
		// Case 3: only F0 was present – colour is unknown, side is known.
		_, _ = app.P.Printf("Front outputs side A  : %s  (colour unknown – no F5/F6 in map)\n", formatOutputList(sum.UnknownA))
		_, _ = app.P.Printf("Front outputs side B  : %s  (colour unknown – no F5/F6 in map)\n", formatOutputList(sum.UnknownB))
	} else {
		_, _ = app.P.Printf("White lights – side A : %s\n", formatOutputList(sum.WhiteA))
		_, _ = app.P.Printf("White lights – side B : %s\n", formatOutputList(sum.WhiteB))
	}

	_, _ = app.P.Printf("Red lights   – side A : %s\n", formatOutputList(sum.RedA))
	_, _ = app.P.Printf("Red lights   – side B : %s\n", formatOutputList(sum.RedB))

	if len(sum.CabinEntries) == 0 {
		_, _ = app.P.Printf("Cabin lights          : (none)\n")
	} else {
		// sort by output number for deterministic output
		entries := sum.CabinEntries
		sort.Slice(entries, func(i, j int) bool { return entries[i].Output < entries[j].Output })
		for _, e := range entries {
			_, _ = app.P.Printf("Cabin light           : O%d  direction %s\n", e.Output, e.Direction)
		}
	}

	return nil
}

// formatOutputList renders a slice of output numbers as "O1, O3, O6" or "(none)".
func formatOutputList(outputs []uint8) string {
	if len(outputs) == 0 {
		return "(none)"
	}
	s := ""
	for i, o := range outputs {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("O%d", o)
	}
	return s
}
