package daemon

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors directories for new skill files and invokes a callback.
type Watcher struct {
	watcher    *fsnotify.Watcher
	dirs       []string
	onNewSkill func(path string)
	stopCh     chan struct{}
}

// NewWatcher creates a file watcher for the given directories.
func NewWatcher(dirs []string, onNewSkill func(path string)) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if err := fsw.Add(dir); err != nil {
			log.Printf("[AJ] Could not watch %s: %v", dir, err)
		}
	}

	return &Watcher{
		watcher:    fsw,
		dirs:       dirs,
		onNewSkill: onNewSkill,
		stopCh:     make(chan struct{}),
	}, nil
}

// Start begins watching for file system events. Blocks until Stop is called.
func (w *Watcher) Start() {
	for {
		select {
		case <-w.stopCh:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			// We care about new files and directories being created
			if event.Has(fsnotify.Create) {
				path := event.Name
				// If a new directory was created, watch it too (for skill subdirs)
				if isDir(path) {
					w.watcher.Add(path)
				}
				// If a skill.md was created, notify
				if strings.HasSuffix(filepath.Base(path), "skill.md") {
					w.onNewSkill(path)
				}
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[AJ] watcher error: %v", err)
		}
	}
}

// Stop closes the watcher.
func (w *Watcher) Stop() {
	close(w.stopCh)
	w.watcher.Close()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
