package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

/*
 * Create a gitignore file in ${GIT_DIR}/info/exclude
 * if there are ignores defined in the configuration file.
 * Returns `error` if the file could not be created or written to.
 */
func processIgnores(ignores []string, dst string) error {
	if len(ignores) == 0 {
		return nil
	}
		gitignorePath := filepath.Join(dst, "info", "exclude")
		gitignoreFile, err := os.Create(gitignorePath)
		if err != nil {
			return err
		}
		for _, ignore := range ignores {
			if _, err := gitignore_file.WriteString(ignore + "\n"); err != nil {
				gitignore_file.Close()
				return err
			}
		}
		if err := gitignore_file.Close(); err != nil {
			return nil
		}
	}
	return nil
}

func snapshot(logger *log.Logger, dir *SyncDir) error {
	name := filepath.Base(dir.Dir)

	dst := filepath.Join(dir.Destination, ".git")
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// create bare repo if missing
	if _, err := os.Stat(filepath.Join(dst, "HEAD")); os.IsNotExist(err) {
		if _, err := run(logger, name, exec.Command("git", "init", "--bare", dst)); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	env := []string{
		"GIT_COMMITTER_NAME=src-expose",
		"GIT_COMMITTER_EMAIL=support@sourcegraph.com",
		"GIT_AUTHOR_NAME=src-expose",
		"GIT_AUTHOR_EMAIL=support@sourcegraph.com",
		"GIT_DIR=" + dst,
		"GIT_WORK_TREE=" + dir.Dir,
	}

	// pass the same path as is used for `GIT_DIR` in the environment
	if err := processIgnores(dir.Ignore, dst); err != nil {
		logger.Printf("Unable to set up ignoring files because %s", err)
	}

	cmd := exec.Command("git", "status", "--porcelain", "--no-renames")
	cmd.Env = env
	cmd.Dir = dir.Dir
	n, err := run(logger, name, cmd)
	if err != nil {
		return err
	}

	// no lines in output of git status means nothing changed
	if n == 0 {
		logger.Printf("%s: nothing changed", name)
		return nil
	}

	cmds := [][]string{
		// we can't just git add, since if we are tracking files that are part
		// of .gitignore they will continue to be tracked. So we empty the
		// index.
		{"git", "rm", "-r", "-q", "--cached", "--ignore-unmatch", "."},

		// git add -A makes the index reflect the work tree
		{"git", "add", "-A"},

		{"git", "commit", "-m", "Sync at " + time.Now().Format("Mon Jan _2 15:04:05 2006")},
	}
	for _, a := range cmds {
		cmd := exec.Command(a[0], a[1:]...)
		cmd.Env = env
		cmd.Dir = dir.Dir
		_, err := run(logger, name, cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

func run(logger *log.Logger, name string, cmd *exec.Cmd) (int, error) {
	outW := &lineCountWriter{w: os.Stdout, prefix: []byte("> ")}
	errW := &lineCountWriter{w: os.Stdout, prefix: []byte("! ")}

	cmd.Stdout = outW
	cmd.Stderr = errW

	logger.Printf("%s> %v", name, strings.Join(cmd.Args, " "))
	err := cmd.Run()

	_ = outW.Close()
	_ = errW.Close()

	return outW.lines, err
}

type lineCountWriter struct {
	w      io.Writer
	prefix []byte

	inline bool
	lines  int
}

func (w *lineCountWriter) Write(b []byte) (int, error) {
	n := 0
	for len(b) > 0 {
		if !w.inline {
			w.lines++
			_, err := w.w.Write(w.prefix)
			if err != nil {
				return n, err
			}
		}

		var off int
		if i := bytes.Index(b, []byte{'\n'}); i < 0 {
			off = len(b)
			w.inline = true
		} else {
			off = i + 1 // include newline
			w.inline = false
		}

		var part []byte
		part, b = b[:off], b[off:]

		m, err := w.w.Write(part)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (w *lineCountWriter) Close() error {
	// write a newline if there is one missing
	if w.inline {
		w.inline = false
		_, err := w.w.Write([]byte{'\n'})
		return err
	}
	return nil
}

// SyncDir creates a commit of Dir into the bare git repo Destination.
type SyncDir struct {
	// Before if non-empty is a command run before syncing.
	Before string `yaml:",omitempty"`

	// Dir is the directory to treat as the git working directory.
	Dir string `yaml:",omitempty"`

	// Destination is the directory containing the bare git repo.
	Destination string `yaml:",omitempty"`

	// MinDuration defines the minimum wait between syncs for Dir.
	MinDuration time.Duration `yaml:",omitempty"`

	// Ignore is a list of `gitignore` patterns identifying items in Dir to exclude.
	Ignore []string `yaml:",omitempty"`

	// last stores the time of the last sync. Compared against MinDuration to
	// determine if we should run.
	last time.Time
}

// Snapshotter manages the running over several syncs.
type Snapshotter struct {
	// Root is the directory Before is run from. If a SyncDir's Dir is
	// relative, it will be resolved relative to this directory. Defaults to
	// PWD.
	Root string

	// If a SyncDir's Destination is relative, it will be resolved relative to
	// Destination. Defaults to ~/.sourcegraph/src-expose-repos
	Destination string

	// Before is a command run before sync. Before is run from Dir.
	Before string

	// Dirs is a list of directories to sync.
	Dirs []*SyncDir

	// DirMode defines what behaviour to use if Dir is missing.
	//
	//  - fail (default)
	//  - ignore
	//  - remove_dest
	DirMode string

	// Duration defines how often sync should run.
	Duration time.Duration
}

func (o *Snapshotter) SetDefaults() error {
	if o.Root == "" {
		d, err := os.Getwd()
		if err != nil {
			return err
		}
		o.Root = d
	}

	if o.Destination == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		o.Destination = filepath.Join(h, ".sourcegraph", "src-expose-repos")
	}

	if o.DirMode == "" {
		o.DirMode = "fail"
	}

	if o.Duration == 0 {
		o.Duration = 10 * time.Second
	}

	for i, s := range o.Dirs {
		if s.Destination == "" && !filepath.IsAbs(s.Dir) {
			s.Destination = s.Dir
		}

		d, err := abs(o.Root, s.Dir)
		if err != nil {
			return err
		}
		s.Dir = d

		d, err = abs(o.Destination, s.Destination)
		if err != nil {
			return err
		}
		s.Destination = d

		o.Dirs[i] = s
	}

	return nil
}

func abs(root, dir string) (string, error) {
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	return filepath.Abs(dir)
}

func (o *Snapshotter) Run(logger *log.Logger) error {
	if err := o.SetDefaults(); err != nil {
		return err
	}

	if o.Before != "" {
		cmd := exec.Command("sh", "-c", o.Before)
		cmd.Dir = o.Root
		if _, err := run(logger, "root", cmd); err != nil {
			return err
		}
	}

	for _, s := range o.Dirs {
		if time.Since(s.last) < s.MinDuration {
			continue
		}
		s.last = time.Now()

		if s.Before != "" {
			cmd := exec.Command("sh", "-c", s.Before)
			cmd.Dir = s.Dir
			if _, err := run(logger, filepath.Base(s.Dir), cmd); err != nil {
				return err
			}
		}

		if _, err := os.Stat(s.Dir); err != nil {
			switch o.DirMode {
			case "fail":
				return errors.Wrapf(err, "sync source dir missing: %v", s.Dir)
			case "ignore":
				logger.Printf("dir %s missing, ignoring", s.Dir)
				continue
			case "remove_dest":
				logger.Printf("dir %s missing, removing %s", s.Dir, s.Destination)
				if err := os.RemoveAll(s.Destination); err != nil {
					return errors.Wrapf(err, "failed to remove sync destination %s", s.Destination)
				}

			}
		}

		if err := snapshot(logger, s); err != nil {
			return err
		}
	}

	return nil
}
