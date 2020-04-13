package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	watchDir = flag.String("watch", "", "Which directory to watch for changes to do live reload")

	gopath     = os.Getenv("GOPATH")
	softDir    = filepath.Join(gopath, "soft")
	softGopath = filepath.Join(softDir, "p")
)

func main() {
	flag.Parse()

	if *watchDir == "" {
		log.Fatal("Must specify watch dir")
	}

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

	args := flag.Args()

	var cmd *exec.Cmd
	if len(args) > 1 {
		cmd = exec.Command(args[0], args[1:]...)
	} else {
		cmd = exec.Command(args[0])
	}

	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("Couldn't open stdin pipe: %v", err)
	}

	go watchChanges(gopath, *watchDir, stdin)

	log.Fatal(cmd.Run())
}
