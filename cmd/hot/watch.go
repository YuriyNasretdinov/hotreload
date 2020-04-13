package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func watchChanges(gopath, watchDir string, stdin io.WriteCloser) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("watchChanges: fsnotify.NewWatcher(): %v", err)
	}

	err = filepath.Walk(watchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return watcher.Add(path)
		}

		return nil
	})

	if err != nil {
		log.Fatalf("watchChanges: walk(%q): %v", watchDir, err)
	}

	for ev := range watcher.Events {
		if ev.Op == fsnotify.Chmod {
			continue
		}

		if ev.Op != fsnotify.Write {
			log.Fatalf("Only WRITE changes are supported for live reload. Received %s event for %q", ev.Op, ev.Name)
		}

		time.Sleep(time.Millisecond * 25)

		goPkg := strings.TrimPrefix(ev.Name, gopath+"/src/")

		log.Printf("Received write for %s", goPkg)
	}
}
