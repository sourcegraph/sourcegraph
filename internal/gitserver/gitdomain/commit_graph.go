package gitdomain

import (
	"strings"
)

type CommitGraph struct {
	graph map[string][]string
	order []string
}

func (c *CommitGraph) Graph() map[string][]string { return c.graph }
func (c *CommitGraph) Order() []string            { return c.order }

// ParseCommitGraph converts the output of git log into a map from commits to
// parent commits, and a topological ordering of commits such that parents come
// before children. If a commit is listed but has no ancestors then its parent
// slice is empty, but is still present in the map and the ordering. If the
// ordering is to be correct, the git log output must be formatted with
// `git log --pretty="%H %P" --topo-order`.
func ParseCommitGraph(lines []string) *CommitGraph {
	// Process lines backwards so that we see all parents before children. We get a
	// topological ordering by simply scraping the keys off in this order.

	n := len(lines) - 1
	for i := range len(lines) / 2 {
		lines[i], lines[n-i] = lines[n-i], lines[i]
	}

	graph := make(map[string][]string, len(lines))
	order := make([]string, 0, len(lines))

	var prefix []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, " ")

		if len(parts) == 1 {
			graph[parts[0]] = []string{}
		} else {
			graph[parts[0]] = parts[1:]
		}

		order = append(order, parts[0])

		for _, part := range parts[1:] {
			if _, ok := graph[part]; !ok {
				graph[part] = []string{}
				prefix = append(prefix, part)
			}
		}
	}

	return &CommitGraph{
		graph: graph,
		order: append(prefix, order...),
	}
}
