package backend

import (
	"cmp"
	"slices"

	"github.com/grafana/regexp"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/zoekt"

	proto "github.com/sourcegraph/zoekt/cmd/zoekt-sourcegraph-indexserver/protos/sourcegraph/zoekt/configuration/v1"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/ctags_config"
	"github.com/sourcegraph/sourcegraph/schema"
)

// ZoektIndexOptions are options which change what we index for a
// repository. Everytime a repository is indexed by zoekt this structure is
// fetched. See getIndexOptions in the zoekt codebase.
//
// We only specify a subset of the fields from zoekt.IndexOptions.
type ZoektIndexOptions struct {
	// Name is the Repository Name.
	Name string

	// RepoID is the Sourcegraph Repository ID.
	RepoID api.RepoID

	// Public is true if the repository is public and does not require auth
	// filtering.
	Public bool

	// Fork is true if the repository is a fork.
	Fork bool

	// Archived is true if the repository is archived.
	Archived bool

	// LargeFiles is a slice of glob patterns where matching file paths should
	// be indexed regardless of their size. The pattern syntax can be found
	// here: https://golang.org/pkg/path/filepath/#Match.
	LargeFiles []string

	// Symbols if true will make zoekt index the output of ctags.
	Symbols bool

	// Branches is a slice of branches to index.
	Branches []zoekt.RepositoryBranch `json:",omitempty"`

	// Priority indicates ranking in results, higher first.
	Priority float64 `json:",omitempty"`

	// DocumentRanksVersion when non-empty will lead to indexing using offline
	// ranking. When the string changes this will also cause us to re-index
	// with new ranks.
	DocumentRanksVersion string `json:",omitempty"`

	// Error if non-empty indicates the request failed for the repo.
	Error string `json:",omitempty"`

	LanguageMap map[string]ctags_config.ParserType

	ShardConcurrency int32 `json:",omitempty"`
}

func (o *ZoektIndexOptions) FromProto(p *proto.ZoektIndexOptions) {
	o.Name = p.GetName()
	o.RepoID = api.RepoID(p.GetRepoId())
	o.Public = p.GetPublic()
	o.Fork = p.GetFork()
	o.Archived = p.GetArchived()
	o.LargeFiles = p.GetLargeFiles()
	o.Symbols = p.GetSymbols()
	o.Priority = p.GetPriority()
	o.DocumentRanksVersion = p.GetDocumentRanksVersion()
	o.Error = p.GetError()

	branches := make([]zoekt.RepositoryBranch, 0, len(p.GetBranches()))
	for _, b := range p.GetBranches() {
		branches = append(branches, zoekt.RepositoryBranch{
			Name:    b.GetName(),
			Version: b.GetVersion(),
		})
	}

	o.Branches = branches

	languageMap := make(map[string]ctags_config.ParserType)
	for _, entry := range p.GetLanguageMap() {
		languageMap[entry.Language] = uint8(entry.Ctags.Number())
	}
	o.LanguageMap = languageMap
	o.ShardConcurrency = p.GetShardConcurrency()
}

func (o *ZoektIndexOptions) ToProto() *proto.ZoektIndexOptions {
	branches := make([]*proto.ZoektRepositoryBranch, 0, len(o.Branches))
	for _, b := range o.Branches {
		branches = append(branches, &proto.ZoektRepositoryBranch{
			Name:    b.Name,
			Version: b.Version,
		})
	}

	languageMap := make([]*proto.LanguageMapping, 0)
	for language, engine := range o.LanguageMap {
		languageMap = append(languageMap, &proto.LanguageMapping{Language: language, Ctags: proto.CTagsParserType(engine)})
	}

	return &proto.ZoektIndexOptions{
		Name:                 o.Name,
		RepoId:               int32(o.RepoID),
		Public:               o.Public,
		Fork:                 o.Fork,
		Archived:             o.Archived,
		LargeFiles:           o.LargeFiles,
		Symbols:              o.Symbols,
		Branches:             branches,
		Priority:             o.Priority,
		DocumentRanksVersion: o.DocumentRanksVersion,
		Error:                o.Error,
		LanguageMap:          languageMap,
		ShardConcurrency:     o.ShardConcurrency,
	}
}

// RepoIndexOptions are the options used by GetIndexOptions for a specific
// repository.
type RepoIndexOptions struct {
	// Name is the Repository Name.
	Name string

	// RepoID is the Sourcegraph Repository ID.
	RepoID api.RepoID

	// Public is true if the repository is public and does not require auth
	// filtering.
	Public bool

	// Priority indicates ranking in results, higher first.
	Priority float64

	// DocumentRanksVersion when non-empty will lead to indexing using offline
	// ranking. When the string changes this will also cause us to re-index
	// with new ranks.
	DocumentRanksVersion string

	// Fork is true if the repository is a fork.
	Fork bool

	// Archived is true if the repository is archived.
	Archived bool

	// GetVersion is used to resolve revisions for a repo. If it fails, the
	// error is encoded in the body. If the revision is missing, an empty
	// string should be returned rather than an error.
	GetVersion func(branch string) (string, error)
}

type getRepoIndexOptsFn func(repoID api.RepoID) (*RepoIndexOptions, error)

