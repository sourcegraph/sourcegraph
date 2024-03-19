package search

import (
	"bytes"
	"context"
	"io"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/protocol"
	"github.com/sourcegraph/sourcegraph/internal/search/casetransform"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func regexSearchBatch(
	ctx context.Context,
	p *protocol.PatternInfo,
	zf *zipFile,
	contextLines int32,
) ([]protocol.FileMatch, error) {
	m, err := toMatchTree(p.Query, p.IsCaseSensitive)
	if err != nil {
		return nil, err
	}

	lm := toLangMatcher(p)
	pm, err := toPathMatcher(p)
	if err != nil {
		return nil, err
	}

	ctx, cancel, sender := newLimitedStreamCollector(ctx, p.Limit)
	defer cancel()
	err = regexSearch(ctx, m, pm, lm, zf, p.PatternMatchesContent, p.PatternMatchesPath, p.IsCaseSensitive, sender, contextLines)
	return sender.collected, err
}

// regexSearch concurrently searches files in zr looking for matches using m.
//
// This code is base on reading the techniques detailed in
// http://blog.burntsushi.net/ripgrep/
//
// The stdlib regexp is pretty powerful and in fact implements many of the
// features in ripgrep. Our implementation gives high performance via pruning
// aggressively which files to consider (non-binary under a limit) and
// optimizing for assuming most lines will not contain a match.
//
// If there is no more low-hanging fruit and perf is not acceptable, we could
// consider using ripgrep directly (modify it to search zip archives).
//
// TODO(keegan) return search statistics
func regexSearch(
	ctx context.Context,
	m matchTree,
	pm *pathMatcher,
	lm langMatcher,
	zf *zipFile,
	patternMatchesContent, patternMatchesPaths bool,
	isCaseSensitive bool,
	sender matchSender,
	contextLines int32,
) (err error) {
	tr, ctx := trace.New(ctx, "regexSearch")
	defer tr.EndWithErr(&err)

	tr.SetAttributes(attribute.Stringer("path", pm))
	tr.SetAttributes(attribute.String("matchTree", m.String()))

	if !patternMatchesContent && !patternMatchesPaths {
		patternMatchesContent = true
	}

	// If we reach limit we use cancel to stop the search
	var cancel context.CancelFunc
	if deadline, ok := ctx.Deadline(); ok {
		// If a deadline is set, try to finish before the deadline expires.
		timeout := time.Duration(0.9 * float64(time.Until(deadline)))
		tr.AddEvent("set timeout", attribute.Stringer("duration", timeout))
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	files := zf.Files

	var (
		lastFileIdx   = atomic.NewInt32(-1)
		filesSkipped  atomic.Uint32
		filesSearched atomic.Uint32
	)

	g, ctx := errgroup.WithContext(ctx)

	contextCanceled := atomic.NewBool(false)
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		contextCanceled.Store(true)
		close(done)
	}()
	defer func() { cancel(); <-done }()

	// Start workers. They read from files and write to matches.
	for range numWorkers {
		l := fileLoader{zf: zf, isCaseSensitive: isCaseSensitive}
		var f *srcFile

		// A callback to use when detecting the language. The function signature includes an
		// error so to match the expectations of the library we use, helping avoid allocations.
		getContent := func() ([]byte, error) { //nolint:unparam
			l.load(f)
			return l.fileBuf, nil
		}

		g.Go(func() error {
			for !contextCanceled.Load() {
				idx := int(lastFileIdx.Inc())
				if idx >= len(files) {
					return nil
				}

				f = &files[idx]

				// Apply path filters
				if !pm.Matches(f.Name) {
					filesSkipped.Inc()
					continue
				}
				filesSearched.Inc()

				// Check pattern against file path and contents
				match := false
				fm := protocol.FileMatch{
					Path:     f.Name,
					LimitHit: false,
				}

				if patternMatchesPaths {
					match = m.MatchesString(f.Name)
				}

				if !match && patternMatchesContent {
					if _, ok := m.(*allMatchTree); ok {
						// Avoid loading the file if this pattern always matches
						match = true
					} else {
						l.load(f)

						// find limit+1 matches so we know whether we hit the limit
						var locs [][]int
						match, locs = m.MatchesFile(l.fileMatchBuf, sender.Remaining()+1)
						fm = locsToFileMatch(l.fileBuf, f.Name, locs, contextLines)
					}
				}

				if match {
					// Apply language filters and send result
					langMatch, lang := lm.Matches(f.Name, getContent)
					if langMatch {
						fm.Language = lang
						sender.Send(fm)
					}
				}
			}
			return nil
		})
	}

	err = g.Wait()
	if err == nil && ctx.Err() == context.DeadlineExceeded {
		// We stopped early because we were about to hit the deadline.
		err = ctx.Err()
	}

	tr.AddEvent(
		"done",
		attribute.Int("filesSkipped", int(filesSkipped.Load())),
		attribute.Int("filesSearched", int(filesSearched.Load())),
	)

	return err
}

// fileLoader loads files from the zipfile. It keeps a reference to the last
// file it loaded, in case it's requested twice (for example first from language
// detection, then pattern matching).
type fileLoader struct {
	zf              *zipFile
	isCaseSensitive bool

	currFile *srcFile

	// fileBuf is the original data (used for the content preview)
	fileBuf []byte
	// fileMatchBuf is what we match against, and may be a lower-cased version of fileBuf
	fileMatchBuf []byte

	// scratchBuf is reused between file searches to avoid
	// re-allocating. It is only used if we need to transform the input
	// before matching. For example we lower case the input in the case of
	// ignoreCase.
	scratchBuf []byte
}

func (l *fileLoader) load(f *srcFile) {
	if f == l.currFile {
		return
	}
	l.currFile = f

	l.fileBuf = l.zf.DataFor(f)
	l.fileMatchBuf = l.fileBuf
	if !l.isCaseSensitive {
		// If we are ignoring case, we transform the input instead of
		// relying on the regular expression engine which can be
		// slow. compilePattern has already lowercased the pattern. We also
		// trade some correctness for perf by using a non-utf8 aware
		// lowercase function.
		if l.scratchBuf == nil {
			l.scratchBuf = make([]byte, l.zf.MaxLen)
		}
		l.fileMatchBuf = l.scratchBuf[:len(l.fileBuf)]
		casetransform.BytesToLowerASCII(l.fileMatchBuf, l.fileBuf)
	}
}

// readAll will read r until EOF into b. It returns the number of bytes
// read. If we do not reach EOF, an error is returned.
func readAll(r io.Reader, b []byte) (int, error) {
	n := 0
	for {
		if len(b) == 0 {
			// We may be at EOF, but it hasn't returned that
			// yet. Technically r.Read is allowed to return 0,
			// nil, but it is strongly discouraged. If they do, we
			// will just return an err.
			scratch := []byte{'1'}
			_, err := r.Read(scratch)
			if err == io.EOF {
				return n, nil
			}
			return n, errors.New("reader is too large")
		}

		m, err := r.Read(b)
		n += m
		b = b[m:]
		if err != nil {
			if err == io.EOF { // done
				return n, nil
			}
			return n, err
		}
	}
}

func locsToFileMatch(fileBuf []byte, name string, locs [][]int, contextLines int32) protocol.FileMatch {
	if len(locs) == 0 {
		return protocol.FileMatch{
			Path:     name,
			LimitHit: false,
		}
	}
	ranges := locsToRanges(fileBuf, locs)
	chunks := chunkRanges(ranges, contextLines*2)
	cms := chunksToMatches(fileBuf, chunks, contextLines)
	return protocol.FileMatch{
		Path:         name,
		ChunkMatches: cms,
		LimitHit:     false,
	}
}

// locs must be sorted, non-overlapping, and must be valid slices of buf.
func locsToRanges(buf []byte, locs [][]int) []protocol.Range {
	ranges := make([]protocol.Range, 0, len(locs))

	prevStart := 0
	prevStartLine := 0

	c := columnHelper{
		data: buf,
	}

	for _, loc := range locs {
		start, end := loc[0], loc[1]

		startLine := prevStartLine + bytes.Count(buf[prevStart:start], []byte{'\n'})
		endLine := startLine + bytes.Count(buf[start:end], []byte{'\n'})

		firstLineStart := 0
		if off := bytes.LastIndexByte(buf[:start], '\n'); off >= 0 {
			firstLineStart = off + 1
		}

		lastLineStart := firstLineStart
		if off := bytes.LastIndexByte(buf[:end], '\n'); off >= 0 {
			lastLineStart = off + 1
		}

		ranges = append(ranges, protocol.Range{
			Start: protocol.Location{
				Offset: int32(start),
				Line:   int32(startLine),
				Column: int32(c.get(firstLineStart, start)),
			},
			End: protocol.Location{
				Offset: int32(end),
				Line:   int32(endLine),
				Column: int32(c.get(lastLineStart, end)),
			},
		})

		prevStart = start
		prevStartLine = startLine
	}

	return ranges
}
