package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server/sshagent"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server/urlredactor"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/perforce"
	"github.com/sourcegraph/sourcegraph/internal/unpack"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var patchID uint64

func (s *Server) handleCreateCommitFromPatchBinary(w http.ResponseWriter, r *http.Request) {
	var req protocol.CreateCommitFromPatchRequest
	var resp protocol.CreateCommitFromPatchResponse
	var status int

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp := new(protocol.CreateCommitFromPatchResponse)
		resp.SetError("", "", "", errors.Wrap(err, "decoding CreateCommitFromPatchRequest"))
		status = http.StatusBadRequest
	} else {
		status, resp = s.createCommitFromPatch(r.Context(), req)
	}

	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) createCommitFromPatch(ctx context.Context, req protocol.CreateCommitFromPatchRequest) (int, protocol.CreateCommitFromPatchResponse) {
	logger := s.Logger.Scoped("createCommitFromPatch", "").
		With(
			log.String("repo", string(req.Repo)),
			log.String("baseCommit", string(req.BaseCommit)),
			log.String("targetRef", req.TargetRef),
		)

	var resp protocol.CreateCommitFromPatchResponse

	repo := string(protocol.NormalizeRepo(req.Repo))
	repoDir := filepath.Join(s.ReposDir, repo)
	repoGitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(repoGitDir); os.IsNotExist(err) {
		repoGitDir = filepath.Join(s.ReposDir, repo)
		if _, err := os.Stat(repoGitDir); os.IsNotExist(err) {
			resp.SetError(repo, "", "", errors.Wrap(err, "gitserver: repo does not exist"))
			return http.StatusInternalServerError, resp
		}
	}

	var (
		remoteURL *vcs.URL
		err       error
	)

	if req.Push != nil && req.Push.RemoteURL != "" {
		remoteURL, err = vcs.ParseURL(req.Push.RemoteURL)
	} else {
		remoteURL, err = s.getRemoteURL(ctx, req.Repo)
	}

	ref := req.TargetRef
	// If the push is to a Gerrit project,we need to push to a magic ref.
	if req.PushRef != nil && *req.PushRef != "" {
		ref = *req.PushRef
	}
	if req.UniqueRef {
		refs, err := s.repoRemoteRefs(ctx, remoteURL, repo, ref)
		if err != nil {
			logger.Error("Failed to get remote refs", log.Error(err))
			resp.SetError(repo, "", "", errors.Wrap(err, "repoRemoteRefs"))
			return http.StatusInternalServerError, resp
		}

		retry := 1
		tmp := ref
		for {
			if _, ok := refs[tmp]; !ok {
				break
			}
			tmp = ref + "-" + strconv.Itoa(retry)
			retry++
		}
		ref = tmp
	}

	if req.Push != nil && req.PushRef == nil {
		ref = ensureRefPrefix(ref)
	}

	if err != nil {
		logger.Error("Failed to get remote URL", log.Error(err))
		resp.SetError(repo, "", "", errors.Wrap(err, "repoRemoteURL"))
		return http.StatusInternalServerError, resp
	}

	redactor := urlredactor.New(remoteURL)
	defer func() {
		if resp.Error != nil {
			resp.Error.Command = redactor.Redact(resp.Error.Command)
			resp.Error.CombinedOutput = redactor.Redact(resp.Error.CombinedOutput)
			if resp.Error.InternalError != "" {
				resp.Error.InternalError = redactor.Redact(resp.Error.InternalError)
			}
		}
	}()

	// Ensure tmp directory exists
	tmpRepoDir, err := tempDir(s.ReposDir, "patch-repo-")
	if err != nil {
		resp.SetError(repo, "", "", errors.Wrap(err, "gitserver: make tmp repo"))
		return http.StatusInternalServerError, resp
	}
	defer cleanUpTmpRepo(logger, tmpRepoDir)

	argsToString := func(args []string) string {
		return strings.Join(args, " ")
	}

	// Temporary logging command wrapper
	prefix := fmt.Sprintf("%d %s ", atomic.AddUint64(&patchID, 1), repo)
	run := func(cmd *exec.Cmd, reason string) ([]byte, error) {
		if !gitdomain.IsAllowedGitCmd(logger, cmd.Args[1:], repoDir) {
			return nil, errors.New("command not on allow list")
		}

		t := time.Now()

		// runRemoteGitCommand since one of our commands could be git push
		out, err := runRemoteGitCommand(ctx, s.RecordingCommandFactory.Wrap(ctx, s.Logger, cmd), true, nil)
		logger := logger.With(
			log.String("prefix", prefix),
			log.String("command", redactor.Redact(argsToString(cmd.Args))),
			log.Duration("duration", time.Since(t)),
			log.String("output", string(out)),
		)

		if err != nil {
			resp.SetError(repo, argsToString(cmd.Args), string(out), errors.Wrap(err, "gitserver: "+reason))
			logger.Warn("command failed", log.Error(err))
		} else {
			logger.Info("command ran successfully")
		}
		return out, err
	}

	tmpGitPathEnv := "GIT_DIR=" + filepath.Join(tmpRepoDir, ".git")

	tmpObjectsDir := filepath.Join(tmpRepoDir, ".git", "objects")
	repoObjectsDir := filepath.Join(repoGitDir, "objects")

	altObjectsEnv := "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + repoObjectsDir

	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tmpRepoDir
	cmd.Env = append(os.Environ(), tmpGitPathEnv)

	if _, err := run(cmd, "init tmp repo"); err != nil {
		return http.StatusInternalServerError, resp
	}

	cmd = exec.CommandContext(ctx, "git", "reset", "-q", string(req.BaseCommit))
	cmd.Dir = tmpRepoDir
	cmd.Env = append(os.Environ(), tmpGitPathEnv, altObjectsEnv)

	if out, err := run(cmd, "basing staging on base rev"); err != nil {
		logger.Error("Failed to base the temporary repo on the base revision",
			log.String("output", string(out)),
		)
		return http.StatusInternalServerError, resp
	}

	applyArgs := append([]string{"apply", "--cached"}, req.GitApplyArgs...)

	cmd = exec.CommandContext(ctx, "git", applyArgs...)
	cmd.Dir = tmpRepoDir
	cmd.Env = append(os.Environ(), tmpGitPathEnv, altObjectsEnv)
	cmd.Stdin = bytes.NewReader(req.Patch)

	if out, err := run(cmd, "applying patch"); err != nil {
		logger.Error("Failed to apply patch", log.String("output", string(out)))
		return http.StatusBadRequest, resp
	}

	messages := req.CommitInfo.Messages
	if len(messages) == 0 {
		messages = []string{"<Sourcegraph> Creating commit from patch"}
	}
	authorName := req.CommitInfo.AuthorName
	if authorName == "" {
		authorName = "Sourcegraph"
	}
	authorEmail := req.CommitInfo.AuthorEmail
	if authorEmail == "" {
		authorEmail = "support@sourcegraph.com"
	}
	committerName := req.CommitInfo.CommitterName
	if committerName == "" {
		committerName = authorName
	}
	committerEmail := req.CommitInfo.CommitterEmail
	if committerEmail == "" {
		committerEmail = authorEmail
	}

	// Commit messages can be arbitrary strings, so using `-m` runs into problems.
	// Instead, feed the commit messages to stdin.
	cmd = exec.CommandContext(ctx, "git", "commit", "-F", "-")
	// NOTE: join messages with a blank line in between ("\n\n")
	// because the previous behavior was to use multiple -m arguments,
	// which concatenate with a blank line in between.
	// Gerrit is the only code host that uses multiple messages at the moment.
	cmd.Stdin = strings.NewReader(strings.Join(messages, "\n\n"))

	cmd.Dir = tmpRepoDir
	cmd.Env = append(os.Environ(), []string{
		tmpGitPathEnv,
		altObjectsEnv,
		fmt.Sprintf("GIT_COMMITTER_NAME=%s", committerName),
		fmt.Sprintf("GIT_COMMITTER_EMAIL=%s", committerEmail),
		fmt.Sprintf("GIT_AUTHOR_NAME=%s", authorName),
		fmt.Sprintf("GIT_AUTHOR_EMAIL=%s", authorEmail),
		fmt.Sprintf("GIT_COMMITTER_DATE=%v", req.CommitInfo.Date),
		fmt.Sprintf("GIT_AUTHOR_DATE=%v", req.CommitInfo.Date),
	}...)

	if out, err := run(cmd, "committing patch"); err != nil {
		logger.Error("Failed to commit patch.", log.String("output", string(out)))
		return http.StatusInternalServerError, resp
	}

	cmd = exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = tmpRepoDir
	cmd.Env = append(os.Environ(), tmpGitPathEnv, altObjectsEnv)

	// We don't use 'run' here as we only want stdout
	out, err := cmd.Output()
	if err != nil {
		resp.SetError(repo, argsToString(cmd.Args), string(out), errors.Wrap(err, "gitserver: retrieving new commit id"))
		return http.StatusInternalServerError, resp
	}
	cmtHash := strings.TrimSpace(string(out))

	// Move objects from tmpObjectsDir to repoObjectsDir.
	err = filepath.Walk(tmpObjectsDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(tmpObjectsDir, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(repoObjectsDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
			return err
		}
		// do the actual move. If dst exists we can ignore the error since it
		// will contain the same content (content addressable FTW).
		if err := os.Rename(path, dst); err != nil && !os.IsExist(err) {
			return err
		}
		return nil
	})
	if err != nil {
		resp.SetError(repo, "", "", errors.Wrap(err, "copying git objects"))
		return http.StatusInternalServerError, resp
	}

	if req.Push != nil {
		if remoteURL.Scheme == "perforce" {
			// the remote URL is a Perforce URL
			// shelve the changelist instead of pushing to a Git host
			cid, err := s.shelveChangelist(ctx, req, cmtHash, remoteURL, tmpGitPathEnv, altObjectsEnv)
			if err != nil {
				resp.SetError(repo, "", "", err)
				return http.StatusInternalServerError, resp
			}

			resp.ChangelistId = cid
		} else {
			cmd = exec.CommandContext(ctx, "git", "push", "--force", remoteURL.String(), fmt.Sprintf("%s:%s", cmtHash, ref))
			cmd.Dir = repoGitDir

			// If the protocol is SSH and a private key was given, we want to
			// use it for communication with the code host.
			if remoteURL.IsSSH() && req.Push.PrivateKey != "" && req.Push.Passphrase != "" {
				// We set up an agent here, which sets up a socket that can be provided to
				// SSH via the $SSH_AUTH_SOCK environment variable and the goroutine to drive
				// it in the background.
				// This is used to pass the private key to be used when pushing to the remote,
				// without the need to store it on the disk.
				agent, err := sshagent.New(logger, []byte(req.Push.PrivateKey), []byte(req.Push.Passphrase))
				if err != nil {
					resp.SetError(repo, "", "", errors.Wrap(err, "gitserver: error creating ssh-agent"))
					return http.StatusInternalServerError, resp
				}
				go agent.Listen()
				// Make sure we shut this down once we're done.
				defer agent.Close()

				cmd.Env = append(
					os.Environ(),
					[]string{
						fmt.Sprintf("SSH_AUTH_SOCK=%s", agent.Socket()),
					}...,
				)
			}

			if out, err = run(cmd, "pushing ref"); err != nil {
				logger.Error("Failed to push", log.String("commit", cmtHash), log.String("output", string(out)))
				return http.StatusInternalServerError, resp
			}
		}
	}
	resp.Rev = "refs/" + strings.TrimPrefix(ref, "refs/")

	if req.PushRef == nil {
		cmd = exec.CommandContext(ctx, "git", "update-ref", "--", ref, cmtHash)
		cmd.Dir = repoGitDir

		if out, err = run(cmd, "creating ref"); err != nil {
			logger.Error("Failed to create ref for commit.", log.String("commit", cmtHash), log.String("output", string(out)))
			return http.StatusInternalServerError, resp
		}
	}

	return http.StatusOK, resp
}

