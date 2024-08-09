package chunkers

import (
	"context"
	"path"
	"strings"

	"github.com/go-enry/go-enry/v2"
	"github.com/pkg/errors"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/css"
	"github.com/smacker/go-tree-sitter/dockerfile"
	"github.com/smacker/go-tree-sitter/elixir"
	"github.com/smacker/go-tree-sitter/elm"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/groovy"
	"github.com/smacker/go-tree-sitter/hcl"
	"github.com/smacker/go-tree-sitter/html"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	markdown "github.com/smacker/go-tree-sitter/markdown/tree-sitter-markdown"
	"github.com/smacker/go-tree-sitter/php"
	"github.com/smacker/go-tree-sitter/protobuf"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/scala"
	"github.com/smacker/go-tree-sitter/sql"
	"github.com/smacker/go-tree-sitter/svelte"
	"github.com/smacker/go-tree-sitter/swift"
	"github.com/smacker/go-tree-sitter/toml"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/smacker/go-tree-sitter/yaml"

	ctxt "github.com/sourcegraph/sourcegraph/internal/codeintel/context"
)

type TreeSitterChunker struct {
	chunkOptions *ChunkOptions
}

func NewTreeSitterChunker(chunkOptions *ChunkOptions) *TreeSitterChunker {
	if chunkOptions == nil {
		chunkOptions = &defaultChunkOptions
	}
	return &TreeSitterChunker{chunkOptions: chunkOptions}
}

func (tsc *TreeSitterChunker) ChunkWithFallback(content, filename string) []ctxt.EmbeddableChunk {
	chunks, err := tsc.doChunk(content, filename)
	if err != nil {
		// Fall back to classic chunker in case of error
		chunks, _ = NewClassicChunker(tsc.chunkOptions).Chunk(content, filename)
	}
	return chunks
}

func (tsc *TreeSitterChunker) Chunk(content, filename string) ([]ctxt.EmbeddableChunk, error) {
	return tsc.doChunk(content, filename)
}

func (tsc *TreeSitterChunker) doChunk(content, filename string) ([]ctxt.EmbeddableChunk, error) {
	parser := sitter.NewParser()
	language := detectLanguage(filename)
	if language == nil {
		return nil, errors.New("no tree-sitter language detected")
	}
	parser.SetLanguage(language)
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(content))
	if err != nil {
		return nil, err
	}

	if tree.RootNode().ChildCount() == 0 || tree.RootNode().Child(0).HasError() {
		return nil, errors.New("error parsing with tree-sitter")
	}

	// Chunk the code
	spans := tsc.chunkNode(tree.RootNode())

	// Fill gaps
	for i := 0; i < len(spans)-1; i++ {
		spans[i] = Span{
			start: spans[i].start,
			end:   spans[i+1].start,
		}
	}

	// Combine small spans into bigger ones
	newSpans := make([]Span, 0)
	curSpan := Span{0, 0}
	for _, span := range spans {
		curSpan = curSpan.Add(span)

		if int(curSpan.Len()) > tsc.chunkOptions.ChunkTokensThreshold*ctxt.CHARS_PER_TOKEN {
			newSpans = append(newSpans, curSpan)
			curSpan = Span{span.end, span.end}
		}
	}
	if curSpan.Len() > 0 {
		newSpans = append(newSpans, curSpan)
	}

	spans = newSpans

	// Transform spans into chunks and filter out empty ones
	chunks := make([]ctxt.EmbeddableChunk, 0)
	line := 0

	for _, span := range spans {
		if span.Len() < 1 {
			continue
		}

		chunk := content[span.start:span.end]
		numLinesInChunk := strings.Count(chunk, "\n")
		chunks = append(chunks, ctxt.EmbeddableChunk{
			FileName:  filename,
			StartLine: line,
			EndLine:   line + numLinesInChunk,
			Content:   chunk,
		})
		line += numLinesInChunk
	}

	return chunks, nil
}

type Span struct {
	start uint32
	end   uint32
}

func (s Span) Len() uint32 {
	return s.end - s.start
}

func (s Span) Add(s2 Span) Span {
	return Span{s.start, s2.end}
}

func (tsc *TreeSitterChunker) chunkNode(node *sitter.Node) []Span {
	chunks := make([]Span, 0)
	curSpan := Span{start: node.StartByte(), end: node.EndByte()}

	maxBytes := uint32(tsc.chunkOptions.ChunkTokensThreshold * ctxt.CHARS_PER_TOKEN)

	for idx := range node.ChildCount() {
		child := node.Child(int(idx))
		if child.EndByte()-child.StartByte() > maxBytes {
			// Recursively chunk the child
			chunks = append(chunks, curSpan)
			curSpan = Span{child.EndByte(), child.EndByte()}
			chunks = append(chunks, tsc.chunkNode(child)...)
		} else if child.EndByte()-child.StartByte()+curSpan.Len() > maxBytes {
			// Start a new span
			chunks = append(chunks, curSpan)
			curSpan = Span{child.StartByte(), child.EndByte()}
		} else {
			// Add to the current span
			curSpan = curSpan.Add(Span{child.EndByte(), child.EndByte()})
		}
	}

	return append(chunks, curSpan)
}

func detectLanguage(filename string) *sitter.Language {
	lang, _ := enry.GetLanguageByExtension(path.Base(filename))
	// TODO do we care about "safe"?

	// TODO file extension preferences
	// h: C++
	if lang == "RenderScript" {
		lang = "Rust"
	} else if lang == "GCC Machine Description" {
		lang = "Markdown"
	} else if lang == "Hack" {
		lang = "PHP"
	}

	switch lang {
	case "Python":
		return python.GetLanguage()
	case "JavaScript", "JSX":
		return javascript.GetLanguage()
	case "TypeScript":
		return typescript.GetLanguage()
	case "TSX":
		return tsx.GetLanguage()
	case "Java":
		return java.GetLanguage()
	case "Scala":
		return scala.GetLanguage()
	case "Kotlin":
		return kotlin.GetLanguage()
	case "C":
		return c.GetLanguage()
	case "C++":
		return cpp.GetLanguage()
	case "C#":
		return csharp.GetLanguage()
	case "Go":
		return golang.GetLanguage()
	case "Ruby":
		return ruby.GetLanguage()
	case "Rust":
		return rust.GetLanguage()
	case "PHP":
		return php.GetLanguage()
	case "Shell":
		return bash.GetLanguage()
	case "Swift":
		return swift.GetLanguage()
	case "SQL":
		return sql.GetLanguage()
	case "TOML":
		return toml.GetLanguage()
	case "CSS":
		return css.GetLanguage()
	case "Dockerfile":
		return dockerfile.GetLanguage()
	case "Elixir":
		return elixir.GetLanguage()
	case "Elm":
		return elm.GetLanguage()
	case "Groovy":
		return groovy.GetLanguage()
	case "HCL":
		return hcl.GetLanguage()
	case "HTML":
		return html.GetLanguage()
	case "Markdown":
		return markdown.GetLanguage()
	case "Protocol Buffer":
		return protobuf.GetLanguage()
	case "Svelte":
		return svelte.GetLanguage()
	case "YAML":
		return yaml.GetLanguage()
	default:
		return nil
	}

}
