package gitcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) Blame(ctx context.Context, startCommit api.CommitID, path string, opt git.BlameOptions) (git.BlameHunkReader, error) {
	if err := checkSpecArgSafety(string(startCommit)); err != nil {
		return nil, err
	}

	// Verify that the blob exists.
	_, err := g.getBlobOID(ctx, startCommit, path)
	if err != nil {
		return nil, err
	}

	r, err := g.NewCommand(ctx, WithArguments(buildBlameArgs(startCommit, path, opt)...))
	if err != nil {
		return nil, err
	}

	return newBlameHunkReader(r), nil
}

func buildBlameArgs(startCommit api.CommitID, path string, opt git.BlameOptions) []string {
	args := []string{"blame", "--porcelain", "--incremental"}
	if opt.IgnoreWhitespace {
		args = append(args, "-w")
	}
	if opt.Range != nil {
		args = append(args, fmt.Sprintf("-L%d,%d", opt.Range.StartLine, opt.Range.EndLine))
	}
	args = append(args, string(startCommit), "--", filepath.ToSlash(path))
	return args
}

// blameHunkReader enables to read hunks from an io.Reader.
type blameHunkReader struct {
	rc io.ReadCloser
	sc *bufio.Scanner

	cur *gitdomain.Hunk

	// commits stores previously seen commits, so new hunks
	// whose annotations are abbreviated by git can still be
	// filled by the correct data even if the hunk entry doesn't
	// repeat them.
	commits map[api.CommitID]*gitdomain.Hunk
}

func newBlameHunkReader(rc io.ReadCloser) git.BlameHunkReader {
	return &blameHunkReader{
		rc:      rc,
		sc:      bufio.NewScanner(rc),
		commits: make(map[api.CommitID]*gitdomain.Hunk),
	}
}

// Read returns a slice of hunks, along with a done boolean indicating if there
// is more to read. After the last hunk has been returned, Read() will return
// an io.EOF error on success.
func (br *blameHunkReader) Read() (_ *gitdomain.Hunk, err error) {
	for {
		// Do we have more to read?
		if !br.sc.Scan() {
			if br.cur != nil {
				if h, ok := br.commits[br.cur.CommitID]; ok {
					br.cur.CommitID = h.CommitID
					br.cur.Author = h.Author
					br.cur.Message = h.Message
				}
				// If we have an ongoing entry, return it
				res := br.cur
				br.cur = nil
				return res, nil
			}
			// Return the scanner error if ther was one
			if err := br.sc.Err(); err != nil {
				return nil, err
			}
			// Otherwise, return the sentinel io.EOF
			return nil, io.EOF
		}

		// Read line from git blame, in porcelain format
		line := br.sc.Text()
		annotation, fields := splitLine(line)

		// On the first read, we have no hunk and the first thing we read is an entry.
		if br.cur == nil {
			br.cur, err = parseEntry(annotation, fields)
			if err != nil {
				return nil, err
			}
			continue
		}

		// After that, we're either reading extras, or a new entry.
		ok, err := parseExtra(br.cur, annotation, fields)
		if err != nil {
			return nil, err
		}

		// If we've finished reading extras, we're looking at a new entry.
		if !ok {
			if h, ok := br.commits[br.cur.CommitID]; ok {
				br.cur.CommitID = h.CommitID
				br.cur.Author = h.Author
				br.cur.Message = h.Message
			} else {
				br.commits[br.cur.CommitID] = br.cur
			}

			res := br.cur

			br.cur, err = parseEntry(annotation, fields)
			if err != nil {
				return nil, err
			}

			return res, nil
		}
	}
}

func (br *blameHunkReader) Close() error {
	return br.rc.Close()
}

// parseEntry turns a `67b7b725a7ff913da520b997d71c840230351e30 10 20 1` line from
// git blame into a hunk.
func parseEntry(rev string, content string) (*gitdomain.Hunk, error) {
	fields := strings.Split(content, " ")
	if len(fields) != 3 {
		return nil, errors.Errorf("Expected at least 4 parts to hunkHeader, but got: '%s %s'", rev, content)
	}

	resultLine, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, err
	}
	numLines, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, err
	}

	return &gitdomain.Hunk{
		CommitID:  api.CommitID(rev),
		StartLine: uint32(resultLine),
		EndLine:   uint32(resultLine + numLines),
	}, nil
}

// parseExtra updates a hunk with data parsed from the other annotations such as `author ...`,
// `summary ...`.
func parseExtra(hunk *gitdomain.Hunk, annotation string, content string) (ok bool, err error) {
	ok = true
	switch annotation {
	case "author":
		hunk.Author.Name = content
	case "author-mail":
		if len(content) >= 2 && content[0] == '<' && content[len(content)-1] == '>' {
			hunk.Author.Email = content[1 : len(content)-1]
		}
	case "author-time":
		var t int64
		t, err = strconv.ParseInt(content, 10, 64)
		hunk.Author.Date = time.Unix(t, 0).UTC()
	case "author-tz":
		// do nothing
	case "committer", "committer-mail", "committer-tz", "committer-time":
	case "summary":
		hunk.Message = content
	case "filename":
		hunk.Filename = content
	case "previous":
	case "boundary":
	default:
		// If it doesn't look like an entry, it's probably an unhandled git blame
		// annotation.
		if len(annotation) != 40 && len(strings.Split(content, " ")) != 3 {
			err = errors.Newf("unhandled git blame annotation: %s")
		}
		ok = false
	}
	return
}

// splitLine splits a scanned line and returns the annotation along
// with the content, if any.
func splitLine(line string) (annotation string, content string) {
	annotation, content, found := strings.Cut(line, " ")
	if found {
		return annotation, content
	}
	return line, ""
}
