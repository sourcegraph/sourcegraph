package symbols

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	ctags "github.com/sourcegraph/go-ctags"

	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/sqliteutil"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	symbolsclient "github.com/sourcegraph/sourcegraph/internal/symbols"
)

func init() {
	sqliteutil.SetLocalLibpath()
}

func TestIsLiteralEquality(t *testing.T) {
	type TestCase struct {
		Regex       string
		WantOk      bool
		WantLiteral string
	}

	for _, test := range []TestCase{
		{Regex: `^foo$`, WantLiteral: "foo", WantOk: true},
		{Regex: `^[f]oo$`, WantLiteral: `foo`, WantOk: true},
		{Regex: `^\\$`, WantLiteral: `\`, WantOk: true},
		{Regex: `^\$`, WantOk: false},
		{Regex: `^\($`, WantLiteral: `(`, WantOk: true},
		{Regex: `\\`, WantOk: false},
		{Regex: `\$`, WantOk: false},
		{Regex: `\(`, WantOk: false},
		{Regex: `foo$`, WantOk: false},
		{Regex: `(^foo$|^bar$)`, WantOk: false},
	} {
		gotOk, gotLiteral, err := isLiteralEquality(test.Regex)
		if err != nil {
			t.Fatal(err)
		}
		if gotOk != test.WantOk {
			t.Errorf("isLiteralEquality(%s) returned %t, wanted %t", test.Regex, gotOk, test.WantOk)
		}
		if gotLiteral != test.WantLiteral {
			t.Errorf("isLiteralEquality(%s) returned the literal %s, wanted %s", test.Regex, gotLiteral, test.WantLiteral)
		}
	}
}

func TestService(t *testing.T) {
	sqliteutil.MustRegisterSqlite3WithPcre()

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.RemoveAll(tmpDir) }()

	files := map[string]string{"a.js": "var x = 1"}
	service := Service{
		FetchTar: func(ctx context.Context, repo api.RepoName, commit api.CommitID) (io.ReadCloser, error) {
			return createTar(files)
		},
		NewParser: func() (ctags.Parser, error) {
			return mockParser{"x", "y"}, nil
		},
		Path: tmpDir,
	}

	if err := service.Start(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()
	client := symbolsclient.Client{URL: server.URL}
	x := result.Symbol{Name: "x", Path: "a.js"}
	y := result.Symbol{Name: "y", Path: "a.js"}

	tests := map[string]struct {
		args search.SymbolsParameters
		want result.Symbols
	}{
		"simple": {
			args: search.SymbolsParameters{First: 10},
			want: []result.Symbol{x, y},
		},
		"onematch": {
			args: search.SymbolsParameters{Query: "x", First: 10},
			want: []result.Symbol{x},
		},
		"nomatches": {
			args: search.SymbolsParameters{Query: "foo", First: 10},
			want: nil,
		},
		"caseinsensitiveexactmatch": {
			args: search.SymbolsParameters{Query: "^X$", First: 10},
			want: []result.Symbol{x},
		},
		"casesensitiveexactmatch": {
			args: search.SymbolsParameters{Query: "^x$", IsCaseSensitive: true, First: 10},
			want: []result.Symbol{x},
		},
		"casesensitivenoexactmatch": {
			args: search.SymbolsParameters{Query: "^X$", IsCaseSensitive: true, First: 10},
			want: nil,
		},
		"caseinsensitiveexactpathmatch": {
			args: search.SymbolsParameters{IncludePatterns: []string{"^A.js$"}, First: 10},
			want: []result.Symbol{x, y},
		},
		"casesensitiveexactpathmatch": {
			args: search.SymbolsParameters{IncludePatterns: []string{"^a.js$"}, IsCaseSensitive: true, First: 10},
			want: []result.Symbol{x, y},
		},
		"casesensitivenoexactpathmatch": {
			args: search.SymbolsParameters{IncludePatterns: []string{"^A.js$"}, IsCaseSensitive: true, First: 10},
			want: nil,
		},
		"exclude": {
			args: search.SymbolsParameters{ExcludePattern: "a.js", IsCaseSensitive: true, First: 10},
			want: nil,
		},
	}
	for label, test := range tests {
		t.Run(label, func(t *testing.T) {
			result, err := client.Search(context.Background(), test.args)
			if err != nil {
				t.Fatal(err)
			}
			if result != nil && !reflect.DeepEqual(*result, test.want) {
				t.Errorf("got %+v, want %+v", *result, test.want)
			}
			if result == nil && test.want != nil {
				t.Errorf("got nil, want %+v", test.want)
			}
		})
	}
}

func createTar(files map[string]string) (io.ReadCloser, error) {
	buf := new(bytes.Buffer)
	w := tar.NewWriter(buf)
	for name, body := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(body)),
		}
		if err := w.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(body)); err != nil {
			return nil, err
		}
	}

	err := w.Close()
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

type mockParser []string

func (m mockParser) Parse(name string, content []byte) ([]*ctags.Entry, error) {
	entries := make([]*ctags.Entry, len(m))
	for i, name := range m {
		entries[i] = &ctags.Entry{Name: name, Path: "a.js"}
	}
	return entries, nil
}

func (mockParser) Close() {}