// repoRemoteRefs returns a map containing ref + commit pairs from the
// remote Git repository starting with the specified prefix.
//
// The ref prefix `ref/<ref type>/` is stripped away from the returned
// refs.
func (s *Server) repoRemoteRefs(ctx context.Context, remoteURL *vcs.URL, repoName, prefix string) (map[string]string, error) {
	// The expected output of this git command is a list of:
	// <commit hash> <ref name>
	cmd := exec.Command("git", "ls-remote", remoteURL.String(), prefix+"*")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	r := urlredactor.New(remoteURL)
	_, err := runCommand(ctx, s.RecordingCommandFactory.WrapWithRepoName(ctx, nil, api.RepoName(repoName), cmd).WithRedactorFunc(r.Redact))
	if err != nil {
		stderr := stderr.Bytes()
		if len(stderr) > 200 {
			stderr = stderr[:200]
		}
		return nil, errors.Errorf("git %s failed: %s (%q)", cmd.Args, err, stderr)
	}

	refs := make(map[string]string)
	raw := stdout.String()
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, errors.Errorf("git %s failed (invalid output): %s", cmd.Args, line)
		}

		split := strings.SplitN(fields[1], "/", 3)
		if len(split) != 3 {
			return nil, errors.Errorf("git %s failed (invalid refname): %s", cmd.Args, fields[1])
		}

		refs[split[2]] = fields[0]
	}
	return refs, nil
}

