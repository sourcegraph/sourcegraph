package gitcli

import (
	"context"
	"io"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
)

func (g *gitCLIBackend) ArchiveReader(ctx context.Context, format git.ArchiveFormat, treeish string, paths []string) (io.ReadCloser, error) {
	// Verify the tree-ish exists, if it doesn't this will return a RevisionNotFoundError:
	_, err := g.getObjectType(ctx, treeish)
	if err != nil {
		return nil, err
	}

	archiveArgs := buildArchiveArgs(format, treeish, paths)

	return g.NewCommand(ctx, "archive", WithArguments(archiveArgs...))
}

func buildArchiveArgs(format git.ArchiveFormat, treeish string, paths []string) []Argument {
	args := []Argument{
		FlagArgument{"--worktree-attributes"},
		ValueFlagArgument{Flag: "--format", Value: string(format)},
	}

	if format == git.ArchiveFormatZip {
		args = append(args, FlagArgument{"-0"})
	}

	args = append(args, SpecSafeValueArgument{treeish}, FlagArgument{"--"})
	for _, p := range paths {
		// We use flag argument here because we're past the `--` separator.
		args = append(args, FlagArgument{pathspecLiteral(p)})
	}

	return args
}

// pathspecLiteral constructs a pathspec that matches a path without interpreting "*" or "?" as special
// characters.
//
// See: https://git-scm.com/docs/gitglossary#Documentation/gitglossary.txt-literal
func pathspecLiteral(s string) string { return ":(literal)" + s }
