package ctags_config

import (
	"strings"

	"github.com/sourcegraph/sourcegraph/lib/codeintel/languages"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

type ParserType = uint8

const (
	UnknownCtags ParserType = iota
	NoCtags
	UniversalCtags
	ScipCtags
)

func ParserTypeToName(parserType ParserType) string {
	switch parserType {
	case NoCtags:
		return "off"
	case UniversalCtags:
		return "universal-ctags"
	case ScipCtags:
		return "scip-ctags"
	default:
		return "unknown-ctags-type"
	}
}

func ParserNameToParserType(name string) (ParserType, error) {
	switch name {
	case "off":
		return NoCtags, nil
	case "universal-ctags":
		return UniversalCtags, nil
	case "scip-ctags":
		return ScipCtags, nil
	default:
		return UnknownCtags, errors.Errorf("unknown parser type: %s", name)
	}
}

func ParserIsNoop(parserType ParserType) bool {
	return parserType == UnknownCtags || parserType == NoCtags
}

func LanguageSupportsParserType(language string, parserType ParserType) bool {
	switch parserType {
	case ScipCtags:
		_, ok := supportedLanguages[strings.ToLower(language)]
		return ok
	default:
		return true
	}
}

var supportedLanguages = map[string]struct{}{
	// TODO: Will support these after 5.1 release
	// "c":          {},
	// "cpp":        {},
	"c_sharp":    {},
	"go":         {},
	"java":       {},
	"javascript": {},
	"kotlin":     {},
	"magik":      {},
	"python":     {},
	"ruby":       {},
	"rust":       {},
	"scala":      {},
	"typescript": {},
	"zig":        {},
}

var DefaultEngines = map[string]ParserType{
	// Add the languages we want to turn on by default (you'll need to
	// update the ctags_config module for supported languages as well)
	"c_sharp":    ScipCtags,
	"go":         ScipCtags,
	"javascript": ScipCtags,
	"kotlin":     ScipCtags,
	"magik":      ScipCtags,
	"python":     ScipCtags,
	"ruby":       ScipCtags,
	"rust":       ScipCtags,
	"scala":      ScipCtags,
	"typescript": ScipCtags,
	"zig":        ScipCtags,

	// TODO: Not ready to turn on the following yet. Worried about not handling enough cases.
	// May wait until after next release
	// "c" / "c++"
	// "java":   ScipCtags,
}

func CreateEngineMap(siteConfig schema.SiteConfiguration) map[string]ParserType {
	// Set the defaults
	engines := make(map[string]ParserType)
	for lang, engine := range DefaultEngines {
		lang = languages.NormalizeLanguage(lang)
		engines[lang] = engine
	}

	// Set any relevant overrides
	configuration := siteConfig.SyntaxHighlighting
	if configuration != nil && configuration.Symbols != nil {
		for lang, engine := range configuration.Symbols.Engine {
			lang = languages.NormalizeLanguage(lang)

			if engine, err := ParserNameToParserType(engine); err != nil {
				engines[lang] = engine
			}
		}
	}

	return engines
}
