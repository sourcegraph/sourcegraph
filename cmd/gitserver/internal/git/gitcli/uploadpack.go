package gitcli

import (
	"context"
	"io"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
)

func (g *gitCLIBackend) UploadPack(ctx context.Context, input io.Reader, protocol string, advertiseRefs bool) (io.ReadCloser, error) {
	args := buildUploadPackArgs(advertiseRefs, g.dir)

	opts := []CommandOptionFunc{
		WithStdin(input),
		WithArguments(args...),
	}
	if protocol != "" {
		opts = append(opts, WithEnv("GIT_PROTOCOL="+protocol))
	}

	return g.NewCommand(ctx, "upload-pack", opts...)
}

func buildUploadPackArgs(advertiseRefs bool, dir common.GitDir) []Argument {
	args := []Argument{
		// Allow partial clones/fetches.
		ConfigArgument{Key: "uploadpack.allowFilter", Value: "true"},

		// Allow to fetch any object. Used in case of race between a resolve ref
		// and a fetch of a commit. Safe to do, since this is only used internally.
		ConfigArgument{Key: "uploadpack.allowAnySHA1InWant", Value: "true"},

		// The maximum size of memory that is consumed by each thread in git-pack-objects[1]
		// for pack window memory when no limit is given on the command line.
		//
		// Important for large monorepos to not run into memory issues when cloned.
		ConfigArgument{Key: "pack.windowMemory", Value: "100m"},

		FlagArgument{"--stateless-rpc"}, FlagArgument{"--strict"},
	}
	if advertiseRefs {
		args = append(args, FlagArgument{"--advertise-refs"})
	}

	args = append(args, SpecSafeValueArgument{string(dir)})

	return args
}