func (s *Server) shelveChangelist(ctx context.Context, req protocol.CreateCommitFromPatchRequest, patchCommit string, remoteURL *vcs.URL, tmpGitPathEnv, altObjectsEnv string) (string, error) {

	repo := string(req.Repo)
	baseCommit := string(req.BaseCommit)

	p4user, p4passwd, p4host, p4depot, _, _ := decomposePerforceRemoteURL(remoteURL)

	if p4depot == "" {
		// the remoteURL was constructed without a path to indicate the depot
		// make a db call to fill that in
		remoteURL, err := s.getRemoteURL(ctx, req.Repo)
		if err != nil {
			return "", errors.Wrap(err, "failed getting a remote url")
		}
		// and decompose again
		_, _, _, p4depot, _, _ = decomposePerforceRemoteURL(remoteURL)
	}

	logger := s.Logger.Scoped("shelveChangelist", "").
		With(
			log.String("repo", repo),
			log.String("baseCommit", baseCommit),
			log.String("patchCommit", patchCommit),
			log.String("targetRef", req.TargetRef),
			log.String("depot", p4depot),
		)

	// use the name of the target branch as the perforce client name
	p4client := strings.TrimPrefix(req.TargetRef, "refs/heads/")

	// do all work in (another) temporary directory
	tmpClientDir, err := tempDir(s.ReposDir, "perforce-client-")
	if err != nil {
		return "", errors.Wrap(err, "gitserver: make tmp repo for Perforce client")
	}
	defer cleanUpTmpRepo(logger, tmpClientDir)

	// we'll need these environment variables for subsequent commands
	commonEnv := append(os.Environ(), []string{
		tmpGitPathEnv,
		altObjectsEnv,
		fmt.Sprintf("P4PORT=%s", p4host),
		fmt.Sprintf("P4USER=%s", p4user),
		fmt.Sprintf("P4PASSWD=%s", p4passwd),
		fmt.Sprintf("P4CLIENT=%s", p4client),
	}...)

	gitCmd := gitCommand{
		ctx:        ctx,
		workingDir: tmpClientDir,
		env:        commonEnv,
	}

	p4Cmd := p4Command{
		ctx:        ctx,
		workingDir: tmpClientDir,
		env:        commonEnv,
	}

	// check to see if there's a changelist for this target branch already
	cid, err := p4Cmd.changeListIDFromClientSpecName(p4client)
	if err == nil && cid != "" {
		return cid, nil
	}

	// extract the base changelist id from the base commit
	baseCID, err := gitCmd.getChangelistIdFromCommit(baseCommit)
	if err != nil {
		errorMessage := "unable to get the base changelist id"
		logger.Error(errorMessage, log.String("baseCommit", baseCommit), log.Error(err))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	}

	// get the list of files involved in the patch
	fileList, err := gitCmd.getListOfFilesInCommit(patchCommit)
	if err != nil {
		errorMessage := "failed listing files in base commit"
		logger.Error(errorMessage, log.String("patchCommit", patchCommit), log.Error(err))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	}

	// format a description for the client spec and the changelist
	// from the commit message(s)
	// be sure to indent lines so that it fits the Perforce form format
	desc := "batch change"
	if len(req.CommitInfo.Messages) > 0 {
		desc = strings.ReplaceAll(strings.Join(req.CommitInfo.Messages, "\n"), "\n", "\n\t")
	}

	// parse the depot path from the repo name
	// depot := strings.SplitN()

	// create a Perforce client spec to use for creating the changelist
	err = p4Cmd.createClientSpec(p4depot, p4client, p4user, desc)
	if err != nil {
		errorMessage := "error creating a client spec"
		logger.Error(errorMessage, log.String("output", digErrorMessage(err)), log.Error(errors.New(errorMessage)))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	}

	// get the files from the Perforce server
	// mark them for editing
	err = p4Cmd.cloneAndEditFiles(fileList, baseCID)
	if err != nil {
		errorMessage := "error getting files from depot"
		logger.Error(errorMessage, log.String("output", digErrorMessage(err)), log.Error(errors.New(errorMessage)))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	}

	// delete the files involved with the batch change because the untar will not overwrite existing files
	for _, fileName := range fileList {
		os.RemoveAll(filepath.Join(tmpClientDir, fileName))
	}

	// overlay with files from the commit
	// 1. create an archive from the commit
	// 2. pipe the archive to `tar -x` to extract it into the temp dir

	// archive the patch commit
	archiveCmd := gitCmd.commandContext("archive", "--format=tar", "--verbose", patchCommit)

	// connect the archive to the untar process
	stdout, err := archiveCmd.StdoutPipe()
	if err != nil {
		errorMessage := "unable to read changed files"
		logger.Error(errorMessage, log.Error(err))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	}

	reader := bufio.NewReader(stdout)

	// start the archive; it'll send stdout (the tar archive) to `unpack.Tar` via the `io.Reader`
	if err := archiveCmd.Start(); err != nil {
		errorMessage := "unable to read changed files"
		logger.Error(errorMessage, log.Error(err))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	}

	err = unpack.Tar(reader, tmpClientDir, unpack.Opts{SkipDuplicates: true})
	if err != nil {
		errorMessage := "unable to read changed files"
		logger.Error(errorMessage, log.Error(err))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	}

	// make sure the untar process completes before moving on
	if err := archiveCmd.Wait(); err != nil {
		errorMessage := "unable to overlay changed files"
		logger.Error(errorMessage, log.Error(err))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	}

	// ensure that there are changes to shelve
	if changes, err := p4Cmd.areThereChangedFiles(); err != nil {
		errorMessage := "unable to verify that there are changed files"
		logger.Error(errorMessage, log.String("output", digErrorMessage(err)), log.Error(errors.New(errorMessage)))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	} else if !changes {
		errorMessage := "no changes to shelve"
		logger.Error(errorMessage, log.Error(errors.New(errorMessage)))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	}

	// submit the changes as a shelved changelist

	// create a changelist form with the description
	changeForm, err := p4Cmd.generateChangeForm(desc)
	if err != nil {
		errorMessage := "failed generating a change form"
		logger.Error(errorMessage, log.String("output", digErrorMessage(err)), log.Error(errors.New(errorMessage)))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	}

	// feed the changelist form into `p4 shelve`
	// capture the output to parse for a changelist id
	cid, err = p4Cmd.shelveChangelist(changeForm)
	if err != nil {
		errorMessage := "failed shelving the changelist"
		logger.Error(errorMessage, log.String("output", digErrorMessage(err)), log.Error(errors.New(errorMessage)))
		return "", errors.Wrap(err, "gitserver: "+errorMessage)
	}

	// return the changelist id as a string - it'll be returned as a string to the caller in lieu of an int pointer
	// because protobuf doesn't do scalar pointers
	return cid, nil
}

