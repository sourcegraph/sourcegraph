package search

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/protocol"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func BenchmarkSearchRegex_large_fixed(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/golang/go",
		Commit: "0ebaca6ba27534add5930a95acffa9acff182e2b",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value: "error handler",
			},
		},
	})
}

func BenchmarkSearchRegex_rare_fixed(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/golang/go",
		Commit: "0ebaca6ba27534add5930a95acffa9acff182e2b",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value: "REBOOT_CMD",
			},
		},
	})
}

func BenchmarkSearchRegex_large_fixed_casesensitive(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/golang/go",
		Commit: "0ebaca6ba27534add5930a95acffa9acff182e2b",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value: "error handler",
			},
			IsCaseSensitive: true,
		},
	})
}

func BenchmarkSearchRegex_large_empty_pattern(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/golang/go",
		Commit: "0ebaca6ba27534add5930a95acffa9acff182e2b",
		PatternInfo: protocol.PatternInfo{
			IncludePaths: []string{".*"},
			Query: &protocol.PatternNode{
				Value: "",
			},
		},
	})
}

func BenchmarkSearchRegex_large_lang_filter_common(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/golang/go",
		Commit: "0ebaca6ba27534add5930a95acffa9acff182e2b",
		PatternInfo: protocol.PatternInfo{
			IncludeLangs: []string{"Go"},
			Query: &protocol.PatternNode{
				Value: "error handler",
			},
		},
	})
}

func BenchmarkSearchRegex_large_lang_filter_uncommon(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/golang/go",
		Commit: "0ebaca6ba27534add5930a95acffa9acff182e2b",
		PatternInfo: protocol.PatternInfo{
			IncludeLangs: []string{"C"},
			Query: &protocol.PatternNode{
				Value: "errorr",
			},
		},
	})
}

func BenchmarkSearchRegex_large_re_dotstar(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/golang/go",
		Commit: "0ebaca6ba27534add5930a95acffa9acff182e2b",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value:    ".*",
				IsRegExp: true,
			},
		},
	})
}

func BenchmarkSearchRegex_large_re_common(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/golang/go",
		Commit: "0ebaca6ba27534add5930a95acffa9acff182e2b",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value:    "func +[A-Z]",
				IsRegExp: true,
			},
			IsCaseSensitive: true,
		},
	})
}

func BenchmarkSearchRegex_large_re_anchor(b *testing.B) {
	// TODO(keegan) PERF regex engine performs poorly since LiteralPrefix
	// is empty when ^. We can improve this by:
	// * Transforming the regex we use to prune a file to be more
	// performant/permissive.
	// * Searching for any literal (Rabin-Karp aka bytes.Index) or group
	// of literals (Aho-Corasick).
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/golang/go",
		Commit: "0ebaca6ba27534add5930a95acffa9acff182e2b",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value:    "^func +[A-Z]",
				IsRegExp: true,
			},
			IsCaseSensitive: true,
		},
	})
}

func BenchmarkSearchRegex_large_capture_group(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/golang/go",
		Commit: "0ebaca6ba27534add5930a95acffa9acff182e2b",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value:    "(TODO|FIXME)",
				IsRegExp: true,
			},
			IsCaseSensitive: true,
		},
	})
}

func BenchmarkSearchRegex_large_path(b *testing.B) {
	do := func(b *testing.B, content, path bool) {
		benchSearchRegex(b, &protocol.Request{
			Repo:   "github.com/golang/go",
			Commit: "0ebaca6ba27534add5930a95acffa9acff182e2b",
			PatternInfo: protocol.PatternInfo{
				Query: &protocol.PatternNode{
					Value:    "http.*client",
					IsRegExp: true,
				},
				IsCaseSensitive:       true,
				PatternMatchesContent: content,
				PatternMatchesPath:    path,
			},
		})
	}
	b.Run("path only", func(b *testing.B) { do(b, false, true) })
	b.Run("content only", func(b *testing.B) { do(b, true, false) })
	b.Run("both path and content", func(b *testing.B) { do(b, true, true) })
}

