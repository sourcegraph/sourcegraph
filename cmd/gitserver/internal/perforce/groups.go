package perforce

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// P4GroupMembersArguments are the arguments for P4GroupMembers.
type P4GroupMembersArguments struct {
	// P4PORT is the address of the Perforce server.
	P4Port string

	// P4User is the Perforce username to authenticate with.
	P4User string
	// P4Passwd is the Perforce password to authenticate with.
	P4Passwd string

	// Group is the name of the group to get members for.
	Group string
}

// P4GroupMembers returns all usernames that are members of the given group.
func P4GroupMembers(ctx context.Context, fs gitserverfs.FS, args P4GroupMembersArguments) ([]string, error) {
	options := []P4OptionFunc{
		WithAuthentication(args.P4User, args.P4Passwd),
		WithHost(args.P4Port),
	}

	options = append(options, WithArguments("-Mj", "-ztag", "group", "-o", args.Group))

	p4home, err := fs.P4HomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create p4home dir")
	}

	scratchDir, err := fs.TempDir("p4-group-")
	if err != nil {
		return nil, errors.Wrap(err, "could not create temp dir to invoke 'p4 group'")
	}
	defer os.Remove(scratchDir)

	cmd := NewBaseCommand(ctx, p4home, scratchDir, options...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctxerr := ctx.Err(); ctxerr != nil {
			err = errors.Wrap(ctxerr, "p4 group context error")
		}

		if len(out) > 0 {
			err = errors.Wrapf(err, `failed to run command "p4 group" (output follows)\n\n%s`, specifyCommandInErrorMessage(string(out), cmd.Unwrap()))
		}

		return nil, err
	}

	if len(out) == 0 {
		// no error, but also no members. Maybe the group doesn't have any members?
		return nil, nil
	}

	return parseP4GroupMembers(out)
}

func parseP4GroupMembers(out []byte) ([]string, error) {
	var jsonGroup map[string]any
	err := json.Unmarshal(out, &jsonGroup)
	if err != nil {
		return nil, errors.Wrap(err, "malformed output from p4 group")
	}

	users := make([]string, 0)
	currentUserIdx := 0
	for {
		user, ok := jsonGroup[fmt.Sprintf("Users%d", currentUserIdx)]
		currentUserIdx++
		if !ok {
			break
		}
		username, ok := user.(string)
		if !ok {
			continue
		}
		users = append(users, username)
	}

	return users, nil
}
