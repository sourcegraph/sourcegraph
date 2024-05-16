package gitcli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
)

func (g *gitCLIBackend) Blame(ctx context.Context, startCommit api.CommitID, path string, opt git.BlameOptions) (git.BlameHunkReader, error) {
	// Verify that the blob exists.
	_, err := g.getBlobOID(ctx, startCommit, path)
	if err != nil {
		return nil, err
	}

	r, err := g.NewCommand(ctx, "blame", WithArguments(buildBlameArgs(startCommit, path, opt)...))
	if err != nil {
		return nil, err
	}

	return newBlameHunkReader(r), nil
}

func buildBlameArgs(startCommit api.CommitID, path string, opt git.BlameOptions) []Argument {
	args := []Argument{
		FlagArgument{"--porcelain"},
		FlagArgument{"--incremental"},
	}
	if opt.IgnoreWhitespace {
		args = append(args, FlagArgument{"-w"})
	}
	if opt.Range != nil {
		// We treat this as safe because the rendering of the flag is owned by us and we
		// only print digits from user input.
		args = append(args, FlagArgument{fmt.Sprintf("-L%d,%d", opt.Range.StartLine, opt.Range.EndLine)})
	}
	args = append(args, SpecSafeValueArgument{string(startCommit)}, FlagArgument{"--"}, SpecSafeValueArgument{filepath.ToSlash(path)})
	return args
}

// blameHunkReader enables to read hunks from an io.Reader.
type blameHunkReader struct {
	rc io.ReadCloser
	sc *bufio.Scanner

	cur *gitdomain.Hunk
}

func newBlameHunkReader(rc io.ReadCloser) git.BlameHunkReader {
	return &blameHunkReader{
		rc:  rc,
		sc:  bufio.NewScanner(rc),
		cur: &gitdomain.Hunk{},
	}
}

// copyHunk creates a copy of the hunk, including any fields that are
// references.
func copyHunk(h *gitdomain.Hunk) *gitdomain.Hunk {
	dup := *h
	if h.PreviousCommit != nil {
		previousCommit := *h.PreviousCommit
		dup.PreviousCommit = &previousCommit
	}
	return &dup
}

// Read returns the next blame hunk.
func (br *blameHunkReader) Read() (*gitdomain.Hunk, error) {
	// Blame hunks follow a structured output, starting with the
	// hunk header: <commit hash> <original file start line> <current file start line> <number of lines>
	// followed by the hunk body.
	// The hunk body is a list of lines in the format: <field name> <field value>
	// A hunk body terminates with the filename field.
	// If a hunk is part of the same commit as a previous hunk, only
	// the difference in fields is returned, so the previous commit
	// should be cached until a new commit is encountered.
	for br.sc.Scan() {
		// First parse the hunk header.
		err := parseHeader(br.cur, br.sc.Bytes())
		if err != nil {
			return nil, err
		}

		// After that, we're reading the hunk body.
		for br.sc.Scan() {
			annotation, fields, _ := bytes.Cut(br.sc.Bytes(), []byte(" "))
			done, err := parseBody(br.cur, annotation, fields)
			if err != nil {
				return nil, err
			}

			// If we've finished reading the body we can return the hunk.
			if done {
				// Copy the hunk before returning
				return copyHunk(br.cur), nil
			}
		}
	}

	// Return the scanner error if there was one
	if err := br.sc.Err(); err != nil {
		return nil, err
	}

	// Otherwise, return the sentinel io.EOF
	return nil, io.EOF
}

func (br *blameHunkReader) Close() error {
	return br.rc.Close()
}

// parseHeader reads a `67b7b725a7ff913da520b997d71c840230351e30 10 20 1` line from
// git blame and updates the provided hunk. If the commit ID is
// different from the current one, the hunk is reset.
func parseHeader(hunk *gitdomain.Hunk, line []byte) error {
	fields := bytes.SplitN(line, []byte(" "), 4)

	resultLine, err := strconv.Atoi(string(fields[2]))
	if err != nil {
		return err
	}
	numLines, err := strconv.Atoi(string(fields[3]))
	if err != nil {
		return err
	}

	if string(hunk.CommitID) != string(fields[0]) {
		// Start of a new commit, reset all the fields.
		*hunk = gitdomain.Hunk{CommitID: api.CommitID(fields[0])}
	}
	hunk.StartLine = uint32(resultLine)
	hunk.EndLine = uint32(resultLine + numLines)

	return nil
}

func unquotedStringFromBytes(s []byte) string {
	str := string(s)
	unquotedString, err := strconv.Unquote(str)
	if err != nil {
		return str
	}
	return unquotedString
}

// parseBody updates a hunk with data parsed from the other annotations such as `author ...`,
// `summary ...`.
func parseBody(hunk *gitdomain.Hunk, annotation []byte, content []byte) (done bool, err error) {
	switch string(annotation) {
	case "author":
		hunk.Author.Name = string(content)
	case "author-mail":
		hunk.Author.Email = string(bytes.Trim(content, "<>"))
	case "author-time":
		var t int64
		t, err = strconv.ParseInt(string(content), 10, 64)
		if err != nil {
			return false, err
		}
		hunk.Author.Date = time.Unix(t, 0).UTC()
	case "summary":
		hunk.Message = string(content)
	case "previous":
		commitID, filename, _ := bytes.Cut(content, []byte(" "))
		hunk.PreviousCommit = &gitdomain.PreviousCommit{
			CommitID: api.CommitID(commitID),
			Filename: unquotedStringFromBytes(filename),
		}
	case "filename":
		hunk.Filename = unquotedStringFromBytes(content)
		// filename designates the end of a hunk body
		return true, nil
	}
	return false, nil
}
