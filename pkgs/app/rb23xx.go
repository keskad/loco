package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/commandstation"
	"github.com/keskad/loco/pkgs/decoders"
)

const wifiCV = 200

// RBWifiAction reads CV200 to determine which function number controls the WiFi router,
// then enables or disables that function on the decoder.
func (app *LocoApp) RBWifiAction(mode string, locoId uint8, enable bool, timeout time.Duration) error {
	if cmdErr := app.initializeCommandStation(); cmdErr != nil {
		return cmdErr
	}
	defer app.station.CleanUp()

	// Read CV200 to find the function number assigned to the WiFi router
	fnNum, err := app.station.ReadCV(commandstation.Mode(mode), commandstation.LocoCV{
		LocoId: commandstation.LocoAddr(locoId),
		Cv: commandstation.CV{
			Num: commandstation.CVNum(wifiCV),
		},
	}, commandstation.Timeout(timeout))
	if err != nil {
		return fmt.Errorf("failed to read CV%d (WiFi function number): %w", wifiCV, err)
	}

	logrus.Debugf("CV%d = %d, toggling F%d to enabled=%v", wifiCV, fnNum, fnNum, enable)

	// Send the function command
	return app.station.SendFn(commandstation.Mode(mode), commandstation.LocoAddr(locoId), commandstation.FuncNum(fnNum), enable)
}

func (app *LocoApp) ClearSoundSlot(slot uint8, opts ...decoders.Option) error {
	rb := decoders.NewRailboxRB23xx(opts...)
	return rb.ClearSoundSlot(slot)
}

// SyncSoundSlot synchronises a local directory with the given sound slot on the decoder:
//   - files present locally but missing on the decoder are uploaded
//   - files present on the decoder but missing locally are deleted from the decoder
//   - files present on both sides but differing in size (KB) are re-uploaded
//   - unless syncWithoutLast is true, the 5 most recently modified local files
//     (modified within the last 24 h) are always re-uploaded
//
// When dryRun is true, no changes are made – only a summary is printed.
func (app *LocoApp) SyncSoundSlot(slot uint8, localDir string, dryRun bool, syncWithoutLast bool, opts ...decoders.Option) error {
	rb := decoders.NewRailboxRB23xx(opts...)

	if dryRun {
		_, _ = app.P.Printf("[dry-run] no changes will be made\n")
	}

	// --- build map of local files: name → size in bytes ---
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("cannot read local directory %q: %w", localDir, err)
	}
	type localInfo struct {
		sizeBytes int64
		modTime   time.Time
	}
	localFiles := make(map[string]localInfo, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fi, statErr := e.Info()
		if statErr != nil {
			return fmt.Errorf("cannot stat %q: %w", e.Name(), statErr)
		}
		localFiles[e.Name()] = localInfo{sizeBytes: fi.Size(), modTime: fi.ModTime()}
	}

	// --- determine the set of "recently modified" files to always re-upload ---
	// Up to 5 local files modified within the last 24 h, sorted newest-first.
	recentlyModified := make(map[string]bool)
	if !syncWithoutLast {
		cutoff := time.Now().Add(-24 * time.Hour)

		type nameTime struct {
			name    string
			modTime time.Time
		}
		var candidates []nameTime
		for name, info := range localFiles {
			if info.modTime.After(cutoff) {
				candidates = append(candidates, nameTime{name, info.modTime})
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].modTime.After(candidates[j].modTime)
		})
		if len(candidates) > 5 {
			candidates = candidates[:5]
		}
		for _, c := range candidates {
			recentlyModified[c.name] = true
		}
		if len(recentlyModified) > 0 {
			logrus.Debugf("sync: %d recently modified file(s) will be force-uploaded (modified within last 24 h)", len(recentlyModified))
		}
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
				if recentlyModified[name] {
					_, _ = app.P.Printf("recent:   %s (modified within last 24 h)\n", name)
					logrus.Infof("sync: force-uploading %q – modified within last 24 h", name)
				} else {
					logrus.Debugf("sync: skipping %q (size within tolerance: local %d KB, remote %d KB)", name, localSizeKB, remoteSizeKB)
					continue
				}
			} else {
				_, _ = app.P.Printf("changed:  %s (local %d KB, remote %d KB)\n", name, localSizeKB, remoteSizeKB)
				logrus.Infof("sync: re-uploading %q (local %d KB, remote %d KB)", name, localSizeKB, remoteSizeKB)
			}
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

// WatchSoundSlot watches localDir for filesystem changes and triggers SyncSoundSlot
// each time a file is created, written or removed. A debounce of 500 ms is applied
// so that rapid bursts of events (e.g. an editor saving atomically) produce only
// one synchronisation run. The function blocks until the process is interrupted
// (i.e. the watcher channels are closed). Errors – including a failed initial sync
// or a failed triggered sync – are logged and printed, but never stop the watch loop.
func (app *LocoApp) WatchSoundSlot(slot uint8, localDir string, dryRun bool, syncWithoutLast bool, opts ...decoders.Option) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("cannot create filesystem watcher: %w", err)
	}
	defer watcher.Close()

	if err = watcher.Add(localDir); err != nil {
		return fmt.Errorf("cannot watch directory %q: %w", localDir, err)
	}

	_, _ = app.P.Printf("watch: watching %q for changes (Ctrl+C to stop)\n", localDir)
	logrus.Infof("watch: fsnotify watcher started on %q", localDir)

	runSync := func(reason string) {
		_, _ = app.P.Printf("watch: %s, syncing…\n", reason)
		logrus.Infof("watch: %s, triggering sync of %q → slot %d", reason, localDir, slot)
		if syncErr := app.SyncSoundSlot(slot, localDir, dryRun, syncWithoutLast, opts...); syncErr != nil {
			_, _ = app.P.Printf("watch: sync error: %v\n", syncErr)
			logrus.Errorf("watch: sync failed: %v", syncErr)
		}
	}

	// Run an initial sync before entering the watch loop.
	// Errors are non-fatal – the loop still starts afterwards.
	runSync("starting initial sync")

	const debounce = 500 * time.Millisecond
	var timer *time.Timer

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// React to write, create and remove events; ignore chmod/rename noise.
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				logrus.Debugf("watch: fsnotify event %s on %q", event.Op, event.Name)
				// Debounce: reset the timer on every new event within the window.
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(debounce, func() { runSync("change detected") })
			}

		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			// Log watcher errors but keep the loop running.
			_, _ = app.P.Printf("watch: watcher error: %v\n", watchErr)
			logrus.Errorf("watch: watcher error: %v", watchErr)
		}
	}
}
