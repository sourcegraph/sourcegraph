package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/hostname"
	"github.com/sourcegraph/sourcegraph/internal/version"
)

func main() {
	liblog := log.Init(log.Resource{
		Name:       env.MyName,
		Version:    version.Version(),
		InstanceID: hostname.Get(),
	})
	defer liblog.Sync()

	// s := memory.NewStorage()
	// rem := libgit.NewRemote(s, &config.RemoteConfig{
	// 	Name: "origin",
	// 	URLs: []string{"https://github.com/sourcegraph/private-automation-testing"},
	// })
	// Auth := &http.BasicAuth{Username: "git", Password: "TEST"}
	// err := rem.FetchContext(context.Background(), &libgit.FetchOptions{
	// 	Auth:     Auth,
	// 	RefSpecs: []config.RefSpec{config.DefaultPushRefSpec},
	// })
	// if err != nil {
	// 	panic(err.Error())
	// }
	// r, err := libgit.Open(s, nil)
	// if err != nil {
	// 	panic(err.Error())
	// }
	// ref, err := r.Head()
	// if err != nil {
	// 	panic(err.Error())
	// }
	// fmt.Printf(ref.Hash().String())

	// time, _ := git.LatestCommitTimestamp(log.Scoped("tester", ""), common.GitDir("/Users/erik/Code/sourcegraph/sourcegraph/.git"))
	// fmt.Printf("Latest commit timestamp for repo is %s %d\n", time, time.Unix())

	gitDir := common.GitDir("/Users/erik/Code/sourcegraph/sourcegraph/.git")

	gitStart := time.Now()
	f, err := os.CreateTemp("", "")
	if err != nil {
		panic(fmt.Sprintf("oh no failed to create temp file: %v", err))
	}
	f.Close()
	cmd := exec.Command("git", "archive", "--format=tar", "-o", f.Name(), "81126012e7d1d324801bd98ec5f64c8fc3b16769")
	gitDir.Set(cmd)
	if err := cmd.Run(); err != nil {
		panic(fmt.Sprintf("git archive failed: %v", err))
	}
	fmt.Printf("Git took %s\n", time.Since(gitStart))

	// ================================
	// ================================
	// ================================

	goGitStart := time.Now()
	pr, pw := io.Pipe()
	var wg sync.WaitGroup
	go func() {
		f, err := os.CreateTemp("", "")
		if err != nil {
			panic(fmt.Sprintf("oh no failed to create temp file: %v", err))
		}
		fmt.Printf("Writing to file %s\n", f.Name())

		_, err = io.Copy(f, pr)
		if err != nil {
			panic(fmt.Sprintf("oh no failed to write file: %v", err))
		}
		f.Close()
		pr.Close()
		wg.Done()
	}()
	wg.Add(1)

	err = git.Archive(context.Background(), gitDir, pw, git.Tar, "81126012e7d1d324801bd98ec5f64c8fc3b16769")
	if err != nil {
		panic(fmt.Sprintf("oh no failed to archive: %v", err))
	}
	pw.Close()
	wg.Wait()
	fmt.Printf("Go-Git took %s\n", time.Since(goGitStart))
	time.Sleep(10 * time.Second)
}
