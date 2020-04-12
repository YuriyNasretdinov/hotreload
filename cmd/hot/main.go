package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	gopath     = os.Getenv("GOPATH")
	softDir    = filepath.Join(gopath, "soft")
	softGopath = filepath.Join(softDir, "p")
)

func main() {
	if gopath == "" {
		log.Fatal("GOPATH must be set")
	}

	log.Printf("Starting to rewrite %s", gopath)
	os.Stderr.Write([]byte("\n"))

	syncDir(filepath.Join(gopath, "src"), filepath.Join(softGopath, "src"))

	os.Setenv("GOPATH", softGopath)

	os.Stderr.Write([]byte("\n"))

	if wd, err := os.Getwd(); err == nil && strings.HasPrefix(wd, gopath+string(os.PathSeparator)) {
		newDir := softGopath + string(os.PathSeparator) + strings.TrimPrefix(wd, gopath+string(os.PathSeparator))
		log.Printf("Changing current directory to %s", newDir)
		os.Chdir(newDir)
	}

	ex, err := exec.LookPath(os.Args[1])
	if err != nil {
		log.Fatalf("Could not find executable for %s: %s", os.Args[1], err.Error())
	}

	log.Printf("Running %s %v", ex, os.Args[1:])

	syscall.Exec(ex, os.Args[1:], os.Environ())
}
