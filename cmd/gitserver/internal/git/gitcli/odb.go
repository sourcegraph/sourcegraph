package gitcli

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/format/config"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) GetCommit(ctx context.Context, commit api.CommitID, includeModifiedFiles bool) (*git.GitCommitWithFiles, error) {
	if err := checkSpecArgSafety(string(commit)); err != nil {
		return nil, err
	}

	args := buildGetCommitArgs(commit, includeModifiedFiles)

	r, err := g.NewCommand(ctx, WithArguments(args...))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	rawCommit, err := io.ReadAll(r)
	if err != nil {
		// If exit code is 128 and `fatal: bad object` is part of stderr, most likely we
		// are referencing a commit that does not exist.
		// We want to return a gitdomain.RevisionNotFoundError in that case.
		var e *CommandFailedError
		if errors.As(err, &e) && e.ExitStatus == 128 && bytes.Contains(e.Stderr, []byte("fatal: bad object")) {
			return nil, &gitdomain.RevisionNotFoundError{Repo: g.repoName, Spec: string(commit)}
		}

		return nil, err
	}

	c, err := parseCommitLogOutput(bytes.TrimPrefix(rawCommit, []byte{'\x1e'}))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse commit log output")
	}
	return c, nil
}

func buildGetCommitArgs(commit api.CommitID, includeModifiedFiles bool) []string {
	args := []string{"log", logFormatWithoutRefs, "-n", "1"}
	if includeModifiedFiles {
		args = append(args, "--name-only")
	}
	args = append(args, string(commit))
	return args
}

const (
	partsPerCommit = 10 // number of \x00-separated fields per commit

	// This format string has 10 parts:
	//  1) oid
	//  2) author name
	//  3) author email
	//  4) author time
	//  5) committer name
	//  6) committer email
	//  7) committer time
	//  8) message body
	//  9) parent hashes
	// 10) modified files (optional)
	//
	// Each commit starts with an ASCII record separator byte (0x1E), and
	// each field of the commit is separated by a null byte (0x00).
	//
	// Refs are slow, and are intentionally not included because they are usually not needed.
	logFormatWithoutRefs = "--format=format:%x1e%H%x00%aN%x00%aE%x00%at%x00%cN%x00%cE%x00%ct%x00%B%x00%P%x00"
)

func parseCommitLogOutput(rawCommit []byte) (*git.GitCommitWithFiles, error) {
	parts := bytes.Split(rawCommit, []byte{'\x00'})
	if len(parts) != partsPerCommit {
		return nil, errors.Newf("internal error: expected %d parts, got %d", partsPerCommit, len(parts))
	}

	return parseCommitFromLog(parts)
}

// parseCommitFromLog parses the next commit from data and returns the commit and the remaining
// data. The data arg is a byte array that contains NUL-separated log fields as formatted by
// logFormatFlag.
func parseCommitFromLog(parts [][]byte) (*git.GitCommitWithFiles, error) {
	// log outputs are newline separated, so all but the 1st commit ID part
	// has an erroneous leading newline.
	parts[0] = bytes.TrimPrefix(parts[0], []byte{'\n'})
	commitID := api.CommitID(parts[0])

	authorTime, err := strconv.ParseInt(string(parts[3]), 10, 64)
	if err != nil {
		return nil, errors.Errorf("parsing git commit author time: %s", err)
	}
	committerTime, err := strconv.ParseInt(string(parts[6]), 10, 64)
	if err != nil {
		return nil, errors.Errorf("parsing git commit committer time: %s", err)
	}

	var parents []api.CommitID
	if parentPart := parts[8]; len(parentPart) > 0 {
		parentIDs := bytes.Split(parentPart, []byte{' '})
		parents = make([]api.CommitID, len(parentIDs))
		for i, id := range parentIDs {
			parents[i] = api.CommitID(id)
		}
	}

	var fileNames []string
	if fileOut := string(bytes.TrimSpace(parts[9])); fileOut != "" {
		fileNames = strings.Split(fileOut, "\n")
	}

	return &git.GitCommitWithFiles{
		Commit: &gitdomain.Commit{
			ID:        commitID,
			Author:    gitdomain.Signature{Name: string(parts[1]), Email: string(parts[2]), Date: time.Unix(authorTime, 0).UTC()},
			Committer: &gitdomain.Signature{Name: string(parts[4]), Email: string(parts[5]), Date: time.Unix(committerTime, 0).UTC()},
			Message:   gitdomain.Message(strings.TrimSuffix(string(parts[7]), "\n")),
			Parents:   parents,
		},
		ModifiedFiles: fileNames,
	}, nil
}

func (g *gitCLIBackend) ReadFile(ctx context.Context, commit api.CommitID, path string) (io.ReadCloser, error) {
	if err := gitdomain.EnsureAbsoluteCommit(commit); err != nil {
		return nil, err
	}

	blobOID, err := g.getBlobOID(ctx, commit, path)
	if err != nil {
		if err == errIsSubmodule {
			return io.NopCloser(bytes.NewReader(nil)), nil
		}
		return nil, err
	}

	return g.NewCommand(ctx, WithArguments("cat-file", "-p", string(blobOID)))
}

