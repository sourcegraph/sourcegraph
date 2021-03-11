package conversion

import (
	"context"
	"io"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/lsif/protocol/reader"
)

type Pair struct {
	Element Element
	Err     error
}

// Read reads the given content as line-separated JSON objects and returns a channel of Pair values
// for each non-empty line.
func Read(ctx context.Context, r io.Reader) <-chan Pair {
	elements := make(chan Pair)

	go func() {
		defer close(elements)

		for pair := range reader.Read(ctx, r) {
			element := Element{
				ID:      pair.Element.ID,
				Type:    pair.Element.Type,
				Label:   pair.Element.Label,
				Payload: translatePayload(pair.Element.Payload),
			}

			elements <- Pair{Element: element, Err: pair.Err}
		}
	}()

	return elements
}

func translatePayload(payload interface{}) interface{} {
	switch v := payload.(type) {
	case reader.Edge:
		return Edge(v)

	case reader.MetaData:
		return MetaData(v)

	case reader.PackageInformation:
		return PackageInformation(v)

	case reader.Diagnostic:
		return Diagnostic(v)

	case reader.Range:
		return Range{Range: v}

	case reader.ResultSet:
		return ResultSet{ResultSet: v}

	case reader.Moniker:
		return Moniker{Moniker: v}

	case []reader.Diagnostic:
		diagnostics := make([]Diagnostic, 0, len(v))
		for _, v := range v {
			diagnostics = append(diagnostics, Diagnostic(v))
		}

		return diagnostics
	}

	return payload
}