func BenchmarkSearchRegex_small_fixed(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/sourcegraph/go-langserver",
		Commit: "4193810334683f87b8ed5d896aa4753f0dfcdf20",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value: "object not found",
			},
		},
	})
}

func BenchmarkSearchRegex_small_fixed_casesensitive(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/sourcegraph/go-langserver",
		Commit: "4193810334683f87b8ed5d896aa4753f0dfcdf20",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value: "object not found",
			},
			IsCaseSensitive: true,
		},
	})
}

func BenchmarkSearchRegex_small_re_dotstar(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/sourcegraph/go-langserver",
		Commit: "4193810334683f87b8ed5d896aa4753f0dfcdf20",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value:    ".*",
				IsRegExp: true,
			},
		},
	})
}

func BenchmarkSearchRegex_small_re_common(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/sourcegraph/go-langserver",
		Commit: "4193810334683f87b8ed5d896aa4753f0dfcdf20",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value:    "func +[A-Z]",
				IsRegExp: true,
			},
			IsCaseSensitive: true,
		},
	})
}

func BenchmarkSearchRegex_small_re_anchor(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/sourcegraph/go-langserver",
		Commit: "4193810334683f87b8ed5d896aa4753f0dfcdf20",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value:    "^func +[A-Z]",
				IsRegExp: true,
			},
			IsCaseSensitive: true,
		},
	})
}

func BenchmarkSearchRegex_small_capture_group(b *testing.B) {
	benchSearchRegex(b, &protocol.Request{
		Repo:   "github.com/sourcegraph/go-langserver",
		Commit: "4193810334683f87b8ed5d896aa4753f0dfcdf20",
		PatternInfo: protocol.PatternInfo{
			Query: &protocol.PatternNode{
				Value:    "(TODO|FIXME)",
				IsRegExp: true,
			},
			IsCaseSensitive: true,
		},
	})
}

