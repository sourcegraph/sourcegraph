package lsifstore

import (
	"encoding/base64"
	"strings"

	"github.com/sourcegraph/scip/bindings/go/scip"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/symbols"
)

func translateRange(r *scip.Range) shared.Range {
	return newRange(int(r.Start.Line), int(r.Start.Character), int(r.End.Line), int(r.End.Character))
}

func newRange(startLine, startCharacter, endLine, endCharacter int) shared.Range {
	return shared.Range{
		Start: shared.Position{
			Line:      startLine,
			Character: startCharacter,
		},
		End: shared.Position{
			Line:      endLine,
			Character: endCharacter,
		},
	}
}

func explodeSymbol(symbol string) (string, error) {
	s, err := symbols.NewExplodedSymbol(symbol)
	if err != nil {
		return "", err
	}

	return strings.Join([]string{
		base64.StdEncoding.EncodeToString([]byte(s.Scheme)),
		base64.StdEncoding.EncodeToString([]byte(s.PackageManager)),
		base64.StdEncoding.EncodeToString([]byte(s.PackageName)),
		base64.StdEncoding.EncodeToString([]byte(s.PackageVersion)),
		base64.StdEncoding.EncodeToString([]byte(s.DescriptorNamespace)),
		base64.StdEncoding.EncodeToString([]byte(s.DescriptorSuffix)),
	}, "$"), nil
}
