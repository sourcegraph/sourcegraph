package perforce

import (
	"context"
	"encoding/json"
	"os"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/byteutils"
	"github.com/sourcegraph/sourcegraph/internal/perforce"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type P4UsersArguments struct {
	// P4PORT is the address of the Perforce server.
	P4Port string
	// P4User is the Perforce username to authenticate with.
	P4User string
	// P4Passwd is the Perforce password to authenticate with.
	P4Passwd string
}

// P4Users returns all of users known to the Perforce server.
func P4Users(ctx context.Context, fs gitserverfs.FS, args P4UsersArguments) ([]perforce.User, error) {
	options := []P4OptionFunc{
		WithAuthentication(args.P4User, args.P4Passwd),
		WithHost(args.P4Port),
	}

	options = append(options, WithArguments("-Mj", "-ztag", "users"))

	p4home, err := fs.P4HomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create p4home dir")
	}

	scratchDir, err := fs.TempDir("p4-users-")
	if err != nil {
		return nil, errors.Wrap(err, "could not create temp dir to invoke 'p4 users'")
	}
	defer os.Remove(scratchDir)

	cmd := NewBaseCommand(ctx, p4home, scratchDir, options...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctxerr := ctx.Err(); ctxerr != nil {
			err = errors.Wrap(ctxerr, "p4 users context error")
		}
		if len(out) > 0 {
			err = errors.Wrapf(err, `failed to run command "p4 users" (output follows)\n\n%s`, specifyCommandInErrorMessage(string(out), cmd.Unwrap()))
		}
		return nil, err
	}

	if len(out) == 0 {
		// no error, but also no users. Maybe the user doesn't have access to any users?
		return nil, nil
	}

	users := make([]perforce.User, 0)
	lr := byteutils.NewLineReader(out)
	for lr.Scan() {
		line := lr.Line()
		// the output of `p4 -Mj -ztag users` is a series of JSON-formatted user definitions, one per line.
		var user perforceSJONUser
		err := json.Unmarshal(line, &user)
		if err != nil {
			return nil, errors.Wrap(err, "malformed output from p4 users")
		}
		users = append(users, perforce.User{
			Username: user.User,
			Email:    user.Email,
		})
	}

	return users, nil
}

type perforceUserType string

func (t perforceUserType) Valid() bool {
	switch t {
	case perforceUserTypeStandard,
		perforceUserTypeOperator,
		perforceUserTypeService:
		return true
	default:
		return false
	}
}

const (
	perforceUserTypeStandard perforceUserType = "standard"
	perforceUserTypeOperator perforceUserType = "operator"
	perforceUserTypeService  perforceUserType = "service"
)

// perforceSJONUser is a definition of a user that matches the format returned from
// `p4 -Mj -ztag users`.
type perforceSJONUser struct {
	Email    string `json:"Email,omitempty"`
	User     string `json:"User,omitempty"`
	Password string `json:"Password,omitempty"`
	FullName string `json:"FullName,omitempty"`
	// Access is seconds since the Epoch, but p4 quotes it in the output, so it's a string
	Access string           `json:"Access,omitempty"`
	Update string           `json:"Update,omitempty"`
	Type   perforceUserType `json:"Type,omitempty"`
}
