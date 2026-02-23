package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/decoders"
)

func (app *LocoApp) ClearSoundSlot(slot uint8, opts ...decoders.Option) error {
	rb := decoders.NewRailboxRB23xx(opts...)
	return rb.ClearSoundSlot(slot)
}

// SyncSoundSlot synchronises a local directory with the given sound slot on the decoder:
//   - files present locally but missing on the decoder are uploaded
//   - files present on the decoder but missing locally are deleted from the decoder
//   - files present on both sides but differing in size (KB) are re-uploaded
//
// When dryRun is true, no changes are made – only a summary is printed.
func (app *LocoApp) SyncSoundSlot(slot uint8, localDir string, dryRun bool, opts ...decoders.Option) error {
	rb := decoders.NewRailboxRB23xx(opts...)

	if dryRun {
		_, _ = app.P.Printf("[dry-run] no changes will be made\n")
	}

	// --- build map of local files: name → size in bytes ---
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("cannot read local directory %q: %w", localDir, err)
	}
	type localInfo struct{ sizeBytes int64 }
	localFiles := make(map[string]localInfo, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fi, statErr := e.Info()
		if statErr != nil {
			return fmt.Errorf("cannot stat %q: %w", e.Name(), statErr)
		}
		localFiles[e.Name()] = localInfo{sizeBytes: fi.Size()}
	}

	// --- build map of remote files: name → size in KB ---
	remoteList, err := rb.ListSoundSlot(slot)
	if err != nil {
		return fmt.Errorf("cannot list slot %d on decoder: %w", slot, err)
	}
	remoteFiles := make(map[string]int64, len(remoteList))
	for _, info := range remoteList {
		remoteFiles[info.Name] = info.SizeKB
	}

	// --- upload missing or changed files ---
	changes := 0
	for name, local := range localFiles {
		remoteSizeKB, existsRemotely := remoteFiles[name]
		if existsRemotely {
			// decoder reports size in KB (1 KB = 1024 bytes); round up local size
			localSizeKB := (local.sizeBytes + 1023) / 1024
			diff := localSizeKB - remoteSizeKB
			if diff < 0 {
				diff = -diff
			}
			if diff <= 1 {
				logrus.Debugf("sync: skipping %q (size within tolerance: local %d KB, remote %d KB)", name, localSizeKB, remoteSizeKB)
				continue
			}
			_, _ = app.P.Printf("changed:  %s (local %d KB, remote %d KB)\n", name, localSizeKB, remoteSizeKB)
			logrus.Infof("sync: re-uploading %q (local %d KB, remote %d KB)", name, localSizeKB, remoteSizeKB)
		} else {
			_, _ = app.P.Printf("upload:   %s\n", name)
			logrus.Infof("sync: uploading new file %q to slot %d", name, slot)
		}

		changes++
		if dryRun {
			continue
		}

		f, openErr := os.Open(filepath.Join(localDir, name))
		if openErr != nil {
			return fmt.Errorf("cannot open %q: %w", name, openErr)
		}
		uploadErr := rb.UploadSoundFile(slot, name, f)
		_ = f.Close()
		if uploadErr != nil {
			return fmt.Errorf("upload %q failed: %w", name, uploadErr)
		}
	}

	// --- delete orphaned files ---
	for name := range remoteFiles {
		if _, exists := localFiles[name]; exists {
			continue
		}
		_, _ = app.P.Printf("delete:   %s\n", name)
		logrus.Infof("sync: deleting %q from slot %d on decoder", name, slot)
		changes++
		if dryRun {
			continue
		}
		if delErr := rb.DeleteSoundFile(slot, name); delErr != nil {
			return fmt.Errorf("delete %q failed: %w", name, delErr)
		}
	}

	if changes == 0 {
		_, _ = app.P.Printf("everything is up to date\n")
	}

	return nil
}
