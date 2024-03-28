package perforce

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// P4TestWithTrustArguments are the arguments for P4TestWithTrust.
type P4TestWithTrustArguments struct {
	// P4PORT is the address of the Perforce server.
	P4Port string
	// P4User is the Perforce username to authenticate with.
	P4User string
	// P4Passwd is the Perforce password to authenticate with.
	P4Passwd string
}

// P4TestWithTrust attempts to test the Perforce server and performs a trust operation when needed.
func P4TestWithTrust(ctx context.Context, fs gitserverfs.FS, args P4TestWithTrustArguments) error {
	// Attempt to check connectivity, may be prompted to trust.

	err := P4Test(ctx, fs, P4TestArguments(args))
	if err == nil {
		return nil // The test worked, session still valid for the user
	}

	// If the output indicates that we have to run p4trust first, do that.
	if strings.Contains(err.Error(), "To allow connection use the 'p4 trust' command.") {
		err := P4Trust(ctx, fs, P4TrustArguments{
			P4Port: args.P4Port,
		})
		if err != nil {
			return errors.Wrap(err, "trust")
		}
		// Now attempt to run p4test again.
		err = P4Test(ctx, fs, P4TestArguments(args))
		if err != nil {
			return errors.Wrap(err, "testing connection after trust")
		}
		return nil
	}

	// Something unexpected happened, bubble up the error
	return err
}

// P4UserIsSuperUserArguments are the arguments for P4UserIsSuperUser.
type P4UserIsSuperUserArguments struct {
	// P4Port is the address of the Perforce server.
	P4Port string
	// P4User is the Perforce username to authenticate with.
	P4User string
	// P4Passwd is the Perforce password to authenticate with.
	P4Passwd string
}

// P4UserIsSuperUser checks if the given credentials are for a super level user.
// If the user is a super user, no error is returned. If not, ErrIsNotSuperUser
// is returned.
// Other errors may occur.
func P4UserIsSuperUser(ctx context.Context, fs gitserverfs.FS, args P4UserIsSuperUserArguments) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	options := []P4OptionFunc{
		WithAuthentication(args.P4User, args.P4Passwd),
		WithHost(args.P4Port),
	}

	// Validate the user has "super" access with "-u" option, see https://www.perforce.com/perforce/r12.1/manuals/cmdref/protects.html
	options = append(options, WithArguments("protects", "-u", args.P4User))

	p4home, err := fs.P4HomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to create p4home dir")
	}

	scratchDir, err := fs.TempDir("p4-protects-")
	if err != nil {
		return errors.Wrap(err, "could not create temp dir to invoke 'p4 protects'")
	}
	defer os.Remove(scratchDir)

	cmd := NewBaseCommand(ctx, p4home, scratchDir, options...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctxerr := ctx.Err(); ctxerr != nil {
			err = ctxerr
		}

		if strings.Contains(err.Error(), "You don't have permission for this operation.") {
			return ErrIsNotSuperUser
		}

		if len(out) > 0 {
			err = errors.Errorf("%s (output follows)\n\n%s", err, out)
		}

		return err
	}

	return nil
}

var ErrIsNotSuperUser = errors.New("the user does not have super access")

// P4TrustArguments are the arguments to P4Trust.
type P4TrustArguments struct {
	// P4PORT is the address of the Perforce server.
	P4Port string
}

// P4Trust blindly accepts fingerprint of the Perforce server.
func P4Trust(ctx context.Context, fs gitserverfs.FS, args P4TrustArguments) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	options := []P4OptionFunc{
		WithHost(args.P4Port),
	}

	options = append(options, WithArguments("trust", "-y", "-f"))

	p4home, err := fs.P4HomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to create p4home dir")
	}

	scratchDir, err := fs.TempDir("p4-trust-")
	if err != nil {
		return errors.Wrap(err, "could not create temp dir to invoke 'p4 trust'")
	}
	defer os.Remove(scratchDir)

	cmd := NewBaseCommand(ctx, p4home, scratchDir, options...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctxerr := ctx.Err(); ctxerr != nil {
			err = ctxerr
		}
		if len(out) > 0 {
			err = errors.Errorf("%s (output follows)\n\n%s", err, out)
		}
		return err
	}
	return nil
}

// P4TestArguments are the arguments to the P4Test function.
type P4TestArguments struct {
	// P4PORT is the address of the Perforce server.
	P4Port string
	// P4User is the Perforce username to authenticate with.
	P4User string
	// P4Passwd is the Perforce password to authenticate with.
	P4Passwd string
}

// P4Test uses `p4 login -s` to test the Perforce connection: port, user, passwd.
// If the command times out after 10 seconds, it will be tried one more time.
func P4Test(ctx context.Context, fs gitserverfs.FS, args P4TestArguments) error {
	runCommand := func() error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		options := []P4OptionFunc{
			WithAuthentication(args.P4User, args.P4Passwd),
			WithHost(args.P4Port),
		}

		// `p4 ping` requires extra-special access, so we want to avoid using it
		//
		// p4 login -s checks the connection and the credentials,
		// so it seems like the perfect alternative to `p4 ping`.
		options = append(options, WithArguments("login", "-s"))

		p4home, err := fs.P4HomeDir()
		if err != nil {
			return errors.Wrap(err, "failed to create p4home dir")
		}

		scratchDir, err := fs.TempDir("p4-login-")
		if err != nil {
			return errors.Wrap(err, "could not create temp dir to invoke 'p4 login'")
		}
		defer os.Remove(scratchDir)

		cmd := NewBaseCommand(ctx, p4home, scratchDir, options...)

		out, err := cmd.CombinedOutput()
		if err != nil {
			if ctxerr := ctx.Err(); ctxerr != nil {
				err = errors.Wrap(ctxerr, "p4 login context error")
			}
			if len(out) > 0 {
				err = errors.Errorf("%s (output follows)\n\n%s", err, specifyCommandInErrorMessage(string(out), cmd.Unwrap()))
			}
			return err
		}
		return nil
	}
	err := runCommand()
	if err != nil && errors.Is(err, context.DeadlineExceeded) {
		err = runCommand()
	}
	return err
}
