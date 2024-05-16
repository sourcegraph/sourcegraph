package gitcli

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) LatestCommitTimestamp(ctx context.Context) (time.Time, error) {
	r, err := g.NewCommand(ctx, "rev-list", WithArguments(
		FlagArgument{"--all"},
		FlagArgument{"--timestamp"},
		FlagArgument{"-n"},
		FlagArgument{"1"},
	))
	if err != nil {
		return time.Time{}, err
	}

	stdout, err := io.ReadAll(r)
	err = errors.Append(err, r.Close())
	if err != nil {
		return time.Time{}, err
	}

	now := time.Now().UTC()

	words := bytes.Split(bytes.TrimSpace(stdout), []byte(" "))
	// An empty rev-list output, without an error, is okay. This is probably just
	// an empty repository.
	if len(words) < 2 {
		return now, nil
	}

	// We should have a timestamp and a commit hash; format is
	// 1521316105 ff03fac223b7f16627b301e03bf604e7808989be
	epoch, err := strconv.ParseInt(string(words[0]), 10, 64)
	if err != nil {
		g.logger.Warn("failed to parse LatestCommitTimestamp", log.String("output", string(stdout)))
		// If the timestamp can't be parsed, we just return time.Now().
		return now, nil
	}

	stamp := time.Unix(epoch, 0).UTC()
	if stamp.After(now) {
		return now, nil
	}
	return stamp, nil
}