type gitCommand struct {
	ctx        context.Context
	workingDir string
	env        []string
}

func (g gitCommand) commandContext(args ...string) *exec.Cmd {
	cmd := exec.CommandContext(g.ctx, "git", args...)
	cmd.Dir = g.workingDir
	cmd.Env = g.env
	return cmd
}

func (g gitCommand) getChangelistIdFromCommit(baseCommit string) (string, error) {
	// get the commit message from the base commit so that we can parse the base changelist id from it
	cmd := g.commandContext("show", "--no-patch", "--pretty=format:%B", baseCommit)
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "unable to retrieve base commit message")
	}
	// extract the base changelist id from the commit message
	baseCID, err := perforce.GetP4ChangelistID(string(out))
	if err != nil {
		return "", errors.Wrap(err, "unable to parse base changelist id from"+string(out))
	}
	return baseCID, nil
}

func (g gitCommand) getListOfFilesInCommit(patchCommit string) ([]string, error) {
	cmd := g.commandContext("diff-tree", "--no-commit-id", "--name-only", "-r", patchCommit)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "unable to retrieve files in base commit")
	}
	var fileList []string
	for _, file := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		file = strings.TrimSpace(file)
		if file != "" {
			fileList = append(fileList, file)
		}
	}
	if len(fileList) <= 0 {
		return nil, errors.New("no files in base commit")
	}
	return fileList, nil
}

