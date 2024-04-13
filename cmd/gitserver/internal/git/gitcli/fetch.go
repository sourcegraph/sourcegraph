package gitcli

import (
	"bufio"
	"bytes"
	"context"
	"io"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/urlredactor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) Fetch(ctx context.Context, opt git.FetchOptions) (git.RefUpdateIterator, io.Reader, error) {
	redactor := urlredactor.New(opt.RemoteURL)

	args, env := buildFetchArgs(opt)
	// see issue #7322: skip LFS content in repositories with Git LFS configured.
	env = append(env, "GIT_LFS_SKIP_SMUDGE=1")

	stderrR, stderrW := io.Pipe()
	r, err := g.NewCommand(ctx,
		WithArguments(args...),
		WithEnv(env...),
		WithOutputRedactor(redactor.Redact),
		WithStderr(stderrW),
	)
	if err != nil {
		return nil, nil, err
	}

	return &refUpdateIterator{
		stdout: r,
		onCancel: func() error {
			return errors.Append(stderrR.Close(), stderrW.Close())
		},
		sc: bufio.NewScanner(r),
	}, stderrR, nil
}

type refUpdateIterator struct {
	stdout   io.ReadCloser
	sc       *bufio.Scanner
	onCancel func() error
}

func (i *refUpdateIterator) Next() (git.RefUpdate, error) {
	for i.sc.Scan() {
		if len(i.sc.Bytes()) == 0 {
			continue
		}
		return parseRefUpdateLine(i.sc.Bytes())
	}

	if err := i.sc.Err(); err != nil {
		return git.RefUpdate{}, err
	}

	return git.RefUpdate{}, io.EOF
}

func (i *refUpdateIterator) Close() error {
	cancelErr := i.onCancel()
	err := i.stdout.Close()
	if cancelErr != nil {
		err = errors.Append(err, cancelErr)
	}
	return err
}

func buildFetchArgs(opt git.FetchOptions) (args, env []string) {
	env = []string{
		// disable password prompt
		"GIT_ASKPASS=true",
		// Suppress asking to add SSH host key to known_hosts (which will hang because
		// the command is non-interactive).
		//
		// And set a timeout to avoid indefinite hangs if the server is unreachable.
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes -o ConnectTimeout=30",
		// Identify HTTP requests with a user agent. Please keep the git/ prefix because GitHub breaks the protocol v2
		// negotiation of clone URLs without a `.git` suffix (which we use) without it. Don't ask.
		"GIT_HTTP_USER_AGENT=git/Sourcegraph-Bot",
	}

	if opt.TLSConfig.SSLNoVerify {
		env = append(env, "GIT_SSL_NO_VERIFY=true")
	}
	if opt.TLSConfig.SSLCAInfo != "" {
		env = append(env, "GIT_SSL_CAINFO="+opt.TLSConfig.SSLCAInfo)
	}

	// If we have creds in the URL, pass them in via the credHelper instead of
	// as part of the URL, because args are visible in `ps` output, leaking the
	// credentials easily.
	remoteURLArg := opt.RemoteURL.String()
	credentialHelper := []string{}
	password, ok := opt.RemoteURL.User.Password()
	if ok && !opt.RemoteURL.IsSSH() {
		// Remove the user section from the remoteURL so that git consults credential
		// helpers for the username/password.
		ru := *opt.RemoteURL
		ru.User = nil
		remoteURLArg = ru.String()

		// Next up, add out credential helper.
		// Note: We add an ADDITIONAL credential helper here, the previous
		// one is just unsetting any existing ones.
		credentialHelper = []string{"-c", "credential.helper=!f() { echo \"username=$GIT_SG_USERNAME\npassword=$GIT_SG_PASSWORD\"; }; f"}
		env = append(env,
			"GIT_SG_USERNAME="+opt.RemoteURL.User.Username(),
			"GIT_SG_PASSWORD="+password,
		)
	}

	args = []string{
		// Unset credential helper because the command is non-interactive.
		// Even when we pass a second credential helper for HTTP credentials,
		// we will need this. Otherwise, the original credential helper will be used
		// as well.
		"-c", "credential.helper=",
	}
	args = append(args, credentialHelper...)
	args = append(args,
		"-c", "protocol.version=2",
		"fetch",
		"--progress",
		"--prune",
		"--porcelain",
		remoteURLArg,
	)

	return args, env
}

func parseRefUpdateLine(line []byte) (u git.RefUpdate, _ error) {
	line = bytes.TrimSpace(line)
	// format:
	// <flag> <old-object-id> <new-object-id> <local-reference>
	if len(line) == 0 {
		return git.RefUpdate{}, errors.New("empty git ref update output")
	}
	if line[0] == ' ' {
		u.Type = git.RefUpdateTypeFastForwardUpdate
		line[0] = 'x'
	}
	parts := bytes.Fields(line)
	if len(parts) != 4 {
		return git.RefUpdate{}, errors.Newf("invalid ref update format, expected exactly 4 fields %q", line)
	}

	if line[0] != 'x' {
		switch git.RefUpdateType(line[0]) {
		case git.RefUpdateTypeFastForwardUpdate:
			u.Type = git.RefUpdateTypeFastForwardUpdate
		case git.RefUpdateTypeForcedUpdate:
			u.Type = git.RefUpdateTypeForcedUpdate
		case git.RefUpdateTypePruned:
			u.Type = git.RefUpdateTypePruned
		case git.RefUpdateTypeTagUpdate:
			u.Type = git.RefUpdateTypeTagUpdate
		case git.RefUpdateTypeNewRef:
			u.Type = git.RefUpdateTypeNewRef
		case git.RefUpdateTypeFailed:
			u.Type = git.RefUpdateTypeFailed
		case git.RefUpdateTypeUnchanged:
			u.Type = git.RefUpdateTypeUnchanged
		default:
			return git.RefUpdate{}, errors.Newf("invalid ref update type %q", line[0])
		}
	}
	u.OldSHA = api.CommitID(parts[1])
	u.NewSHA = api.CommitID(parts[2])
	u.LocalReference = string(parts[3])

	return u, nil
}
