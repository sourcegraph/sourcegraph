package htmlutil_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/htmlutil"
)

func TestSanitizationPolicy(t *testing.T) {
	cmp := func(tb testing.TB, got, want string) {
		tb.Helper()
		if !cmp.Equal(want, got) {
			tb.Fatalf("html not sanitized correctly: (+want) (-got)\n+ %s\n- %s", want, got)
		}
	}

	for _, tt := range []struct {
		name      string
		inputHTML string
		want      string
	}{
		{
			name:      "a.name contains whitespace, letters, numbers, hyphens, or underscores",
			inputHTML: `<a name="safe link"/><a name="#illegal"/>`,
			want:      `<a name="safe link"/>`,
		},
		{
			name:      "only a.rel must be nofollow",
			inputHTML: `<a rel="nofollow"/><a rel="illegal"/>`,
			want:      `<a rel="nofollow"/>`,
		},
		{
			name:      "a.class must be anchor",
			inputHTML: `<a class="anchor"/><a class="illegal"/>`,
			want:      `<a class="anchor"/>`,
		},
		{
			name:      "a.aria-hidden must be true",
			inputHTML: `<a aria-hidden="true"/><a aria-hidden="false"/>`,
			want:      `<a aria-hidden="true"/>`,
		},
		{
			name:      "input.type must be checkbox",
			inputHTML: `<input type="checkbox"/><input type="number"/>`,
			want:      `<input type="checkbox"/>`,
		},
		{
			name:      "input.checked if true must be either empty or have no value",
			inputHTML: `<input checked=""/><input checked/><input checked="checked"/><input checked="true"/>`,
			want:      `<input checked=""/><input checked=""/>`,
		},
		{
			name:      "input.disabled if true must be either empty or have no value",
			inputHTML: `<input disabled=""/><input disabled/><input disabled="disabled"/><input disabled="true"/>`,
			want:      `<input disabled=""/><input disabled=""/>`,
		},
		{
			name:      "pre.class is either chroma or starts with chroma-",
			inputHTML: `<pre class="chroma"/><pre class="chroma-c1"/><pre class="illegal">Keep me</pre>`,
			want:      `<pre class="chroma"/><pre class="chroma-c1"/><pre>Keep me</pre>`, // here we just drop the illegal class
		},
		{
			name:      "code.class is either chroma or starts with chroma-",
			inputHTML: `<code class="chroma"/><code class="chroma-c1"/><code class="illegal">Keep me</code>`,
			want:      `<code class="chroma"/><code class="chroma-c1"/><code>Keep me</code>`, // here we just drop the illegal class
		},
		{
			name:      "span.class can be chroma or start with chroma-",
			inputHTML: `<span class="chroma"/><span class="chroma-c1"/><span class="illegal">Keep me</span>`,
			want:      `<span class="chroma"/><span class="chroma-c1"/><span>Keep me</span>`, // here we just drop the illegal class
		},
		{
			name:      "span.class can start with ansi-",
			inputHTML: `<span class="ansi-red"/><span class="illegal">Keep me</span>`,
			want:      `<span class="ansi-red"/><span>Keep me</span>`, // here we just drop the illegal class
		},
		{
			name:      "align is only allowed in img and p elements",
			inputHTML: `<img align="top"/><p align="left"/><div align="right"></div>`,
			want:      `<img align="top"/><p align="left"/><div></div>`, // here we just drop the illegal attribute
		},
		{
			name:      "picture is only allowed sans-attributes",
			inputHTML: `<picture class="good-one" width="900" src="example.com/pic"/>`,
			want:      `<picture/>`,
		},
		{
			name:      "allowed video attributes",
			inputHTML: `<video src="example.com/video.mp4" poster="example.com/thumbnail.png" width="250" height="250" playsinline muted autoplay loop controls />`,
			want:      `<video src="example.com/video.mp4" poster="example.com/thumbnail.png" width="250" height="250" playsinline="" muted="" autoplay="" loop="" controls=""/>`,
		},
		{
			name:      "allowed video attributes",
			inputHTML: `<video src="example.com/video.mp4" poster="example.com/thumbnail.png" width="250" height="250" playsinline muted autoplay loop controls />`,
			want:      `<video src="example.com/video.mp4" poster="example.com/thumbnail.png" width="250" height="250" playsinline="" muted="" autoplay="" loop="" controls=""/>`,
		},
		{
			name:      "allowed track attributes",
			inputHTML: `<track src="example.com/track.vtt" kind="subtitles" srclang="en" default label="High Speed Dirt" />`,
			want:      `<track src="example.com/track.vtt" kind="subtitles" srclang="en" default="" label="High Speed Dirt"/>`,
		},
		{
			name:      "allowed source attributes",
			inputHTML: `<source srcset="logo-wide.png" src="example.com/logo.png" type="image/png" media="{min-width: 800px}" width="600" height="900" sizes="1x,2x"/>`,
			want:      `<source srcset="logo-wide.png" src="example.com/logo.png" type="image/png" media="{min-width: 800px}" width="600" height="900" sizes="1x,2x"/>`,
		},
		{
			name:      "parseable fully-qualified links get target=_blank",
			inputHTML: `<a href="https://example.com"/><area href="http://example.com"/>`,
			want:      `<a href="https://example.com" rel="nofollow noopener" target="_blank"/><area href="http://example.com" rel="nofollow"/>`,
		},
	} {
		// htmlutil should enforce a set of sanitization rules to make sure the generated HTML is safe to render.
		// We also want to make sure all 3 wrapper functions produce identical results.
		t.Run(tt.name, func(t *testing.T) {
			t.Run("Sanitize", func(t *testing.T) {
				cmp(t, htmlutil.Sanitize(tt.inputHTML), tt.want)
			})

			t.Run("SanitizeBytes", func(t *testing.T) {
				b := htmlutil.SanitizeBytes([]byte(tt.inputHTML))
				cmp(t, string(b), tt.want)
			})

			t.Run("SanitizeReader", func(t *testing.T) {
				r := strings.NewReader(tt.inputHTML)
				cmp(t, htmlutil.SanitizeReader(r).String(), tt.want)
			})
		})
	}
}

func TestPatchChromaTypes(t *testing.T) {
	_ = htmlutil.SyntaxHighlightingOptions()
	checkChromaTypes(t)

	// Types are patched once, even if called multiple times.
	_ = htmlutil.SyntaxHighlightingOptions()
	checkChromaTypes(t)
}

// checkChromaTypes checks that all chroma types except chroma.PreWrapper
// have "chroma-" prefix.
func checkChromaTypes(tb testing.TB) {
	tb.Helper()
	prefix := "chroma-"
	for t, cls := range chroma.StandardTypes {
		has := strings.HasPrefix(cls, prefix)
		if t == chroma.PreWrapper {
			require.False(tb, has, "chroma.PreWrapper should not have a custom prefix, got %s", cls)
		} else {
			require.True(tb, has, "type %s should have %q prefix, got %q", t, prefix, cls)
		}
	}
}
