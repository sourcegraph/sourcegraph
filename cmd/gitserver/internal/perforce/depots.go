package perforce

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type perforceDepotType string

func (t perforceDepotType) Valid() bool {
	switch t {
	case perforceDepotTypeLocal,
		perforceDepotTypeRemote,
		perforceDepotTypeStream,
		perforceDepotTypeSpec,
		perforceDepotTypeUnload,
		perforceDepotTypeArchive,
		perforceDepotTypeTangent,
		perforceDepotTypeGraph:
		return true
	default:
		return false
	}
}

const (
	perforceDepotTypeLocal   perforceDepotType = "local"
	perforceDepotTypeRemote  perforceDepotType = "remote"
	perforceDepotTypeStream  perforceDepotType = "stream"
	perforceDepotTypeSpec    perforceDepotType = "spec"
	perforceDepotTypeUnload  perforceDepotType = "unload"
	perforceDepotTypeArchive perforceDepotType = "archive"
	perforceDepotTypeTangent perforceDepotType = "tangent"
	perforceDepotTypeGraph   perforceDepotType = "graph"
)

// perforceDepot is a definiton of a depot that matches the format
// returned from `p4 -Mj -ztag depots`
type perforceDepot struct {
	Desc string `json:"desc,omitempty"`
	Map  string `json:"map,omitempty"`
	Name string `json:"name,omitempty"`
	// Time is seconds since the Epoch, but p4 quotes it in the output, so it's a string
	Time string `json:"time,omitempty"`
	// Type is local, remote, stream, spec, unload, archive, tangent, graph
	Type perforceDepotType `json:"type,omitempty"`
}

// P4DepotsArguments contains the arguments for P4Depots.
type P4DepotsArguments struct {
	// P4Port is the address of the Perforce server.
	P4Port string
	// P4User is the Perforce username to authenticate with.
	P4User string
	// P4Passwd is the Perforce password to authenticate with.
	P4Passwd string

	// NameFilter is a filter for the depot names to return.
	NameFilter string
}

// P4Depots returns all of the depots to which the user has access on the host
// and whose names match the given nameFilter, which can contain asterisks (*) for wildcards
// if nameFilter is blank, return all depots.
func P4Depots(ctx context.Context, fs gitserverfs.FS, args P4DepotsArguments) ([]perforceDepot, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	options := []P4OptionFunc{
		WithAuthentication(args.P4User, args.P4Passwd),
		WithHost(args.P4Port),
	}

	if args.NameFilter == "" {
		options = append(options, WithArguments("-Mj", "-ztag", "depots"))
	} else {
		options = append(options, WithArguments("-Mj", "-ztag", "depots", "-e", args.NameFilter))
	}

	p4home, err := fs.P4HomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create p4home dir")
	}

	scratchDir, err := fs.TempDir("p4-depots-")
	if err != nil {
		return nil, errors.Wrap(err, "could not create temp dir to invoke 'p4 depots'")
	}
	defer os.Remove(scratchDir)

	cmd := NewBaseCommand(ctx, p4home, scratchDir, options...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctxerr := ctx.Err(); ctxerr != nil {
			err = errors.Wrap(ctxerr, "p4 depots context error")
		}
		if len(out) > 0 {
			err = errors.Wrapf(err, `failed to run command "p4 depots" (output follows)\n\n%s`, specifyCommandInErrorMessage(string(out), cmd.Unwrap()))
		}
		return nil, err
	}
	depots := make([]perforceDepot, 0)
	if len(out) > 0 {
		// the output of `p4 -Mj -ztag depots` is a series of JSON-formatted depot definitions, one per line
		buf := bufio.NewScanner(bytes.NewBuffer(out))
		for buf.Scan() {
			depot := perforceDepot{}
			err := json.Unmarshal(buf.Bytes(), &depot)
			if err != nil {
				return nil, errors.Wrap(err, "malformed output from p4 depots")
			}
			depots = append(depots, depot)
		}
		if err := buf.Err(); err != nil {
			return nil, errors.Wrap(err, "malformed output from p4 depots")
		}
		return depots, nil
	}

	// no error, but also no depots. Maybe the user doesn't have access to any depots?
	return depots, nil
}
