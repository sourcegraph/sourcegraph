package gitcli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
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
	"github.com/sourcegraph/sourcegraph/internal/byteutils"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) GetCommit(ctx context.Context, commit api.CommitID, includeModifiedFiles bool) (*git.GitCommitWithFiles, error) {
	if err := checkSpecArgSafety(string(commit)); err != nil {
		return nil, err
	}

	// commit sometimes is not a commitID today, so we run a revparse first to make
	// sure we're dealing with a commit ID. This will also report errors like
	// "cannot resolve to commit" as a RevisionNotFoundError.
	commitID, err := g.revParse(ctx, string(commit))
	if err != nil {
		return nil, err
	}

	args := buildGetCommitArgs(commitID, includeModifiedFiles)

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

	c, err := parseCommitFromLog(bytes.TrimPrefix(rawCommit, []byte{'\x1e'}))
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

// parseCommitFromLog parses the next commit from data and returns the commit and the remaining
// data. The data arg is a byte array that contains NUL-separated log fields as formatted by
// logFormatFlag.
func parseCommitFromLog(rawCommit []byte) (*git.GitCommitWithFiles, error) {
	parts := bytes.Split(rawCommit, []byte{'\x00'})
	if len(parts) != partsPerCommit {
		return nil, errors.Newf("internal error: expected %d parts, got %d", partsPerCommit, len(parts))
	}

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

func (g *gitCLIBackend) BehindAhead(ctx context.Context, left, right string) (*gitdomain.BehindAhead, error) {
	if err := checkSpecArgSafety(left); err != nil {
		return nil, err
	}
	if err := checkSpecArgSafety(right); err != nil {
		return nil, err
	}

	if left == "" {
		left = "HEAD"
	}

	if right == "" {
		right = "HEAD"
	}

	rc, err := g.NewCommand(ctx, WithArguments("rev-list", "--count", "--left-right", fmt.Sprintf("%s...%s", left, right)))
	if err != nil {
		return nil, errors.Wrap(err, "running git rev-list")
	}
	defer rc.Close()

	out, err := io.ReadAll(rc)
	if err != nil {
		var e *CommandFailedError
		if errors.As(err, &e) {
			switch {
			case e.ExitStatus == 128 && bytes.Contains(e.Stderr, []byte("fatal: ambiguous argument")):
				fallthrough
			case e.ExitStatus == 128 && bytes.Contains(e.Stderr, []byte("fatal: Invalid symmetric difference expression")):
				return nil, &gitdomain.RevisionNotFoundError{
					Repo: g.repoName,
					Spec: fmt.Sprintf("%s...%s", left, right),
				}
			}
		}

		return nil, errors.Wrap(err, "reading git rev-list output")
	}

	behindAhead := strings.Split(strings.TrimSuffix(string(out), "\n"), "\t")
	b, err := strconv.ParseUint(behindAhead[0], 10, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse behindahead output %q", out)
	}
	a, err := strconv.ParseUint(behindAhead[1], 10, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse behindahead output %q", out)
	}
	return &gitdomain.BehindAhead{Behind: uint32(b), Ahead: uint32(a)}, nil
}

func (g *gitCLIBackend) FirstEverCommit(ctx context.Context) (api.CommitID, error) {
	rc, err := g.NewCommand(ctx, WithArguments("rev-list", "--reverse", "--date-order", "--max-parents=0", "HEAD"))
	if err != nil {
		return "", err
	}
	defer rc.Close()

	out, err := io.ReadAll(rc)
	if err != nil {
		var cmdFailedErr *CommandFailedError
		if errors.As(err, &cmdFailedErr) {
			if cmdFailedErr.ExitStatus == 129 && bytes.Contains(cmdFailedErr.Stderr, []byte(revListUsageString)) {
				// If the error is due to an empty repository, return a sentinel error.
				e := &gitdomain.RevisionNotFoundError{
					Repo: g.repoName,
					Spec: "HEAD",
				}
				return "", e
			}
		}

		return "", errors.Wrap(err, "git rev-list command failed")
	}

	lines := bytes.TrimSpace(out)
	tokens := bytes.SplitN(lines, []byte("\n"), 2)
	if len(tokens) == 0 {
		return "", errors.New("FirstEverCommit returned no revisions")
	}
	first := tokens[0]
	id := api.CommitID(bytes.TrimSpace(first))
	return id, nil
}

const revListUsageString = `usage: git rev-list [<options>] <commit>... [--] [<path>...]`

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
	defer func() {
		closeErr := it.Close()
		if err == nil {
			err = closeErr
		}
	}()

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
	if err := checkSpecArgSafety(string(commit)); err != nil {
		return nil, err
	}

	if path != "" {
		// Trailing slash is necessary to ls-tree under the dir (not just
		// to list the dir's tree entry in its parent dir).
		path = filepath.Clean(rel(path)) + "/"
	}

	return g.lsTree(ctx, commit, path, recursive)
}

func (g *gitCLIBackend) lsTree(ctx context.Context, commit api.CommitID, path string, recurse bool) (_ git.ReadDirIterator, err error) {
	// Note: We don't call filepath.Clean(path) because ReadDir needs to pass
	// path with a trailing slash.

	args := []string{
		"ls-tree",
		"--long", // show size
		"--full-name",
		"-z",
		string(commit),
	}
	if recurse {
		args = append(args, "-r", "-t") // -t: Show tree entries even when going to recurse them.
	}
	if path != "" {
		// Note: We need to use :(literal) here to prevent glob expansion which
		// would lead to incorrect results.
		args = append(args, "--", pathspecLiteral(filepath.ToSlash(path)))
	}

	r, err := g.NewCommand(ctx, WithArguments(args...))
	if err != nil {
		return nil, err
	}

	sc := bufio.NewScanner(r)
	sc.Split(byteutils.ScanNullLines)

	return &readDirIterator{
		ctx:      ctx,
		g:        g,
		sc:       sc,
		repoName: g.repoName,
		commit:   commit,
		path:     path,
		r:        r,
	}, nil
}

type readDirIterator struct {
	ctx      context.Context
	g        *gitCLIBackend
	sc       *bufio.Scanner
	repoName api.RepoName
	commit   api.CommitID
	path     string
	fdsSeen  int
	r        io.ReadCloser
}

func (it *readDirIterator) Next() (fs.FileInfo, error) {
	for it.sc.Scan() {
		line := it.sc.Bytes()
		if len(line) == 0 {
			continue
		}

		tabPos := bytes.IndexByte(line, '\t')
		if tabPos == -1 {
			return nil, errors.Errorf("invalid `git ls-tree` output: %q", line)
		}
		info := bytes.SplitN(line[:tabPos], []byte(" "), 4)

		if len(info) != 4 {
			return nil, errors.Errorf("invalid `git ls-tree` output: %q", line)
		}

		name := string(line[tabPos+1:])
		typ := info[1]
		sha := info[2]
		if !gitdomain.IsAbsoluteRevision(string(sha)) {
			return nil, errors.Errorf("invalid `git ls-tree` SHA output: %q", sha)
		}
		oid, err := decodeOID(api.CommitID(sha))
		if err != nil {
			return nil, err
		}

		sizeStr := string(bytes.TrimSpace(info[3]))
		var size int64
		if sizeStr != "-" {
			// Size of "-" indicates a dir or submodule.
			size, err = strconv.ParseInt(sizeStr, 10, 64)
			if err != nil || size < 0 {
				return nil, errors.Errorf("invalid `git ls-tree` size output: %q (error: %s)", sizeStr, err)
			}
		}

		var sys any
		modeVal, err := strconv.ParseInt(string(info[0]), 8, 32)
		if err != nil {
			return nil, err
		}

		loadModConf := sync.OnceValues(func() (config.Config, error) {
			return it.g.gitModulesConfig(it.ctx, it.commit)
		})

		mode := os.FileMode(modeVal)
		switch string(typ) {
		case "blob":
			if mode&gitdomain.ModeSymlink != 0 {
				mode = os.ModeSymlink
			} else {
				// Regular file.
				mode = mode | 0o644
			}
		case "commit":
			mode = gitdomain.ModeSubmodule

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

		it.fdsSeen++

		return &fileutil.FileInfo{
			Name_: name, // full path relative to root (not just basename)
			Mode_: mode,
			Size_: size,
			Sys_:  sys,
		}, nil
	}

	if err := it.sc.Err(); err != nil {
		var cfe *CommandFailedError
		if errors.As(err, &cfe) {
			if bytes.Contains(cfe.Stderr, []byte("exists on disk, but not in")) {
				return nil, &os.PathError{Op: "ls-tree", Path: filepath.ToSlash(it.path), Err: os.ErrNotExist}
			}
			if cfe.ExitStatus == 128 && bytes.Contains(cfe.Stderr, []byte("fatal: not a tree object")) {
				return nil, &gitdomain.RevisionNotFoundError{Repo: it.repoName, Spec: string(it.commit)}
			}
			if cfe.ExitStatus == 128 && bytes.Contains(cfe.Stderr, []byte("fatal: Not a valid object name")) {
				return nil, &gitdomain.RevisionNotFoundError{Repo: it.repoName, Spec: string(it.commit)}
			}
		}
		return nil, err
	}

	// If we are listing the empty root tree, we will have no output.
	if it.fdsSeen == 0 && filepath.Clean(it.path) != "." {
		return nil, &os.PathError{Op: "git ls-tree", Path: it.path, Err: os.ErrNotExist}
	}

	return nil, io.EOF
}

func (it *readDirIterator) Close() error {
	if err := it.r.Close(); err != nil {
		var cfe *CommandFailedError
		if errors.As(err, &cfe) {
			if bytes.Contains(cfe.Stderr, []byte("exists on disk, but not in")) {
				return &os.PathError{Op: "ls-tree", Path: filepath.ToSlash(it.path), Err: os.ErrNotExist}
			}
			if cfe.ExitStatus == 128 && bytes.Contains(cfe.Stderr, []byte("fatal: not a tree object")) {
				return &gitdomain.RevisionNotFoundError{Repo: it.repoName, Spec: string(it.commit)}
			}
			if cfe.ExitStatus == 128 && bytes.Contains(cfe.Stderr, []byte("fatal: Not a valid object name")) {
				return &gitdomain.RevisionNotFoundError{Repo: it.repoName, Spec: string(it.commit)}
			}
		}
		return err
	}

	return nil
}

// gitModulesConfig returns the gitmodules configuration for the given commit.
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

func (g *gitCLIBackend) PruneObjects(ctx context.Context, expiry time.Time) error {
	r, err := g.NewCommand(ctx, WithArguments("prune", fmt.Sprintf("--expire=%s", expiry.Format(time.RFC3339))))
	if err != nil {
		return err
	}
	// There is no interesting output on stdout, so just discard it.
	_, err = io.Copy(io.Discard, r)
	return errors.Append(err, r.Close())
}

func (g *gitCLIBackend) PackObjects(ctx context.Context) error {
	r, err := g.NewCommand(ctx, WithArguments(
		"pack-objects",
		// We ask git-pack-objects(1) to pack loose unreachable
		// objects. This implies `--revs`, but as we don't supply
		// any revisions via stdin all objects will be considered
		// unreachable. The effect is that we simply pack all loose
		// objects into a new packfile, regardless of whether they
		// are reachable or not.
		"--pack-loose-unreachable",
		// Skip any objects which are part of an alternative object
		// directory.
		"--local",
		// Only pack objects which are not yet part of a different,
		// local pack.
		"--incremental",
		// Only create the packfile if it would contain at least one
		// object.
		"--non-empty",
		// We don't care about any kind of progress meter.
		"--quiet",
		// We need to tell git-pack-objects(1) where to write the
		// new packfile and what prefix it should have. We of course
		// want to write it into the main object directory and have
		// the same "pack-" prefix like normal packfiles would.
		g.dir.Path("objects", "pack", "pack"),
	))
	if err != nil {
		return err
	}
	// There is no interesting output on stdout, so just discard it.
	_, err = io.Copy(io.Discard, r)
	return errors.Append(err, r.Close())
}

func (g *gitCLIBackend) PrunePacked(ctx context.Context) error {
	r, err := g.NewCommand(ctx, WithArguments("prune-packed", "--quiet"))
	if err != nil {
		return err
	}
	// There is no interesting output on stdout, so just discard it.
	_, err = io.Copy(io.Discard, r)
	return errors.Append(err, r.Close())
}

func (g *gitCLIBackend) Repack(ctx context.Context, opts git.RepackOptions) error {
	args := []string{
		"-c", "repack.useDeltaIslands=true",
		"-c", fmt.Sprintf("repack.writeBitmaps=%s", strconv.FormatBool(opts.WriteBitmap)),
		"-c", "pack.writeBitmapLookupTable=true",
		"-c", "pack.island=r(e)fs/heads",
		"-c", "pack.island=r(e)fs/tags",
		"-c", "pack.islandCore=e",
		"repack",
	}

	if opts.Geometric {
		// We use a geometric factor `r`, which means that every successively larger
		// packfile must have at least `r` times the number of objects.
		//
		// This factor ultimately determines how many packfiles there can be at a
		// maximum in a repository for a given number of objects. The maximum number
		// of objects with `n` packfiles and a factor `r` is `(1 - r^n) / (1 - r)`.
		// E.g. with a factor of 4 and 10 packfiles, we can have at most 349,525
		// objects, with 16 packfiles we can have 1,431,655,765 objects. Contrary to
		// that, having a factor of 2 will translate to 1023 objects at 10 packfiles
		// and 65535 objects at 16 packfiles at a maximum.
		//
		// So what we're effectively choosing here is how often we need to repack
		// larger parts of the repository. The higher the factor the more we'll have
		// to repack as the packfiles will be larger. On the other hand, having a
		// smaller factor means we'll have to repack less objects as the slices we
		// need to repack will have less objects.
		//
		// The end result is a hybrid approach between incremental repacks and full
		// repacks: we won't typically repack the full repository, but only a subset
		// of packfiles.
		//
		// For now, we choose a geometric factor of two. Large repositories nowadays
		// typically have a few million objects, which would boil down to having at
		// most 32 packfiles in the repository. This number is not scientifically
		// chosen though any may be changed at a later point in time.
		args = append(args, "--geometric", "2")
	}

	if opts.Local {
		args = append(args, "-l")
	}

	if opts.DeleteLoose {
		args = append(args, "-d")
	}

	if opts.Cruft {
		args = append(args, "--cruft")
		args = append(args, "--pack-kept-objects")
	}

	if !opts.CruftExpiration.IsZero() {
		args = append(args, fmt.Sprintf("--cruft-expiration=%s", opts.CruftExpiration.Format(time.RFC3339)))
	}

	if opts.WriteMultiPackIndex {
		args = append(args, "--write-midx")
	}

	r, err := g.NewCommand(ctx, WithArguments(args...))
	if err != nil {
		return err
	}
	// There is no interesting output on stdout, so just discard it.
	_, err = io.Copy(io.Discard, r)
	return errors.Append(err, r.Close())
}
