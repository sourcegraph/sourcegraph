package gogit

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	sggit "github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func NewCachingBackendFactory(maxCacheSize int64) func(common.GitDir) (sggit.GitBackend, error) {
	// No need to handle the error here, the only way this function can fail is if the
	// size is < 0, which cannot happen.
	size := maxCacheSize / int64(fsCacheSize)
	if size < 2 {
		size = 2
	}
	cache, _ := lru.New[common.GitDir, *git.Repository](int(size))
	var cacheMu sync.Mutex
	return func(path common.GitDir) (sggit.GitBackend, error) {
		cacheMu.Lock()
		defer cacheMu.Unlock()

		r, ok := cache.Get(path)
		if ok {
			return NewBackend(r), nil
		}
		// TODO: Verify that gogit is actually concurrency safe. Otherwise,
		// we should just cache the billy FS.
		r, err := newRepository(path)
		if err != nil {
			return nil, err
		}
		cache.Add(path, r)
		return NewBackend(r), nil
	}
}

// 96 MB is the default for now. It seems rather high, let's see.
const fsCacheSize = 96 * cache.MiByte

func newRepository(path common.GitDir) (*git.Repository, error) {
	s := filesystem.NewStorage(osfs.New(path.Path()), cache.NewObjectLRU(fsCacheSize))
	return git.Open(s, nil)
}

func NewBackend(r *git.Repository) sggit.GitBackend {
	return &gogitBackend{repo: r}
}

type gogitBackend struct {
	repo *git.Repository
}

func (g *gogitBackend) Config() sggit.GitConfigBackend {
	return g
}

func (g *gogitBackend) GetObject(ctx context.Context, objectName string) (*gitdomain.GitObject, error) {
	h, err := g.repo.ResolveRevision(plumbing.Revision(objectName))
	if err != nil {
		return nil, err
	}

	o, err := g.repo.Object(plumbing.AnyObject, *h)
	if err != nil {
		return nil, err
	}

	var typ gitdomain.ObjectType

	switch o.Type() {
	case plumbing.InvalidObject:
		return nil, errors.Newf("invalid object type")
	case plumbing.CommitObject:
		typ = gitdomain.ObjectTypeCommit
	case plumbing.TreeObject:
		typ = gitdomain.ObjectTypeTree
	case plumbing.BlobObject:
		typ = gitdomain.ObjectTypeBlob
	case plumbing.TagObject:
		typ = gitdomain.ObjectTypeTag
	default:
		return nil, errors.Newf("unknown object type %s", o.Type())
	}

	return &gitdomain.GitObject{
		ID:   gitdomain.OID(*h),
		Type: typ,
	}, nil
}

func (g *gogitBackend) MergeBase(ctx context.Context, baseRevspec, headRevspec string) (api.CommitID, error) {
	baseRev, err := g.repo.ResolveRevision(plumbing.Revision(baseRevspec))
	if err != nil {
		return "", err
	}
	headRev, err := g.repo.ResolveRevision(plumbing.Revision(headRevspec))
	if err != nil {
		return "", err
	}
	base, err := g.repo.CommitObject(*baseRev)
	if err != nil {
		return "", err
	}
	head, err := g.repo.CommitObject(*headRev)
	if err != nil {
		return "", err
	}
	bases, err := base.MergeBase(head)
	if err != nil {
		return "", err
	}
	if len(bases) == 0 {
		return "", nil
	}
	return api.CommitID(bases[0].ID().String()), nil
}

func (g *gogitBackend) Blame(ctx context.Context, path string, opt sggit.BlameOptions) (sggit.BlameHunkReader, error) {
	commit, err := g.repo.CommitObject(plumbing.NewHash(string(opt.NewestCommit)))
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return nil, &gitdomain.RevisionNotFoundError{Repo: "therepo", Spec: string(opt.NewestCommit)}
		}
		return nil, err
	}

	blame, err := git.Blame(commit, path)
	if err != nil {
		return nil, err
	}

	return &blameHunkReader{lines: blame.Lines}, nil
}

type blameHunkReader struct {
	lines []*git.Line
	curr  int
}

func (g *blameHunkReader) Read() (*gitdomain.Hunk, error) {
	if g.curr >= len(g.lines) {
		return nil, io.EOF
	}
	l := g.lines[g.curr]
	g.curr++
	return &gitdomain.Hunk{
		StartLine: uint32(g.curr - 1),
		EndLine:   uint32(g.curr),
		Message:   "TODO",
		CommitID:  api.CommitID(l.Hash.String()),
		Filename:  "TODO",
		StartByte: 0, // TODO
		EndByte:   0, // TODO
		Author: gitdomain.Signature{
			Name:  l.AuthorName,
			Email: l.Author,
			Date:  l.Date,
		},
	}, nil
}

func (g *blameHunkReader) Close() error {
	return nil
}

