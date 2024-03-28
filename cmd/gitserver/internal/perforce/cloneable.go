package perforce

import (
	"context"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// IsDepotPathCloneableArguments are the arguments for IsDepotPathCloneable.
type IsDepotPathCloneableArguments struct {
	// P4PORT is the address of the Perforce server.
	P4Port string
	// P4User is the Perforce username to authenticate with.
	P4User string
	// P4Passwd is the Perforce password to authenticate with.
	P4Passwd string

	// DepotPath is the path to the depot to check.
	DepotPath string
}

func IsDepotPathCloneable(ctx context.Context, fs gitserverfs.FS, args IsDepotPathCloneableArguments) error {
	// start with a test and set up trust if necessary
	if err := P4TestWithTrust(ctx, fs, P4TestWithTrustArguments{
		P4Port:   args.P4Port,
		P4User:   args.P4User,
		P4Passwd: args.P4Passwd,
	}); err != nil {
		return errors.Wrap(err, "checking perforce credentials")
	}

	// the path could be a path into a depot, or it could be just a depot
	// expect it to start with at least one slash
	// (the config defines it as starting with two, but converting it to a URL may change that)
	// the first path part will be the depot - subsequent parts define a directory path into a depot
	// ignore the directory parts for now, and only test for access to the depot
	// TODO: revisit if we want to also test for access to the directories, if any are included
	depot := strings.Split(strings.TrimLeft(args.DepotPath, "/"), "/")[0]

	// get a list of depots that match the supplied depot (if it's defined)
	depots, err := P4Depots(ctx, fs, P4DepotsArguments{
		P4Port: args.P4Port,

		P4User:   args.P4User,
		P4Passwd: args.P4Passwd,

		NameFilter: depot,
	})
	if err != nil {
		return err
	}
	if len(depots) == 0 {
		// this user doesn't have access to any depots,
		// or to the given depot
		if depot != "" {
			return errors.Newf("the user %s does not have access to the depot %s on the server %s", args.P4User, depot, args.P4Port)
		} else {
			return errors.Newf("the user %s does not have access to any depots on the server %s", args.P4User, args.P4Port)
		}
	}

	// no overt errors, so this depot is cloneable
	return nil
}
