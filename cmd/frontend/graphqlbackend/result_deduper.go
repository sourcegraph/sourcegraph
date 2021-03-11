package graphqlbackend

type searchResultDeduper struct {
	seenFileMatches   map[string]*FileMatchResolver
	seenRepoMatches   map[string]*RepositoryResolver
	seenCommitMatches map[string]*CommitSearchResultResolver
	seenDiffMatches   map[string]*CommitSearchResultResolver
}

func NewDeduper() *searchResultDeduper {
	return &searchResultDeduper{
		seenFileMatches:   make(map[string]*FileMatchResolver),
		seenRepoMatches:   make(map[string]*RepositoryResolver),
		seenCommitMatches: make(map[string]*CommitSearchResultResolver),
		seenDiffMatches:   make(map[string]*CommitSearchResultResolver),
	}
}

// Add adds a SearchResultResolver to the deduper, merging it into
// a previously added SearchResultResolver if the URL has already been seen
func (d *searchResultDeduper) Add(r SearchResultResolver) {
	if fileMatch, ok := r.ToFileMatch(); ok {
		if prev, seen := d.seenFileMatches[fileMatch.URI]; seen {
			prev.appendMatches(fileMatch)
			return
		}
		d.seenFileMatches[fileMatch.URI] = fileMatch
		return
	}

	if repoMatch, ok := r.ToRepository(); ok {
		if _, seen := d.seenRepoMatches[repoMatch.URL()]; seen {
			return
		}
		d.seenRepoMatches[repoMatch.URL()] = repoMatch
		return
	}

	if commitMatch, ok := r.ToCommitSearchResult(); ok {
		if commitMatch.DiffPreview() != nil {
			if _, seen := d.seenDiffMatches[commitMatch.URL()]; seen {
				return
			}
			d.seenDiffMatches[commitMatch.URL()] = commitMatch
			return
		}

		if _, seen := d.seenCommitMatches[commitMatch.URL()]; seen {
			return
		}
		d.seenCommitMatches[commitMatch.URL()] = commitMatch
		return
	}
}

// Seen returns whether the given url exists for a file type in the deduper without
// modifying the contents of the deduper
func (d *searchResultDeduper) Seen(r SearchResultResolver) (ok bool) {
	switch v := r.(type) {
	case *FileMatchResolver:
		_, ok = d.seenFileMatches[v.URI]
	case *RepositoryResolver:
		_, ok = d.seenRepoMatches[v.URL()]
	case *CommitSearchResultResolver:
		if v.DiffPreview() != nil {
			_, ok = d.seenDiffMatches[v.URL()]
		} else {
			_, ok = d.seenCommitMatches[v.URL()]
		}
	}

	return ok
}

// Results returns a slice of SearchResultResolvers, deduplicated from
// the SearchResultResolvers that were added with Add
func (d *searchResultDeduper) Results() []SearchResultResolver {
	total := len(d.seenFileMatches) + len(d.seenRepoMatches) + len(d.seenCommitMatches) + len(d.seenDiffMatches)
	r := make([]SearchResultResolver, 0, total)
	for _, v := range d.seenFileMatches {
		r = append(r, v)
	}
	for _, v := range d.seenRepoMatches {
		r = append(r, v)
	}
	for _, v := range d.seenCommitMatches {
		r = append(r, v)
	}
	for _, v := range d.seenDiffMatches {
		r = append(r, v)
	}
	return r
}