func (g *gogitBackend) SymbolicRefHead(ctx context.Context, short bool) (string, error) {
	ref, err := g.repo.Head()
	if err != nil {
		return "", err
	}
	if short {
		return ref.Name().Short(), nil
	}
	return ref.Name().String(), nil
}

func (g *gogitBackend) RevParseHead(ctx context.Context) (api.CommitID, error) {
	ref, err := g.repo.Head()
	if err != nil {
		return "", err
	}
	if ref == nil {
		return "", &gitdomain.RevisionNotFoundError{Repo: "therepo", Spec: "HEAD"}
	}
	return api.CommitID(ref.Hash().String()), nil
}

func (g *gogitBackend) ReadFile(ctx context.Context, commit api.CommitID, path string) (io.ReadCloser, error) {
	c, err := g.repo.CommitObject(plumbing.NewHash(string(commit)))
	if err != nil {
		return nil, err
	}
	t, err := c.Tree()
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return nil, &gitdomain.RevisionNotFoundError{Repo: "todo reponame", Spec: string(commit)}
		}
		return nil, err
	}
	f, err := t.File(path)
	if err != nil {
		if errors.Is(err, object.ErrFileNotFound) {
			return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
		}
		return nil, err
	}
	return f.Reader()
}

func (g *gogitBackend) GetCommit(ctx context.Context, commit api.CommitID, includeModifiedFiles bool) (*sggit.GitCommitWithFiles, error) {
	c, err := g.repo.CommitObject(plumbing.NewHash(string(commit)))
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return nil, &gitdomain.RevisionNotFoundError{Repo: "todo reponame", Spec: string(commit)}
		}
		return nil, err
	}
	parents := make([]api.CommitID, len(c.ParentHashes))
	for i, p := range c.ParentHashes {
		parents[i] = api.CommitID(p.String())
	}
	res := &sggit.GitCommitWithFiles{
		Commit: &gitdomain.Commit{
			ID:      api.CommitID(c.ID().String()),
			Message: gitdomain.Message(c.Message),
			Parents: parents,
			Author: gitdomain.Signature{
				Name:  c.Author.Name,
				Email: c.Author.Email,
				Date:  c.Author.When,
			},
			Committer: &gitdomain.Signature{
				Name:  c.Committer.Name,
				Email: c.Committer.Email,
				Date:  c.Committer.When,
			},
		},
	}

	if includeModifiedFiles {
		t, err := c.Tree()
		if err != nil {
			return nil, err
		}
		// TODO: Support for all parents.
		parent, err := c.Parent(0)
		if err != nil {
			return nil, err
		}
		parentTree, err := parent.Tree()
		if err != nil {
			return nil, err
		}

		chs, err := t.Diff(parentTree)
		if err != nil {
			return nil, err
		}

		for _, ch := range chs {
			res.ModifiedFiles = append(res.ModifiedFiles, ch.To.Name)
		}
	}

	return res, nil
}

func (g *gogitBackend) Exec(ctx context.Context, args ...string) (io.ReadCloser, error) {
	return nil, errors.New("unsupported")
}

func (g *gogitBackend) Get(ctx context.Context, key string) (string, error) {
	section, subsection, field, err := splitSections(key)
	if err != nil {
		return "", err
	}

	cfg, err := g.repo.Config()
	if err != nil {
		return "", err
	}

	sec := cfg.Raw.Section(section)
	if subsection != config.NoSubsection {
		return sec.Subsection(subsection).Option(field), nil
	}

	return cfg.Raw.Section(section).Option(field), nil

}

func (g *gogitBackend) Set(ctx context.Context, key, value string) error {
	section, subsection, field, err := splitSections(key)
	if err != nil {
		return err
	}

	cfg, err := g.repo.Config()
	if err != nil {
		return err
	}

	cfg.Raw.SetOption(section, subsection, field, value)

	return g.repo.SetConfig(cfg)
}

func (g *gogitBackend) Unset(ctx context.Context, key string) error {
	section, subsection, field, err := splitSections(key)
	if err != nil {
		return err
	}

	cfg, err := g.repo.Config()
	if err != nil {
		return err
	}

	if subsection != config.NoSubsection {
		cfg.Raw.Section(section).Subsection(subsection).RemoveOption(field)
	} else {
		cfg.Raw.Section(section).RemoveOption(field)
	}

	return g.repo.SetConfig(cfg)
}

func splitSections(key string) (section, subsection, field string, err error) {
	s := strings.Split(key, ".")
	if len(s) < 2 {
		return "", "", "", errors.New("key must contain section and field separated by '.'")
	}
	if len(s) > 3 {
		return "", "", "", errors.New("key must contain at most section, one subsection, and field separated by '.'")
	}

	section, field = s[0], s[len(s)-1]
	if len(s) == 3 {
		subsection = s[1]
	} else {
		subsection = config.NoSubsection
	}

	return section, subsection, field, nil
}
