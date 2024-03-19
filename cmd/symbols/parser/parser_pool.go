package parser

import (
	"context"

	"github.com/sourcegraph/go-ctags"

	"github.com/sourcegraph/sourcegraph/internal/ctags_config"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type ParserFactory func(ctags_config.ParserType) (ctags.Parser, error)

type parserPool struct {
	newParser ParserFactory
	pool      map[ctags_config.ParserType]chan ctags.Parser
}

var DefaultParserTypes = []ctags_config.ParserType{ctags_config.UniversalCtags, ctags_config.ScipCtags}

func NewParserPool(newParser ParserFactory, numParserProcesses int, parserTypes []ctags_config.ParserType) (*parserPool, error) {
	pool := make(map[ctags_config.ParserType]chan ctags.Parser)

	if len(parserTypes) == 0 {
		parserTypes = DefaultParserTypes
	}

	// NOTE: We obviously don't make `NoCtags` available in the pool.
	for _, parserType := range parserTypes {
		pool[parserType] = make(chan ctags.Parser, numParserProcesses)
		for range numParserProcesses {
			parser, err := newParser(parserType)
			if err != nil {
				return nil, err
			}
			pool[parserType] <- parser
		}
	}

	parserPool := &parserPool{
		newParser: newParser,
		pool:      pool,
	}

	return parserPool, nil
}

// Get a parser from the pool. Once this parser is no longer in use, the Done method
// MUST be called with either the parser or a nil value (when countering an error).
// Nil values will be recreated on-demand via the factory supplied when constructing
// the pool. This method always returns a non-nil parser with a nil error value.
//
// This method blocks until a parser is available or the given context is canceled.
func (p *parserPool) Get(ctx context.Context, source ctags_config.ParserType) (ctags.Parser, error) {
	if ctags_config.ParserIsNoop(source) {
		return nil, errors.New("NoCtags is not a valid ParserType")
	}

	pool := p.pool[source]

	select {
	case parser := <-pool:
		if parser != nil {
			return parser, nil
		}

		return p.newParser(source)

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *parserPool) Done(parser ctags.Parser, source ctags_config.ParserType) {
	pool := p.pool[source]
	pool <- parser
}