func benchSearchRegex(b *testing.B, p *protocol.Request) {
	if testing.Short() {
		b.Skip("")
	}
	b.ReportAllocs()

	p.Limit = 99999999
	err := validateParams(p)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	path, err := githubStore.PrepareZip(ctx, p.Repo, p.Commit, nil)
	if err != nil {
		b.Fatal(err)
	}

	var zc zipCache
	zf, err := zc.Get(path)
	if err != nil {
		b.Fatal(err)
	}
	defer zf.Close()

	b.ResetTimer()

	for range b.N {
		_, err := regexSearchBatch(ctx, &p.PatternInfo, zf, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestReadAll(t *testing.T) {
	input := []byte("Hello World")

	// If we are the same size as input, it should work
	b := make([]byte, len(input))
	n, err := readAll(bytes.NewReader(input), b)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(input) {
		t.Fatalf("want to read in %d bytes, read %d", len(input), n)
	}
	if string(b[:n]) != string(input) {
		t.Fatalf("got %s, want %s", string(b[:n]), string(input))
	}

	// If we are larger then it should work
	b = make([]byte, len(input)*2)
	n, err = readAll(bytes.NewReader(input), b)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(input) {
		t.Fatalf("want to read in %d bytes, read %d", len(input), n)
	}
	if string(b[:n]) != string(input) {
		t.Fatalf("got %s, want %s", string(b[:n]), string(input))
	}

	// Same size, but modify reader to return 1 byte per call to ensure
	// our loop works.
	b = make([]byte, len(input))
	n, err = readAll(iotest.OneByteReader(bytes.NewReader(input)), b)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(input) {
		t.Fatalf("want to read in %d bytes, read %d", len(input), n)
	}
	if string(b[:n]) != string(input) {
		t.Fatalf("got %s, want %s", string(b[:n]), string(input))
	}

	// If we are too small it should fail
	b = make([]byte, 1)
	_, err = readAll(bytes.NewReader(input), b)
	if err == nil {
		t.Fatal("expected to fail on small buffer")
	}
}

func TestMaxMatches(t *testing.T) {
	t.Skip("TODO: Disabled because it's flaky. See: https://github.com/sourcegraph/sourcegraph/issues/22560")

	pattern := "foo"

	// Create a zip archive which contains our limits + 1
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	maxMatches := 33
	for i := range maxMatches + 1 {
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:   strconv.Itoa(i),
			Method: zip.Store,
		})
		if err != nil {
			t.Fatal(err)
		}
		for range 10 {
			_, _ = w.Write([]byte(pattern))
			_, _ = w.Write([]byte{' '})
			_, _ = w.Write([]byte{'\n'})
		}
	}
	err := zw.Close()
	if err != nil {
		t.Fatal(err)
	}
	zf, err := mockZipFile(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	p := &protocol.PatternInfo{Query: &protocol.PatternNode{Value: pattern}}
	m, err := toMatchTree(p.Query, p.IsCaseSensitive)
	if err != nil {
		t.Fatal(err)
	}

	lm := toLangMatcher(p)
	pm, err := toPathMatcher(p)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel, sender := newLimitedStreamCollector(context.Background(), maxMatches)
	defer cancel()
	err = regexSearch(ctx, m, pm, lm, zf, true, false, false, sender, 0)
	fileMatches := sender.collected
	limitHit := sender.LimitHit()

	if err != nil {
		t.Fatal(err)
	}
	if !limitHit {
		t.Fatalf("expected limitHit on regexSearch")
	}

	totalMatches := 0
	for _, match := range fileMatches {
		totalMatches += match.MatchCount()
	}

	if totalMatches != maxMatches {
		t.Fatalf("expected %d file matches, got %d", maxMatches, totalMatches)
	}
}

// Tests that:
//
// - IncludePaths can match the path in any order
// - A path must match all (not any) of the IncludePaths
// - An empty pattern is allowed
func TestPathMatches(t *testing.T) {
	zipData, err := createZip(map[string]string{
		"a":   "",
		"a/b": "",
		"a/c": "",
		"ab":  "",
		"b/a": "",
		"ba":  "",
		"c/d": "",
	})
	if err != nil {
		t.Fatal(err)
	}
	zf, err := mockZipFile(zipData)
	if err != nil {
		t.Fatal(err)
	}

	patternInfo := &protocol.PatternInfo{
		Query: &protocol.PatternNode{
			Value: "",
		},
		IncludePaths:          []string{"a", "b"},
		PatternMatchesContent: true,
		PatternMatchesPath:    true,
		IsCaseSensitive:       false,
		Limit:                 10,
	}

	fileMatches, err := regexSearchBatch(context.Background(), patternInfo, zf, 0)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"a/b", "ab", "b/a", "ba"}
	got := make([]string, len(fileMatches))
	for i, fm := range fileMatches {
		got[i] = fm.Path
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got file matches %v, want %v", got, want)
	}
}

// githubStore fetches from github and caches across test runs.
var githubStore = &Store{
	GitserverClient: gitserver.NewClient("test"),
	FetchTar:        fetchTarFromGithub,
	Path:            "/tmp/search_test/store",
	Log:             observation.TestContext.Logger,
	ObservationCtx:  &observation.TestContext,
}

func fetchTarFromGithub(ctx context.Context, repo api.RepoName, commit api.CommitID) (io.ReadCloser, error) {
	r, err := fetchTarFromGithubWithPaths(ctx, repo, commit, []string{})
	return r, err
}

func init() {
	// Clear out store so we pick up changes in our store writing code.
	os.RemoveAll(githubStore.Path)
}

func TestRegexSearch(t *testing.T) {
	zipData, _ := createZip(map[string]string{
		"a.go":      "aaaaa11111",
		"b.go":      "bbbbb22222",
		"c.go":      "ccccc3333",
		"README.md": "important info on go",
	})
	file, _ := mockZipFile(zipData)

	type args struct {
		ctx context.Context
		p   *protocol.PatternInfo
		zf  *zipFile
	}
	tests := []struct {
		name    string
		args    args
		wantFm  []protocol.FileMatch
		wantErr bool
	}{
		{
			name: "empty pattern returns a FileMatch with no LineMatches",
			args: args{
				ctx: context.Background(),
				// Check this case specifically.
				p: &protocol.PatternInfo{
					Query:                 &protocol.PatternNode{Value: ""},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: []protocol.FileMatch{{Path: "a.go"}},
		},
		{
			name: "'and' query with matches",
			args: args{
				ctx: context.Background(),
				p: &protocol.PatternInfo{
					Query: &protocol.AndNode{
						Children: []protocol.QueryNode{
							&protocol.PatternNode{Value: "aaaaa"},
							&protocol.PatternNode{Value: "11111"},
						},
					},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: []protocol.FileMatch{{
				Path: "a.go",
				ChunkMatches: []protocol.ChunkMatch{{
					Content:      "aaaaa11111",
					ContentStart: protocol.Location{0, 0, 0},
					Ranges: []protocol.Range{{
						Start: protocol.Location{0, 0, 0},
						End:   protocol.Location{5, 0, 5},
					}, {
						Start: protocol.Location{5, 0, 5},
						End:   protocol.Location{10, 0, 10},
					}},
				}},
			}},
		},
		{
			name: "'and' query with no matches",
			args: args{
				ctx: context.Background(),
				p: &protocol.PatternInfo{
					Query: &protocol.AndNode{
						Children: []protocol.QueryNode{
							&protocol.PatternNode{Value: "aaaaa"},
							&protocol.PatternNode{Value: "22222"},
						},
					},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: nil,
		},
		{
			name: "empty 'and' query",
			args: args{
				p: &protocol.PatternInfo{
					Query:                 &protocol.AndNode{},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: []protocol.FileMatch{{Path: "a.go"}},
		},
		{
			name: "'or' query with matches",
			args: args{
				ctx: context.Background(),
				p: &protocol.PatternInfo{
					Query: &protocol.OrNode{
						Children: []protocol.QueryNode{
							&protocol.PatternNode{Value: "aaaaa"},
							&protocol.PatternNode{Value: "99999"},
						},
					},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: []protocol.FileMatch{{
				Path: "a.go",
				ChunkMatches: []protocol.ChunkMatch{{
					Content:      "aaaaa11111",
					ContentStart: protocol.Location{0, 0, 0},
					Ranges: []protocol.Range{{
						Start: protocol.Location{0, 0, 0},
						End:   protocol.Location{5, 0, 5},
					}},
				}},
			}},
		},
		{
			name: "'or' query with no matches",
			args: args{
				ctx: context.Background(),
				p: &protocol.PatternInfo{
					Query: &protocol.OrNode{
						Children: []protocol.QueryNode{
							&protocol.PatternNode{Value: "jjjjj"},
							&protocol.PatternNode{Value: "99999"},
						},
					},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: nil,
		},
		{
			name: "empty 'or' query",
			args: args{
				p: &protocol.PatternInfo{
					Query:                 &protocol.OrNode{},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: nil,
		},
		{
			name: "query matches on content AND path",
			args: args{
				p: &protocol.PatternInfo{
					Query:                 &protocol.PatternNode{Value: "go"},
					PatternMatchesPath:    true,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: []protocol.FileMatch{{Path: "a.go"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patternInfo := tt.args.p
			patternInfo.IncludePaths = []string{`a\.go`}
			patternInfo.ExcludePaths = `README\.md`
			patternInfo.Limit = 5

			gotFm, err := regexSearchBatch(context.Background(), tt.args.p, tt.args.zf, 0)
			if (err != nil) != tt.wantErr {
				t.Errorf("regexSearch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotFm, tt.wantFm) {
				t.Errorf("regexSearch() gotFm = %v, want %v", gotFm, tt.wantFm)
			}
		})
	}
}

func TestLangFilters(t *testing.T) {
	zipData, _ := createZip(map[string]string{
		"a.go":      "aaaaa11111",
		"README.md": "important info on go",
		"file.m":    "[x,y,z] = sphere; \nr = 2;\nsurf(x*r,y*r,z*r)\naxis equal",
	})
	file, _ := mockZipFile(zipData)

	type args struct {
		ctx context.Context
		p   *protocol.PatternInfo
		zf  *zipFile
	}
	tests := []struct {
		name    string
		args    args
		wantFm  []protocol.FileMatch
		wantErr bool
	}{
		{
			name: "include filter with matches",
			args: args{
				ctx: context.Background(),
				p: &protocol.PatternInfo{
					Query:                 &protocol.PatternNode{Value: ""},
					IncludeLangs:          []string{"Go"},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: []protocol.FileMatch{{
				Path:     "a.go",
				Language: "Go",
			}},
		},
		{
			name: "include filter with no matches",
			args: args{
				ctx: context.Background(),
				p: &protocol.PatternInfo{
					Query:                 &protocol.PatternNode{Value: ""},
					IncludeLangs:          []string{"Go", "Markdown"},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: nil,
		},
		{
			name: "exclude filter with matches",
			args: args{
				ctx: context.Background(),
				p: &protocol.PatternInfo{
					Query:                 &protocol.PatternNode{Value: "aaaa11"},
					ExcludeLangs:          []string{"Markdown", "Ruby"},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: []protocol.FileMatch{{
				Path:     "a.go",
				Language: "Go",
				ChunkMatches: []protocol.ChunkMatch{{
					Content:      "aaaaa11111",
					ContentStart: protocol.Location{0, 0, 0},
					Ranges: []protocol.Range{{
						Start: protocol.Location{1, 0, 1},
						End:   protocol.Location{7, 0, 7},
					}},
				}},
			}},
		},
		{
			name: "include and exclude filters with matches",
			args: args{
				ctx: context.Background(),
				p: &protocol.PatternInfo{
					Query:                 &protocol.PatternNode{Value: ""},
					IncludeLangs:          []string{"Markdown"},
					ExcludeLangs:          []string{"Go", "Ruby"},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: []protocol.FileMatch{{
				Path:     "README.md",
				Language: "Markdown",
			}},
		},
		{
			name: "include filter with ambiguous extension",
			args: args{
				ctx: context.Background(),
				p: &protocol.PatternInfo{
					Query:                 &protocol.PatternNode{Value: ""},
					IncludeLangs:          []string{"MATLAB"},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: []protocol.FileMatch{{
				Path:     "file.m",
				Language: "MATLAB",
			}},
		},
		{
			name: "include filter with ambiguous extension and no matches",
			args: args{
				ctx: context.Background(),
				p: &protocol.PatternInfo{
					Query:                 &protocol.PatternNode{Value: ""},
					IncludeLangs:          []string{"Objective-C"},
					PatternMatchesPath:    false,
					PatternMatchesContent: true,
				},
				zf: file,
			},
			wantFm: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patternInfo := tt.args.p
			patternInfo.Limit = 5

			gotFm, err := regexSearchBatch(context.Background(), tt.args.p, tt.args.zf, 0)
			if (err != nil) != tt.wantErr {
				t.Errorf("regexSearch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotFm, tt.wantFm) {
				t.Errorf("regexSearch() gotFm = %v, want %v", gotFm, tt.wantFm)
			}
		})
	}
}

func Test_locsToRanges(t *testing.T) {
	cases := []struct {
		buf    string
		locs   [][]int
		ranges []protocol.Range
	}{
		{
			// simple multimatch
			buf:  "0.2.4.6.8.",
			locs: [][]int{{0, 2}, {4, 8}},
			ranges: []protocol.Range{{
				Start: protocol.Location{0, 0, 0},
				End:   protocol.Location{2, 0, 2},
			}, {
				Start: protocol.Location{4, 0, 4},
				End:   protocol.Location{8, 0, 8},
			}},
		}, {
			// multibyte match
			buf:  "0.2.🔧.8.",
			locs: [][]int{{2, 8}},
			ranges: []protocol.Range{{
				Start: protocol.Location{2, 0, 2},
				End:   protocol.Location{8, 0, 5},
			}},
		}, {
			// match crosses newlines and ends on a newline
			buf:  "0.2.4.6.\n9.11.14.17",
			locs: [][]int{{2, 9}},
			ranges: []protocol.Range{{
				Start: protocol.Location{2, 0, 2},
				End:   protocol.Location{9, 1, 0},
			}},
		}, {
			// match starts on a newline
			buf:  "0.2.4.6.\n9.11.14.17",
			locs: [][]int{{8, 11}},
			ranges: []protocol.Range{{
				Start: protocol.Location{8, 0, 8},
				End:   protocol.Location{11, 1, 2},
			}},
		}, {
			// match crosses a few lines and has multibyte chars
			buf:  "0.2.🔧.9.\n12.15.18.\n22.25.28.",
			locs: [][]int{{0, 25}},
			ranges: []protocol.Range{{
				Start: protocol.Location{0, 0, 0},
				End:   protocol.Location{25, 2, 3},
			}},
		}, {
			// multiple matches on different lines
			buf:  "0.2.🔧.9.\n12.15.18.\n22.25.28.",
			locs: [][]int{{0, 2}, {2, 3}, {10, 14}, {23, 28}},
			ranges: []protocol.Range{{
				Start: protocol.Location{0, 0, 0},
				End:   protocol.Location{2, 0, 2},
			}, {
				Start: protocol.Location{2, 0, 2},
				End:   protocol.Location{3, 0, 3},
			}, {
				Start: protocol.Location{10, 0, 7},
				End:   protocol.Location{14, 1, 2},
			}, {
				Start: protocol.Location{23, 2, 1},
				End:   protocol.Location{28, 2, 6},
			}},
		}, {
			// multiple adjacent matches with overlap
			buf:  "0.2.🔧.9.\n12.15.18.\n22.25.28.",
			locs: [][]int{{1, 3}, {3, 8}, {13, 14}, {14, 25}},
			ranges: []protocol.Range{{
				Start: protocol.Location{1, 0, 1},
				End:   protocol.Location{3, 0, 3},
			}, {
				Start: protocol.Location{3, 0, 3},
				End:   protocol.Location{8, 0, 5},
			}, {
				Start: protocol.Location{13, 1, 1},
				End:   protocol.Location{14, 1, 2},
			}, {
				Start: protocol.Location{14, 1, 2},
				End:   protocol.Location{25, 2, 3},
			}},
		},
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			got := locsToRanges([]byte(tc.buf), tc.locs)
			require.Equal(t, tc.ranges, got)
		})
	}
}

func TestFileLoader(t *testing.T) {
	zipData, err := createZip(map[string]string{
		"a": "content A",
		"b": "content B",
	})
	if err != nil {
		t.Fatal(err)
	}
	zf, err := mockZipFile(zipData)
	if err != nil {
		t.Fatal(err)
	}

	loader := fileLoader{zf: zf, isCaseSensitive: false}

	// Load a file and check its contents
	f1 := &zf.Files[0]
	loader.load(f1)
	content := loader.fileBuf
	matchContent := loader.fileMatchBuf
	require.Equal(t, f1, loader.currFile)
	require.Len(t, content, 9)
	require.NotEqual(t, content, matchContent)

	// Reload the file and check we return the same contents
	loader.load(f1)
	require.Equal(t, f1, loader.currFile)
	require.Equal(t, content, loader.fileBuf)
	require.Equal(t, matchContent, loader.fileMatchBuf)

	// Load another file
	f2 := &zf.Files[1]
	loader.load(f2)
	require.NotEqual(t, f1, loader.currFile)
	require.NotEqual(t, content, loader.fileBuf)
	require.NotEqual(t, loader.fileBuf, loader.fileMatchBuf)
}
