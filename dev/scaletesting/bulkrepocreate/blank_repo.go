package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sourcegraph/run"
)

type blankRepo struct {
	path     string
	login    string
	password string
	sync.Mutex
}

func newBlankRepo(login string, password string) (*blankRepo, error) {
	path, err := os.MkdirTemp(os.TempDir(), "sourcegraph-blank-repo")
	if err != nil {
		return nil, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return &blankRepo{
		login:    login,
		password: password,
		path:     path,
	}, nil
}

func (r *blankRepo) clone(ctx context.Context, num int) (*blankRepo, error) {
	folder := fmt.Sprintf("%s_%d", filepath.Base(r.path), num)
	newPath := filepath.Join(filepath.Dir(r.path), folder)
	err := run.Bash(ctx, "cp -R", r.path, newPath).Run().Wait()
	if err != nil {
		return nil, err
	}
	other := blankRepo{
		path:     newPath,
		login:    r.login,
		password: r.password,
	}
	return &other, nil
}

func (r *blankRepo) teardown() {
	_ = os.RemoveAll(r.path)
}

func (r *blankRepo) init(ctx context.Context) error {
	err := run.Bash(ctx, "git init").Dir(r.path).Run().Stream(os.Stdout)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(r.path, "README.md"), []byte("blank repo"), 0o755)
	if err != nil {
		return err
	}
	err = run.Bash(ctx, "git add README.md").Dir(r.path).Run().Stream(os.Stdout)
	if err != nil {
		return err
	}
	err = run.Bash(ctx, "git commit -m \"initial commit\"").Dir(r.path).Run().Wait()
	if err != nil {
		return err
	}
	return nil
}

func (r *blankRepo) addRemote(ctx context.Context, name string, gitURL string) error {
	r.Lock()
	defer r.Unlock()
	u, err := url.Parse(gitURL)
	if err != nil {
		return err
	}
	u.User = url.UserPassword(r.login, r.password)
	u.Scheme = "https"
	return run.Bash(ctx, "git remote add", name, u.String()).Dir(r.path).Run().Wait()
}

func (r *blankRepo) pushRemote(ctx context.Context, name string, retry int) error {
	var err error
	for range retry {
		err = r.doPushRemote(ctx, name)
		if err == nil {
			break
		}
	}
	return err
}

func (r *blankRepo) doPushRemote(ctx context.Context, name string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return run.Bash(ctx, "git push", name).Dir(r.path).Run().Wait()
}