var errIsSubmodule = errors.New("blob is a submodule")

func (g *gitCLIBackend) getBlobOID(ctx context.Context, commit api.CommitID, path string) (api.CommitID, error) {
	out, err := g.NewCommand(ctx, WithArguments("ls-tree", string(commit), "--", path))
	if err != nil {
		return "", err
	}
	defer out.Close()

	stdout, err := io.ReadAll(out)
	if err != nil {
		// If exit code is 128 and `not a tree object` is part of stderr, most likely we
		// are referencing a commit that does not exist.
		// We want to return a gitdomain.RevisionNotFoundError in that case.
		var e *CommandFailedError
		if errors.As(err, &e) && e.ExitStatus == 128 {
			if bytes.Contains(e.Stderr, []byte("not a tree object")) || bytes.Contains(e.Stderr, []byte("Not a valid object name")) {
				return "", &gitdomain.RevisionNotFoundError{Repo: g.repoName, Spec: string(commit)}
			}
		}

		return "", err
	}

	stdout = bytes.TrimSpace(stdout)
	if len(stdout) == 0 {
		return "", &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}

	// format: 100644 blob 3bad331187e39c05c78a9b5e443689f78f4365a7	README.md
	fields := bytes.Fields(stdout)
	if len(fields) < 3 {
		return "", errors.Newf("unexpected output while parsing blob OID: %q", string(stdout))
	}
	if string(fields[0]) == "160000" {
		return "", errIsSubmodule
	}
	return api.CommitID(fields[2]), nil
}

// Stat returns a FileInfo describing the named file at commit. If the file is a
// symbolic link, the returned FileInfo describes the symbolic link. lStat makes
// no attempt to follow the link.
func (g *gitCLIBackend) Stat(ctx context.Context, commit api.CommitID, path string) (_ fs.FileInfo, err error) {
	if err := checkSpecArgSafety(string(commit)); err != nil {
		return nil, err
	}

	path = filepath.Clean(rel(path))

	// Special case root, which is not returned by `git ls-tree`.
	if path == "" || path == "." {
		rev, err := g.revParse(ctx, string(commit)+"^{tree}")
		if err != nil {
			if errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
				return nil, &os.PathError{Op: "ls-tree", Path: path, Err: os.ErrNotExist}
			}
			return nil, err
		}
		oid, err := decodeOID(rev)
		if err != nil {
			return nil, err
		}
		return &fileutil.FileInfo{Mode_: os.ModeDir, Sys_: objectInfo(oid)}, nil
	}

	it, err := g.lsTree(ctx, commit, path, false)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Append(err, it.Close()) }()

	fi, err := it.Next()
	if err != nil {
		if err == io.EOF {
			return nil, &os.PathError{Op: "ls-tree", Path: path, Err: os.ErrNotExist}
		}
		return nil, err
	}

	return fi, nil
}

func (g *gitCLIBackend) ReadDir(ctx context.Context, commit api.CommitID, path string, recursive bool) (git.ReadDirIterator, error) {
	if path != "" {
		// Trailing slash is necessary to ls-tree under the dir (not just
		// to list the dir's tree entry in its parent dir).
		path = filepath.Clean(rel(path)) + "/"
	}

	return g.lsTree(ctx, commit, path, recursive)
}