type p4Command struct {
	ctx        context.Context
	workingDir string
	env        []string
}

func (p p4Command) commandContext(args ...string) *exec.Cmd {
	cmd := exec.CommandContext(p.ctx, "p4", args...)
	cmd.Dir = p.workingDir
	cmd.Env = p.env
	return cmd
}

// Uses `p4 changes` to see if there is a changelist already associated with the given client spec
func (p p4Command) changeListIDFromClientSpecName(p4client string) (string, error) {
	cmd := p.commandContext("changes",
		"-r",      // list in reverse order, which means that the given changelist id will be the first one listed
		"-m", "1", // limit output to one record, so that the given changelist is the only one listed
		"-l", // use a long listing, which includes the whole commit message
		"-c", p4client,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrap(err, string(out))
	}
	pcl, err := perforce.ParseChangelistOutput(string(out))
	if err != nil {
		return "", errors.Wrap(err, string(out))
	}
	return pcl.ID, nil
}

const clientSpecForm = `Client:	%s
Owner:	%s
Description:
	%s
Root:	%s
Options:	noallwrite noclobber nocompress unlocked nomodtime normdir
SubmitOptions:	submitunchanged
LineEnd:	local
View:	%s... //%s/...
`

// Uses `p4 client` to create a client spec used to sync files with the depot
// Returns an error if `p4 client` fails
// error -> error from exec.Cmd
// __|- error -> combined output from `p4 client`
func (p p4Command) createClientSpec(p4depot, p4client, p4user, description string) error {
	clientSpec := fmt.Sprintf(
		clientSpecForm,
		p4client,
		p4user,
		description,
		p.workingDir,
		p4depot,
		p4client,
	)
	cmd := p.commandContext("client", "-i")
	cmd.Stdin = bytes.NewReader([]byte(clientSpec))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// clones/downloads given files at the given base changelist
// returns an error if the sync or edit fails
// error -> error from exec.Cmd
// __|- error -> combined output from sync or edit
func (p p4Command) cloneAndEditFiles(fileList []string, baseChangelistId string) error {
	// want to specify the file at the base changelist revision
	// build a slice of file names with the changelist id appended
	filesWithCid := append([]string(nil), fileList...)
	for i := 0; i < len(filesWithCid); i++ {
		filesWithCid[i] = filesWithCid[i] + "@" + baseChangelistId
	}
	if err := p.cloneFiles(filesWithCid); err != nil {
		return err
	}
	if err := p.editFiles(fileList); err != nil {
		return err
	}
	return nil
}

// Uses `p4 sync` to copy/clone the given files from the depot to the local workspace
// Returns an error if `p4 sync` fails
// error -> error from exec.Cmd
// __|- error -> combined output from `p4 sync`
func (p p4Command) cloneFiles(filesWithCid []string) error {
	cmd := p.commandContext("sync")
	cmd.Args = append(cmd.Args, filesWithCid...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// Uses `p4 edit` to mark files as being edited
// Returns an error if `p4 edit` fails
// error -> error from exec.Cmd
// __|- error -> combined output from `p4 edit`
func (p p4Command) editFiles(fileList []string) error {
	cmd := p.commandContext("edit")
	cmd.Args = append(cmd.Args, fileList...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// Uses `p4 diff` to get a list of the files that have changed in the workspace
// Returns true if the file list has 1+ files in it
// Returns false if the file list is empty
// Returns an error if `p4 diff` fails
// error -> error from exec.Cmd
// __|- error -> combined output from `p4 diff`
func (p p4Command) areThereChangedFiles() (bool, error) {
	// use p4 diff to list the changes
	diffCmd := p.commandContext("diff", "-f", "-sa")

	// capture the output of `p4 diff` and count the lines
	// so that the output can be returned in an error message
	out, err := diffCmd.CombinedOutput()
	if err != nil {
		return false, errors.Wrap(err, string(out))
	}
	return len(strings.Split(string(out), "\n")) > 0, nil
}

// Uses `p4 change -o` to generate a form for the default changelist
// Injects the given `description` into the form.
// All lines of `description` after the first must begin with a tab character.
// Returns an error if `p4 change` fails
// error -> error from exec.Cmd
// __|- error -> combined output from `p4 change`
func (p p4Command) generateChangeForm(description string) (string, error) {
	cmd := p.commandContext("change", "-o")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrap(err, string(out))
	}
	// add the commit message to the change form
	return strings.Replace(string(out), "<enter description here>", description, 1), nil
}

var cidPattern = lazyregexp.New(`Change (\d+) files shelved`)

// Uses `p4 shelve` to shelve a changelist with the given form
// Returns an error if `p4 shelve` fails
// error -> error from exec.Cmd
// __|- error -> combined output from `p4 shelve`
// Returns an error if the output of `p4 shelve` does not contain a changelist id
// error -> "p4 shelve output does not contain a changelist id"
// __|- error -> combined output from `p4 shelve`
func (p p4Command) shelveChangelist(changeForm string) (string, error) {
	cmd := p.commandContext("shelve", "-i")
	changeBuffer := bytes.Buffer{}
	changeBuffer.Write([]byte(changeForm))
	cmd.Stdin = &changeBuffer
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrap(err, string(out))
	}
	matches := cidPattern.FindStringSubmatch(string(out))
	if len(matches) != 2 {
		return "", errors.Wrap(errors.New("p4 shelve output does not contain a changelist id"), string(out))
	}
	return matches[1], nil
}

// Return the deepest error message from a wrapped error.
// "Deepest" is somewhat facetious, as it does only one unwrap.
func digErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	innerError := errors.Unwrap(err)
	if innerError != nil {
		msg = innerError.Error()
	}
	return msg
}

func cleanUpTmpRepo(logger log.Logger, path string) {
	err := os.RemoveAll(path)
	if err != nil {
		logger.Warn("unable to clean up tmp repo", log.String("path", path), log.Error(err))
	}
}

// ensureRefPrefix checks whether the ref is a full ref and contains the
// "refs/heads" prefix (i.e. "refs/heads/master") or just an abbreviated ref
// (i.e. "master") and adds the "refs/heads/" prefix if the latter is the case.
//
// Copied from git package to avoid cycle import when testing git package.
func ensureRefPrefix(ref string) string {
	return "refs/heads/" + strings.TrimPrefix(ref, "refs/heads/")
}
