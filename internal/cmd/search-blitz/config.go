package main

import (
	"bytes"
	"embed"
	_ "embed"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

//go:embed queries*.txt
var queriesFS embed.FS

//go:embed attribution/*.txt
var attributionFS embed.FS

type Config struct {
	Queries []*QueryConfig
}

type QueryConfig struct {
	Query   string
	Snippet string
	Name    string

	// An unset interval defaults to 5m
	Interval time.Duration

	// An empty value for Protocols means "all"
	Protocols []Protocol
}

var allProtocols = []Protocol{Batch, Stream}

// Protocol represents either the graphQL Protocol or the streaming Protocol
type Protocol uint8

const (
	Batch Protocol = iota
	Stream
)

// Loads queries from queryFile if set, otherwise falls back to
// embedded set of queries.
func loadConfig(queryFile, env string) (_ *Config, err error) {
	if queryFile != "" {
		queriesRaw, err := os.ReadFile(queryFile)
		if err != nil {
			return nil, err
		}
		queries, err := parseQueries(queriesRaw)
		if err != nil {
			return nil, err
		}
		return &Config{Queries: queries}, nil
	}

	if env == "" {
		env = "cloud"
	}

	queriesRaw, err := queriesFS.ReadFile("queries.txt")
	if err != nil {
		return nil, err
	}

	if env != "cloud" {
		extra, err := queriesFS.ReadFile("queries_" + env + ".txt")
		if err != nil {
			return nil, err
		}
		queriesRaw = append(queriesRaw, '\n')
		queriesRaw = append(queriesRaw, extra...)
	}

	var queries []*QueryConfig
	attributions, err := loadAttributions()
	if err != nil {
		return nil, err
	}
	queries = append(queries, attributions...)

	parsedQueries, err := parseQueries(queriesRaw)
	if err != nil {
		return nil, err
	}
	queries = append(queries, parsedQueries...)

	return &Config{Queries: queries}, nil
}

func parseQueries(queriesRaw []byte) (queries []*QueryConfig, err error) {
	var current QueryConfig
	add := func() {
		q := &QueryConfig{
			Name:  strings.TrimSpace(current.Name),
			Query: strings.TrimSpace(current.Query),
		}
		current = QueryConfig{} // reset
		if q.Query == "" {
			return
		}
		if q.Name == "" {
			err = errors.Errorf("no name set for query %q", q.Query)
		}
		queries = append(queries, q)
	}
	for _, line := range bytes.Split(queriesRaw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if line[0] == '#' {
			add()
			current.Name = string(line[1:])
		} else {
			current.Query += " " + string(line)
		}
	}
	add()
	return queries, err
}

func loadAttributions() ([]*QueryConfig, error) {
	var qs []*QueryConfig
	err := fs.WalkDir(attributionFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		b, err := fs.ReadFile(attributionFS, path)
		if err != nil {
			return err
		}

		// Remove extension and prefix with attr_
		name := "attr_" + strings.Split(d.Name(), ".")[0]

		qs = append(qs, &QueryConfig{
			Snippet:   string(b),
			Name:      name,
			Protocols: []Protocol{Batch}, // only support batch for attribution
		})

		return nil
	})
	return qs, err
}