// GetIndexOptions returns a json blob for consumption by
// sourcegraph-zoekt-indexserver. It is for repos based on site settings c.
func GetIndexOptions(
	logger log.Logger,
	c *schema.SiteConfiguration,
	getRepoIndexOptions getRepoIndexOptsFn,
	getSearchContextRevisions func(repoID api.RepoID) ([]string, error),
	repos ...api.RepoID,
) []ZoektIndexOptions {
	// Limit concurrency to 32 to avoid too many active network requests and
	// strain on gitserver (as ported from zoekt-sourcegraph-indexserver). In
	// the future we want a more intelligent global limit based on scale.
	sema := make(chan struct{}, 32)
	results := make([]ZoektIndexOptions, len(repos))
	getSiteConfigRevisions := siteConfigRevisionsRuleFunc(logger, c)

	for i := range repos {
		sema <- struct{}{}
		go func() {
			defer func() { <-sema }()
			results[i] = getIndexOptions(c, repos[i], getRepoIndexOptions, getSearchContextRevisions, getSiteConfigRevisions)
		}()
	}

	// Wait for jobs to finish (acquire full semaphore)
	for range cap(sema) {
		sema <- struct{}{}
	}

	return results
}

func getIndexOptions(
	c *schema.SiteConfiguration,
	repoID api.RepoID,
	getRepoIndexOptions func(repoID api.RepoID) (*RepoIndexOptions, error),
	getSearchContextRevisions func(repoID api.RepoID) ([]string, error),
	getSiteConfigRevisions revsRuleFunc,
) ZoektIndexOptions {
	opts, err := getRepoIndexOptions(repoID)
	if err != nil {
		return ZoektIndexOptions{
			RepoID: repoID,
			Error:  err.Error(),
		}
	}

	o := ZoektIndexOptions{
		Name:       opts.Name,
		RepoID:     opts.RepoID,
		Public:     opts.Public,
		Priority:   opts.Priority,
		Fork:       opts.Fork,
		Archived:   opts.Archived,
		LargeFiles: c.SearchLargeFiles,
		Symbols:    getBoolPtr(c.SearchIndexSymbolsEnabled, true),

		DocumentRanksVersion: opts.DocumentRanksVersion,
		LanguageMap:          ctags_config.CreateEngineMap(*c),
		ShardConcurrency:     int32(c.SearchIndexShardConcurrency),
	}

	// Set of branch names. Always index HEAD
	branches := map[string]struct{}{"HEAD": {}}

	// Add all branches that are referenced by search.index.branches and search.index.revisions.
	if getSiteConfigRevisions != nil {
		for _, rev := range getSiteConfigRevisions(opts) {
			branches[rev] = struct{}{}
		}
	}

	// Add all branches that are referenced by search contexts
	revs, err := getSearchContextRevisions(opts.RepoID)
	if err != nil {
		return ZoektIndexOptions{
			RepoID: opts.RepoID,
			Error:  err.Error(),
		}
	}
	for _, rev := range revs {
		branches[rev] = struct{}{}
	}

	// empty string means HEAD which is already in the set. Rather than
	// sanitize all inputs, just adjust the set before we start resolving.
	delete(branches, "")

	for branch := range branches {
		v, err := opts.GetVersion(branch)
		if err != nil {
			return ZoektIndexOptions{
				RepoID: opts.RepoID,
				Error:  err.Error(),
			}
		}

		// If we failed to resolve a branch, skip it
		if v == "" {
			continue
		}

		o.Branches = append(o.Branches, zoekt.RepositoryBranch{
			Name:    branch,
			Version: v,
		})
	}

	slices.SortFunc(o.Branches, func(a, b zoekt.RepositoryBranch) int {
		aName := a.Name
		bName := b.Name

		// Zoekt treats first branch as default branch, so put HEAD first. We
		// do this by making the string empty, which is a bottom.
		if aName == "HEAD" {
			aName = ""
		}
		if bName == "HEAD" {
			bName = ""
		}

		return cmp.Compare(aName, bName)
	})

	// If the first branch is not HEAD, do not index anything. This should
	// not happen, since HEAD should always exist if other branches exist.
	if len(o.Branches) == 0 || o.Branches[0].Name != "HEAD" {
		o.Branches = nil
	}

	// Zoekt has a limit of 64 branches
	if len(o.Branches) > 64 {
		o.Branches = o.Branches[:64]
	}

	return o
}

type revsRuleFunc func(*RepoIndexOptions) (revs []string)

func siteConfigRevisionsRuleFunc(logger log.Logger, c *schema.SiteConfiguration) revsRuleFunc {
	if c == nil || c.ExperimentalFeatures == nil {
		return nil
	}

	rules := make([]revsRuleFunc, 0, len(c.ExperimentalFeatures.SearchIndexRevisions))
	for _, rule := range c.ExperimentalFeatures.SearchIndexRevisions {
		rule := rule
		switch {
		case rule.Name != "":
			namePattern, err := regexp.Compile(rule.Name)
			if err != nil {
				logger.Error("error compiling regex from search.index.revisions", log.String("regex", rule.Name), log.Error(err))
				continue
			}

			rules = append(rules, func(o *RepoIndexOptions) []string {
				if !namePattern.MatchString(o.Name) {
					return nil
				}
				return rule.Revisions
			})
		}
	}

	return func(o *RepoIndexOptions) (matched []string) {
		cfg := c.ExperimentalFeatures

		if len(cfg.SearchIndexBranches) != 0 {
			matched = append(matched, cfg.SearchIndexBranches[o.Name]...)
		}

		for _, rule := range rules {
			matched = append(matched, rule(o)...)
		}

		return matched
	}
}

func getBoolPtr(b *bool, default_ bool) bool {
	if b == nil {
		return default_
	}
	return *b
}
