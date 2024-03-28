package shared

import (
	"fmt"
	"strings"
)

type CodeIntelIndexer struct {
	LanguageKey  string
	Name         string
	URN          string
	DockerImages []string
}

const SyntacticIndexer = "scip-syntax"

// allIndexers is a list of all detectable/suggested indexers known to Sourcegraph.
// Two indexers with the same language key will be preferred according to the given order.
var allIndexers = []CodeIntelIndexer{
	// C++
	makeIndexer("C++", "scip-clang", "github.com/sourcegraph/scip-clang"),
	makeInternalIndexer("C++", "lsif-clang"),
	makeInternalIndexer("C++", "lsif-cpp"),

	// Dart
	makeIndexer("Dart", "scip-dart", "github.com/Workiva/scip-dart"),
	makeInternalIndexer("Dart", "lsif-dart"),
	makeIndexer("Dart", "lsif_indexer", "github.com/Workiva/lsif_indexer"),

	// DotNet
	makeInternalIndexer("DotNet", "scip-dotnet"),
	makeIndexer("DotNet", "lsif-dotnet", "github.com/tcz717/LsifDotnet"),

	// Go
	makeInternalIndexer("Go", "scip-go"),
	makeInternalIndexer("Go", "lsif-go"),

	// HIE
	makeIndexer("HIE", "hie-lsif", " github.com/mpickering/hie-lsif"),

	// Jsonnet
	makeInternalIndexer("Jsonnet", "lsif-jsonnet"),

	// JVM: Java, Scala, Kotlin
	makeInternalIndexer("JVM", "scip-java"),
	makeInternalIndexer("JVM", "lsif-java"),

	// OCaml
	makeIndexer("OCaml", "lsif-ocaml", "github.com/rvantonder/lsif-ocaml"),

	// PHP
	makeIndexer("PHP", "scip-php", "github.com/davidrjenni/scip-php", "davidrjenni/scip-php"),
	makeIndexer("PHP", "lsif-php", "github.com/davidrjenni/lsif-php", "davidrjenni/lsif-php"),

	// Python
	makeInternalIndexer("Python", "scip-python"),

	// Ruby, Sorbet
	makeInternalIndexer("Ruby", "scip-ruby"),

	// Rust
	makeInternalIndexer("Rust", "scip-rust"),
	makeIndexer("Rust", "rust-analyzer", "github.com/rust-lang/rust-analyzer"),

	// Terraform
	makeIndexer("Terraform", "lsif-terraform", "github.com/juliosueiras/lsif-terraform"),

	// TypeScript, JavaScript
	makeInternalIndexer("TypeScript", "scip-typescript"),
	makeInternalIndexer("TypeScript", "lsif-node"),
}

func NamesForKey(key string) []string {
	var names []string
	for _, indexer := range allIndexers {
		if indexer.LanguageKey == key {
			names = append(names, indexer.Name)
		}
	}

	return names
}

var extensions = map[string][]string{
	"C++":        {".c", ".cp", ".cpp", ".cxx", ".h", ".hpp", ".hxx"},
	"Dart":       {".dart"},
	"DotNet":     {".cs", ".fs"},
	"Go":         {".go"},
	"HIE":        {".hs"},
	"Jsonnet":    {".jsonnet"},
	"JVM":        {".java", ".kt", ".scala"},
	"OCaml":      {".ml"},
	"PHP":        {".php"},
	"Python":     {".py"},
	"Ruby":       {".rb"},
	"Rust":       {".rs"},
	"Terraform":  {".tf"},
	"TypeScript": {".js", ".jsx", ".ts", ".tsx"},
}

var imageToIndexer = func() map[string]CodeIntelIndexer {
	m := map[string]CodeIntelIndexer{}
	for _, indexer := range allIndexers {
		for _, dockerImage := range indexer.DockerImages {
			m[dockerImage] = indexer
		}
	}

	return m
}()

var PreferredIndexers = func() map[string]CodeIntelIndexer {
	preferred := map[string]CodeIntelIndexer{}

	m := map[string]CodeIntelIndexer{}
	for _, indexer := range allIndexers {
		if p, ok := preferred[indexer.LanguageKey]; ok {
			m[indexer.Name] = p
		} else {
			m[indexer.Name] = indexer
			preferred[indexer.LanguageKey] = indexer
		}
	}

	return m
}()

// A map of file extension to a list of indexers in order of recommendation from most to least.
var LanguageToIndexer = func() map[string][]CodeIntelIndexer {
	m := map[string][]CodeIntelIndexer{}
	for _, indexer := range allIndexers {
		for _, extension := range extensions[indexer.LanguageKey] {
			m[extension] = append(m[extension], indexer)
		}
	}

	return m
}()

func makeInternalIndexer(key, name string) CodeIntelIndexer {
	return makeIndexer(
		key,
		name,
		fmt.Sprintf("github.com/sourcegraph/%s", name),
		fmt.Sprintf("sourcegraph/%s", name),
	)
}

func makeIndexer(key, name, urn string, dockerImages ...string) CodeIntelIndexer {
	return CodeIntelIndexer{
		LanguageKey:  key,
		Name:         name,
		URN:          urn,
		DockerImages: dockerImages,
	}
}

func IndexerFromName(name string) CodeIntelIndexer {
	// drop the Docker image tag if one exists
	name = strings.Split(name, "@sha256:")[0]
	name = strings.Split(name, ":")[0]

	if indexer, ok := imageToIndexer[name]; ok {
		return indexer
	}

	for _, indexer := range allIndexers {
		if indexer.Name == name {
			return indexer
		}
	}

	return CodeIntelIndexer{Name: name}
}