func (g *gitCLIBackend) lsTree(ctx context.Context, commit api.CommitID, path string, recurse bool) (_ git.ReadDirIterator, err error) {
	if err := gitdomain.EnsureAbsoluteCommit(commit); err != nil {
		return nil, err
	}

	// Don't call filepath.Clean(path) because ReadDir needs to pass
	// path with a trailing slash.

	if err := checkSpecArgSafety(path); err != nil {
		return nil, err
	}

	args := []string{
		"ls-tree",
		"--long", // show size
		"--full-name",
		string(commit),
	}
	if recurse {
		args = append(args, "-r", "-t")
	}
	if path != "" {
		args = append(args, "--", filepath.ToSlash(path))
	}

	r, err := g.NewCommand(ctx, WithArguments(args...))
	if err != nil {
		return nil, err
	}

	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			var cfe CommandFailedError
			if errors.As(closeErr, &cfe) {
				if bytes.Contains(cfe.Stderr, []byte("exists on disk, but not in")) {
					err = &os.PathError{Op: "ls-tree", Path: filepath.ToSlash(path), Err: os.ErrNotExist}
				}
			} else {
				err = errors.Append(err, closeErr)
			}
		}
	}()

	sc := bufio.NewScanner(r)

	trimPath := strings.TrimPrefix(path, "./")
	fis := make([]fs.FileInfo, 0)

	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}

		tabPos := strings.IndexByte(line, '\t')
		if tabPos == -1 {
			return nil, errors.Errorf("invalid `git ls-tree` output: %q", line)
		}
		info := strings.SplitN(line[:tabPos], " ", 4)
		name := line[tabPos+1:]
		if len(name) < len(trimPath) {
			// This is in a submodule; return the original path to avoid a slice out of bounds panic
			// when setting the FileInfo._Name below.
			name = trimPath
		}

		if len(info) != 4 {
			return nil, errors.Errorf("invalid `git ls-tree` output: %q", line)
		}
		typ := info[1]
		sha := info[2]
		if !gitdomain.IsAbsoluteRevision(sha) {
			return nil, errors.Errorf("invalid `git ls-tree` SHA output: %q", sha)
		}
		oid, err := decodeOID(api.CommitID(sha))
		if err != nil {
			return nil, err
		}

		sizeStr := strings.TrimSpace(info[3])
		var size int64
		if sizeStr != "-" {
			// Size of "-" indicates a dir or submodule.
			size, err = strconv.ParseInt(sizeStr, 10, 64)
			if err != nil || size < 0 {
				return nil, errors.Errorf("invalid `git ls-tree` size output: %q (error: %s)", sizeStr, err)
			}
		}

		var sys any
		modeVal, err := strconv.ParseInt(info[0], 8, 32)
		if err != nil {
			return nil, err
		}

		loadModConf := sync.OnceValues(func() (config.Config, error) {
			return g.gitModulesConfig(ctx, commit)
		})

		mode := os.FileMode(modeVal)
		switch typ {
		case "blob":
			if mode&gitdomain.ModeSymlink != 0 {
				mode = os.ModeSymlink
			} else {
				// Regular file.
				mode = mode | 0o644
			}
		case "commit":
			mode = mode | gitdomain.ModeSubmodule

			modconf, err := loadModConf()
			if err != nil {
				return nil, err
			}

			submodule := gitdomain.Submodule{
				URL:      modconf.Section("submodule").Subsection(name).Option("url"),
				Path:     modconf.Section("submodule").Subsection(name).Option("path"),
				CommitID: api.CommitID(oid.String()),
			}

			sys = submodule
		case "tree":
			mode = mode | os.ModeDir
		}

		if sys == nil {
			// Some callers might find it useful to know the object's OID.
			sys = objectInfo(oid)
		}

		fis = append(fis, &fileutil.FileInfo{
			Name_: name, // full path relative to root (not just basename)
			Mode_: mode,
			Size_: size,
			Sys_:  sys,
		})
	}
	if err := sc.Err(); err != nil {
		var cfe *CommandFailedError
		if errors.As(err, &cfe) {
			if bytes.Contains(cfe.Stderr, []byte("exists on disk, but not in")) {
				return nil, &os.PathError{Op: "ls-tree", Path: filepath.ToSlash(path), Err: os.ErrNotExist}
			}
			if cfe.ExitStatus == 128 && bytes.Contains(cfe.Stderr, []byte("fatal: not a tree object")) {
				return nil, &gitdomain.RevisionNotFoundError{Repo: g.repoName, Spec: string(commit)}
			}
		}
		return nil, err
	}

	if len(fis) == 0 {
		// If we are listing the empty root tree, we will have no output.
		if filepath.Clean(path) == "." {
			return []fs.FileInfo{}, nil
		}
		return nil, &os.PathError{Op: "git ls-tree", Path: path, Err: os.ErrNotExist}

	}

	// TODO: Needed? This will break chunking and requires all entries to be loaded
	// into memory.
	fileutil.SortFileInfosByName(fis)

	return fis, nil
}

func (g *gitCLIBackend) gitModulesConfig(ctx context.Context, commit api.CommitID) (config.Config, error) {
	r, err := g.ReadFile(ctx, commit, ".gitmodules")
	if err != nil {
		if os.IsNotExist(err) {
			return config.Config{}, nil
		}
		return config.Config{}, err
	}
	defer r.Close()

	modfile, err := io.ReadAll(r)
	if err != nil {
		return config.Config{}, err
	}

	var cfg config.Config
	err = config.NewDecoder(bytes.NewBuffer(modfile)).Decode(&cfg)
	if err != nil {
		return config.Config{}, errors.Wrap(err, "error parsing .gitmodules")
	}
	return cfg, nil
}

// rel strips the leading "/" prefix from the path string, effectively turning
// an absolute path into one relative to the root directory. A path that is just
// "/" is treated specially, returning just ".".
//
// The elements in a file path are separated by slash ('/', U+002F) characters,
// regardless of host operating system convention.
func rel(path string) string {
	if path == "/" {
		return "."
	}
	return strings.TrimPrefix(path, "/")
}

type objectInfo gitdomain.OID

func (oid objectInfo) OID() gitdomain.OID { return gitdomain.OID(oid) }
