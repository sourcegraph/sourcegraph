package graphqlbackend

import (
	"testing"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/highlight"
)

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  bool
	}{
		{
			name:  "text UTF8",
			input: []byte("hello world!"),
			want:  false,
		},
		{
			name: "text ISO-8859-1",
			// "hellö world"
			input: []byte{0x68, 0x65, 0x6c, 0x6c, 0xf6, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64, 0x0a},
			want:  false,
		},
		{
			name: "text UTF16 LE with BOM",
			// "hellö world"
			input: []byte{0xff, 0xfe, 0x68, 0x00, 0x65, 0x00, 0x6c, 0x00, 0x6c, 0x00, 0xf6, 0x00, 0x20, 0x00, 0x77, 0x00, 0x6f, 0x00, 0x72, 0x00, 0x6c, 0x00, 0x64, 0x00, 0x0a, 0x00},
			want:  false,
		},
		{
			// tests files that ARE valid text, but whose mimetype is e.g.
			// "application/postscript" rather than a "text/foobar" mimetype.
			name:  "text postscript",
			input: []byte("%!PS-Adobe-"),
			want:  false,
		},
		{
			name:  "text JSON",
			input: []byte(`{"this is": "some JSON"}`),
			want:  false,
		},
		{
			name:  "binary nonsense",
			input: []byte{0, 1, 255, 3, 4, 5, 6, 7, 8, 9},
			want:  true,
		},
		{
			name: "binary WAV audio",
			// https://sourcegraph.com/github.com/golang/go@a4330ed694c588d495f7c72a9cbb0cd39dde31e8/-/blob/src/net/http/sniff_test.go#L45
			input: []byte("RIFFb\xb8\x00\x00WAVEfmt \x12\x00\x00\x00\x06"),
			want:  true,
		},
		{
			name: "binary MP4 video",
			// https://sourcegraph.com/github.com/golang/go@a4330ed694c588d495f7c72a9cbb0cd39dde31e8/-/blob/src/net/http/sniff_test.go#L55
			input: []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom<\x06t\xbfmdat"),
			want:  true,
		},
		{
			name:  "binary PNG image",
			input: []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52},
			want:  true,
		},
	}
	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			got := highlight.IsBinary(tst.input)
			if got != tst.want {
				t.Fatalf("got %v want %v", got, tst.want)
			}
		})
	}
}
