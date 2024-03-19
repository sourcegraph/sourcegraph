package inventory

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-enry/go-enry/v2"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestGetLang_language(t *testing.T) {
	tests := map[string]struct {
		file fi
		want Lang
	}{
		"empty file": {file: fi{"a.java", ""}, want: Lang{
			Name:       "Java",
			TotalBytes: 0,
			TotalLines: 0,
		}},
		"empty file_unsafe_path": {file: fi{"a.ml", ""}, want: Lang{
			Name:       "",
			TotalBytes: 0,
			TotalLines: 0,
		}},
		"java": {file: fi{"a.java", "a"}, want: Lang{
			Name:       "Java",
			TotalBytes: 1,
			TotalLines: 1,
		}},
		"go": {file: fi{"a.go", "a"}, want: Lang{
			Name:       "Go",
			TotalBytes: 1,
			TotalLines: 1,
		}},
		"go-with-newline": {file: fi{"a.go", "a\n"}, want: Lang{
			Name:       "Go",
			TotalBytes: 2,
			TotalLines: 1,
		}},
		// Ensure that .tsx and .jsx are considered as valid extensions for TypeScript and JavaScript,
		// respectively.
		"override tsx": {file: fi{"a.tsx", "xx"}, want: Lang{
			Name:       "TypeScript",
			TotalBytes: 2,
			TotalLines: 1,
		}},
		"override jsx": {file: fi{"b.jsx", "x"}, want: Lang{
			Name:       "JavaScript",
			TotalBytes: 1,
			TotalLines: 1,
		}},
	}
	for label, test := range tests {
		t.Run(label, func(t *testing.T) {
			lang, err := getLang(context.Background(),
				test.file,
				make([]byte, fileReadBufferSize),
				makeFileReader(test.file.Contents))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(lang, test.want) {
				t.Errorf("Got %q, want %q", lang, test.want)
			}
		})
	}
}

func makeFileReader(contents string) func(context.Context, string) (io.ReadCloser, error) {
	return func(ctx context.Context, path string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(contents)), nil
	}
}

type fi struct {
	Path     string
	Contents string
}

func (f fi) Name() string {
	return f.Path
}

func (f fi) Size() int64 {
	return int64(len(f.Contents))
}

func (f fi) IsDir() bool {
	return false
}

func (f fi) Mode() os.FileMode {
	return os.FileMode(0)
}

func (f fi) ModTime() time.Time {
	return time.Now()
}

func (f fi) Sys() any {
	return any(nil)
}

func TestGet_readFile(t *testing.T) {
	tests := []struct {
		file fs.FileInfo
		want string
	}{
		{file: fi{"a.java", "aaaaaaaaa"}, want: "Java"},
		{file: fi{"b.md", "# Hello"}, want: "Markdown"},

		// The .m extension is used by many languages, but this code is obviously Objective-C. This
		// test checks that this file is detected correctly as Objective-C.
		{
			file: fi{"c.m", "@interface X:NSObject { double x; } @property(nonatomic, readwrite) double foo;"},
			want: "Objective-C",
		},
	}
	for _, test := range tests {
		t.Run(test.file.Name(), func(t *testing.T) {
			fr := makeFileReader(test.file.(fi).Contents)
			lang, err := getLang(context.Background(), test.file, make([]byte, fileReadBufferSize), fr)
			if err != nil {
				t.Fatal(err)
			}
			if lang.Name != test.want {
				t.Errorf("got %q, want %q", lang.Name, test.want)
			}
		})
	}
}

type nopReadCloser struct {
	data   []byte
	reader *bytes.Reader
}

func (n *nopReadCloser) Read(p []byte) (int, error) {
	return n.reader.Read(p)
}

func (n *nopReadCloser) Close() error {
	return nil
}

func BenchmarkGetLang(b *testing.B) {
	files, err := readFileTree("prom-repo-tree.txt")
	if err != nil {
		b.Fatal(err)
	}
	fr := newFileReader(files)
	buf := make([]byte, fileReadBufferSize)
	b.Logf("Calling Get on %d files.", len(files))
	b.ResetTimer()
	for range b.N {
		for _, file := range files {
			_, err = getLang(context.Background(), file, buf, fr)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkIsVendor(b *testing.B) {
	files, err := readFileTree("prom-repo-tree.txt")
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("Calling IsVendor on %d files.", len(files))

	b.ResetTimer()
	for range b.N {
		for _, f := range files {
			_ = enry.IsVendor(f.Name())
		}
	}
}

func newFileReader(files []fs.FileInfo) func(_ context.Context, path string) (io.ReadCloser, error) {
	m := make(map[string]*nopReadCloser, len(files))
	for _, f := range files {
		data := []byte(f.(fi).Contents)
		m[f.Name()] = &nopReadCloser{
			data:   data,
			reader: bytes.NewReader(data),
		}
	}
	return func(_ context.Context, path string) (io.ReadCloser, error) {
		nc, ok := m[path]
		if !ok {
			return nil, errors.Errorf("no file: %s", path)
		}
		nc.reader.Reset(nc.data)
		return nc, nil
	}
}

func readFileTree(name string) ([]fs.FileInfo, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var files []fs.FileInfo
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		path := scanner.Text()
		files = append(files, fi{path, fakeContents(path)})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

func fakeContents(path string) string {
	switch filepath.Ext(path) {
	case ".html":
		return `<html><head><title>hello</title></head><body><h1>hello</h1></body></html>`
	case ".go":
		return `package foo

import "fmt"

// Foo gets foo.
func Foo(x *string) (chan struct{}) {
	panic("hello, world")
}
`
	case ".js":
		return `import { foo } from 'bar'

export function baz(n) {
	return document.getElementById('x')
}
`
	case ".m":
		return `@interface X:NSObject {
	double x;
}

@property(nonatomic, readwrite) double foo;`
	default:
		return ""
	}
}
