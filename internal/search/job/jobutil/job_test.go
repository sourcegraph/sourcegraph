package jobutil

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"regexp/syntax" //nolint:depguard // using the grafana fork of regexp clashes with zoekt, which uses the std regexp/syntax.
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/regexp"
	"github.com/hexops/autogold/v2"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	zoektquery "github.com/sourcegraph/zoekt/query"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/endpoint"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/search"
	searchbackend "github.com/sourcegraph/sourcegraph/internal/search/backend"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/job/printer"
	"github.com/sourcegraph/sourcegraph/internal/search/limits"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/searcher"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	zoektutil "github.com/sourcegraph/sourcegraph/internal/search/zoekt"
	"github.com/sourcegraph/sourcegraph/internal/searcher/protocol"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestNewPlanJob(t *testing.T) {
	cases := []struct {
		query      string
		protocol   search.Protocol
		searchType query.SearchType
		want       autogold.Value
	}{{
		query:      `foo context:@userA`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeLiteral,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . literal)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (SEQUENTIAL
            (ensureUnique . false)
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.searchContextSpec . @userA)
              (PARTIALREPOS
                (ZOEKTREPOSUBSETTEXTSEARCH
                  (fileMatchLimit . 10000)
                  (select . )
                  (zoektQueryRegexps . ["(?i)foo"])
                  (query . substr:"foo")
                  (type . text))))
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.searchContextSpec . @userA)
              (PARTIALREPOS
                (SEARCHERTEXTSEARCH
                  (useFullDeadline . true)
                  (patternInfo . TextPatternInfo{"foo",filematchlimit:10000})
                  (numRepos . 0)
                  (pathRegexps . ["(?i)foo"])
                  (indexed . false))))
            (REPOSEARCH
              (repoOpts.repoFilters . [foo])
              (repoOpts.searchContextSpec . @userA)
              (repoNamePatterns . ["(?i)foo"])))
          NOOP
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.searchContextSpec . @userA))
          NOOP)))))
`),
	}, {
		query:      `foo context:global`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeLiteral,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . literal)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (SEQUENTIAL
            (ensureUnique . false)
            (ZOEKTGLOBALTEXTSEARCH
              (fileMatchLimit . 10000)
              (select . )
              (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
              (includePrivate . true)
              (globalZoektQueryRegexps . ["(?i)foo"])
              (query . substr:"foo")
              (type . text)
              (repoOpts.searchContextSpec . global))
            (REPOSEARCH
              (repoOpts.repoFilters . [foo])
              (repoOpts.searchContextSpec . global)
              (repoNamePatterns . ["(?i)foo"])))
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.searchContextSpec . global))
          NOOP)))))
`),
	}, {
		query:      `foo`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeLiteral,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . literal)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (SEQUENTIAL
            (ensureUnique . false)
            (ZOEKTGLOBALTEXTSEARCH
              (fileMatchLimit . 10000)
              (select . )
              (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
              (includePrivate . true)
              (globalZoektQueryRegexps . ["(?i)foo"])
              (query . substr:"foo")
              (type . text))
            (REPOSEARCH
              (repoOpts.repoFilters . [foo])
              (repoNamePatterns . ["(?i)foo"])))
          REPOSCOMPUTEEXCLUDED
          NOOP)))))
`),
	}, {
		query:      `foo repo:sourcegraph/sourcegraph`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeLiteral,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . literal)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (SEQUENTIAL
            (ensureUnique . false)
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.repoFilters . [sourcegraph/sourcegraph])
              (PARTIALREPOS
                (ZOEKTREPOSUBSETTEXTSEARCH
                  (fileMatchLimit . 10000)
                  (select . )
                  (zoektQueryRegexps . ["(?i)foo"])
                  (query . substr:"foo")
                  (type . text))))
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.repoFilters . [sourcegraph/sourcegraph])
              (PARTIALREPOS
                (SEARCHERTEXTSEARCH
                  (useFullDeadline . true)
                  (patternInfo . TextPatternInfo{"foo",filematchlimit:10000})
                  (numRepos . 0)
                  (pathRegexps . ["(?i)foo"])
                  (indexed . false))))
            (REPOSEARCH
              (repoOpts.repoFilters . [sourcegraph/sourcegraph foo])
              (repoNamePatterns . ["(?i)sourcegraph/sourcegraph","(?i)foo"])))
          NOOP
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.repoFilters . [sourcegraph/sourcegraph]))
          NOOP)))))
`),
	}, {
		query:      `ok ok`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (SEQUENTIAL
            (ensureUnique . false)
            (ZOEKTGLOBALTEXTSEARCH
              (fileMatchLimit . 10000)
              (select . )
              (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
              (includePrivate . true)
              (globalZoektQueryRegexps . ["(?i)(?-s:ok.*?ok)"])
              (query . regex:"ok(?-s:.)*?ok")
              (type . text))
            (REPOSEARCH
              (repoOpts.repoFilters . [(?:ok).*?(?:ok)])
              (repoNamePatterns . ["(?i)(?:ok).*?(?:ok)"])))
          REPOSCOMPUTEEXCLUDED
          NOOP)))))
`),
	}, {
		query:      `ok @thing`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeLiteral,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . literal)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (SEQUENTIAL
            (ensureUnique . false)
            (ZOEKTGLOBALTEXTSEARCH
              (fileMatchLimit . 10000)
              (select . )
              (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
              (includePrivate . true)
              (globalZoektQueryRegexps . ["(?i)ok @thing"])
              (query . substr:"ok @thing")
              (type . text))
            (REPOSEARCH
              (repoOpts.repoFilters . [ok ])
              (repoNamePatterns . ["(?i)ok "])))
          REPOSCOMPUTEEXCLUDED
          NOOP)))))
`),
	}, {
		query:      `@nope`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (ZOEKTGLOBALTEXTSEARCH
            (fileMatchLimit . 10000)
            (select . )
            (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
            (includePrivate . true)
            (globalZoektQueryRegexps . ["(?i)@nope"])
            (query . substr:"@nope")
            (type . text))
          REPOSCOMPUTEEXCLUDED
          NOOP)))))
`),
	}, {
		query:      `foo @bar`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (ZOEKTGLOBALTEXTSEARCH
            (fileMatchLimit . 10000)
            (select . )
            (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
            (includePrivate . true)
            (globalZoektQueryRegexps . ["(?i)(?-s:foo.*?@bar)"])
            (query . regex:"foo(?-s:.)*?@bar")
            (type . text))
          REPOSCOMPUTEEXCLUDED
          NOOP)))))
`),
	}, {
		query:      `type:symbol test`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (ZOEKTGLOBALSYMBOLSEARCH
            (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
            (includePrivate . true)
            (fileMatchLimit . 10000)
            (select . )
            (query . sym:substr:"test")
            (type . symbol))
          REPOSCOMPUTEEXCLUDED
          NOOP)))))
`),
	}, {
		query:      `type:commit test`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (REPOPAGER
            (containsRefGlobs . false)
            (repoOpts.onlyCloned . true)
            (PARTIALREPOS
              (COMMITSEARCH
                (includeModifiedFiles . false)
                (query . *protocol.MessageMatches(test))
                (diff . false)
                (limit . 10000))))
          REPOSCOMPUTEEXCLUDED
          NOOP)))))
`),
	}, {
		query:      `type:diff test`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (REPOPAGER
            (containsRefGlobs . false)
            (repoOpts.onlyCloned . true)
            (PARTIALREPOS
              (DIFFSEARCH
                (includeModifiedFiles . false)
                (query . *protocol.DiffMatches(test))
                (diff . true)
                (limit . 10000))))
          REPOSCOMPUTEEXCLUDED
          NOOP)))))
`),
	}, {
		query:      `type:file type:commit test`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (ZOEKTGLOBALTEXTSEARCH
            (fileMatchLimit . 10000)
            (select . )
            (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
            (includePrivate . true)
            (globalZoektQueryRegexps . [])
            (query . content_substr:"test")
            (type . text))
          (REPOPAGER
            (containsRefGlobs . false)
            (repoOpts.onlyCloned . true)
            (PARTIALREPOS
              (COMMITSEARCH
                (includeModifiedFiles . false)
                (query . *protocol.MessageMatches(test))
                (diff . false)
                (limit . 10000))))
          REPOSCOMPUTEEXCLUDED
          NOOP)))))
`),
	}, {
		query:      `type:file type:path type:repo type:commit type:symbol repo:test test`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (SEQUENTIAL
            (ensureUnique . false)
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.repoFilters . [test])
              (PARTIALREPOS
                (ZOEKTREPOSUBSETTEXTSEARCH
                  (fileMatchLimit . 10000)
                  (select . )
                  (zoektQueryRegexps . ["(?i)test"])
                  (query . substr:"test")
                  (type . text))))
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.repoFilters . [test])
              (PARTIALREPOS
                (SEARCHERTEXTSEARCH
                  (useFullDeadline . true)
                  (patternInfo . TextPatternInfo{/test/,filematchlimit:10000})
                  (numRepos . 0)
                  (pathRegexps . ["(?i)test"])
                  (indexed . false))))
            (REPOSEARCH
              (repoOpts.repoFilters . [test test])
              (repoNamePatterns . ["(?i)test","(?i)test"])))
          NOOP
          (REPOPAGER
            (containsRefGlobs . false)
            (repoOpts.repoFilters . [test])
            (PARTIALREPOS
              (ZOEKTSYMBOLSEARCH
                (fileMatchLimit . 10000)
                (select . )
                (query . sym:substr:"test"))))
          (REPOPAGER
            (containsRefGlobs . false)
            (repoOpts.repoFilters . [test])
            (repoOpts.onlyCloned . true)
            (PARTIALREPOS
              (COMMITSEARCH
                (includeModifiedFiles . false)
                (query . *protocol.MessageMatches(test))
                (diff . false)
                (limit . 10000))))
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.repoFilters . [test]))
          (PARALLEL
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.repoFilters . [test])
              (PARTIALREPOS
                (SEARCHERSYMBOLSEARCH
                  (request.pattern . test)
                  (numRepos . 0)
                  (limit . 10000))))
            NOOP))))))
`),
	}, {
		query:      `type:file type:commit test`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (ZOEKTGLOBALTEXTSEARCH
            (fileMatchLimit . 10000)
            (select . )
            (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
            (includePrivate . true)
            (globalZoektQueryRegexps . [])
            (query . content_substr:"test")
            (type . text))
          (REPOPAGER
            (containsRefGlobs . false)
            (repoOpts.onlyCloned . true)
            (PARTIALREPOS
              (COMMITSEARCH
                (includeModifiedFiles . false)
                (query . *protocol.MessageMatches(test))
                (diff . false)
                (limit . 10000))))
          REPOSCOMPUTEEXCLUDED
          NOOP)))))
`),
	}, {
		query:      `type:file type:path type:repo type:commit type:symbol repo:test test`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (SEQUENTIAL
            (ensureUnique . false)
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.repoFilters . [test])
              (PARTIALREPOS
                (ZOEKTREPOSUBSETTEXTSEARCH
                  (fileMatchLimit . 10000)
                  (select . )
                  (zoektQueryRegexps . ["(?i)test"])
                  (query . substr:"test")
                  (type . text))))
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.repoFilters . [test])
              (PARTIALREPOS
                (SEARCHERTEXTSEARCH
                  (useFullDeadline . true)
                  (patternInfo . TextPatternInfo{/test/,filematchlimit:10000})
                  (numRepos . 0)
                  (pathRegexps . ["(?i)test"])
                  (indexed . false))))
            (REPOSEARCH
              (repoOpts.repoFilters . [test test])
              (repoNamePatterns . ["(?i)test","(?i)test"])))
          NOOP
          (REPOPAGER
            (containsRefGlobs . false)
            (repoOpts.repoFilters . [test])
            (PARTIALREPOS
              (ZOEKTSYMBOLSEARCH
                (fileMatchLimit . 10000)
                (select . )
                (query . sym:substr:"test"))))
          (REPOPAGER
            (containsRefGlobs . false)
            (repoOpts.repoFilters . [test])
            (repoOpts.onlyCloned . true)
            (PARTIALREPOS
              (COMMITSEARCH
                (includeModifiedFiles . false)
                (query . *protocol.MessageMatches(test))
                (diff . false)
                (limit . 10000))))
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.repoFilters . [test]))
          (PARALLEL
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.repoFilters . [test])
              (PARTIALREPOS
                (SEARCHERSYMBOLSEARCH
                  (request.pattern . test)
                  (numRepos . 0)
                  (limit . 10000))))
            NOOP))))))
`),
	}, {
		query:      `(type:commit or type:diff) (a or b)`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		// TODO this output doesn't look right. There shouldn't be any zoekt or repo jobs
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (OR
      (TIMEOUT
        (timeout . 20s)
        (LIMIT
          (limit . 10000)
          (PARALLEL
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.onlyCloned . true)
              (PARTIALREPOS
                (COMMITSEARCH
                  (includeModifiedFiles . false)
                  (query . (*protocol.MessageMatches((?:a)|(?:b))))
                  (diff . false)
                  (limit . 10000))))
            REPOSCOMPUTEEXCLUDED
            (OR
              NOOP
              NOOP))))
      (TIMEOUT
        (timeout . 20s)
        (LIMIT
          (limit . 10000)
          (PARALLEL
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.onlyCloned . true)
              (PARTIALREPOS
                (DIFFSEARCH
                  (includeModifiedFiles . false)
                  (query . (*protocol.DiffMatches((?:a)|(?:b))))
                  (diff . true)
                  (limit . 10000))))
            REPOSCOMPUTEEXCLUDED
            (OR
              NOOP
              NOOP)))))))
`),
	}, {
		query:      `(type:repo a) or (type:file b)`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (OR
      (TIMEOUT
        (timeout . 20s)
        (LIMIT
          (limit . 10000)
          (PARALLEL
            REPOSCOMPUTEEXCLUDED
            (REPOSEARCH
              (repoOpts.repoFilters . [a])
              (repoNamePatterns . ["(?i)a"])))))
      (TIMEOUT
        (timeout . 20s)
        (LIMIT
          (limit . 10000)
          (PARALLEL
            (ZOEKTGLOBALTEXTSEARCH
              (fileMatchLimit . 10000)
              (select . )
              (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
              (includePrivate . true)
              (globalZoektQueryRegexps . [])
              (query . content_substr:"b")
              (type . text))
            REPOSCOMPUTEEXCLUDED
            NOOP))))))
`),
	}, {
		query:      `type:symbol a or b`,
		protocol:   search.Streaming,
		searchType: query.SearchTypeRegex,
		want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (ZOEKTGLOBALSYMBOLSEARCH
            (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
            (includePrivate . true)
            (fileMatchLimit . 10000)
            (select . )
            (query . (or sym:substr:"a" sym:substr:"b"))
            (type . symbol))
          REPOSCOMPUTEEXCLUDED
          (OR
            NOOP
            NOOP))))))
`),
	},
		{
			query:      `repo:contains.path(a) repo:contains.content(b)`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeRegex,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.hasFileContent[0].path . a)
            (repoOpts.hasFileContent[1].content . b))
          (REPOSEARCH
            (repoOpts.hasFileContent[0].path . a)
            (repoOpts.hasFileContent[1].content . b)
            (repoNamePatterns . [])))))))
`),
		}, {
			query:      `file:contains.content(a.*b)`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeKeyword,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . keyword)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (FILECONTAINSFILTER
          (originalPatterns . [])
          (filterPatterns . ["(?i:a.*b)"])
          (PARALLEL
            (ZOEKTGLOBALTEXTSEARCH
              (fileMatchLimit . 10000)
              (select . )
              (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
              (includePrivate . true)
              (globalZoektQueryRegexps . ["(?i)(?-s:a.*b)"])
              (query . regex:"a(?-s:.)*b")
              (type . text))
            REPOSCOMPUTEEXCLUDED
            NOOP))))))
`),
		}, {
			query:      `repo:foo file:contains.content(a.*b) index:no`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeKeyword,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . keyword)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (FILECONTAINSFILTER
          (originalPatterns . [])
          (filterPatterns . ["(?i:a.*b)"])
          (PARALLEL
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.repoFilters . [foo])
              (repoOpts.useIndex . no)
              (PARTIALREPOS
                (ZOEKTREPOSUBSETTEXTSEARCH
                  (fileMatchLimit . 10000)
                  (select . )
                  (zoektQueryRegexps . ["(?i)(?-s:a.*b)"])
                  (query . regex:"a(?-s:.)*b")
                  (type . text))))
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.repoFilters . [foo])
              (repoOpts.useIndex . no)
              (PARTIALREPOS
                (SEARCHERTEXTSEARCH
                  (useFullDeadline . true)
                  (patternInfo . TextPatternInfo{(/a.*b/),filematchlimit:10000})
                  (numRepos . 0)
                  (pathRegexps . ["(?i)a.*b"])
                  (indexed . false))))
            (REPOSCOMPUTEEXCLUDED
              (repoOpts.repoFilters . [foo])
              (repoOpts.useIndex . no))
            NOOP))))))
`),
		}, {
			query:      `repo:contains.file(path:a content:b)`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeRegex,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.hasFileContent[0].path . a)
            (repoOpts.hasFileContent[0].content . b))
          (REPOSEARCH
            (repoOpts.hasFileContent[0].path . a)
            (repoOpts.hasFileContent[0].content . b)
            (repoNamePatterns . [])))))))
`),
		}, {
			query:      `repo:has(key:value)`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeRegex,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.hasKVPs[0].key . ^key$)
            (repoOpts.hasKVPs[0].value . ^value$))
          (REPOSEARCH
            (repoOpts.hasKVPs[0].key . ^key$)
            (repoOpts.hasKVPs[0].value . ^value$)
            (repoNamePatterns . [])))))))
`),
		}, {
			query:      `repo:has.tag(tag)`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeRegex,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.hasKVPs[0].key . ^tag$))
          (REPOSEARCH
            (repoOpts.hasKVPs[0].key . ^tag$)
            (repoNamePatterns . [])))))))
`),
		}, {
			query:      `repo:has.topic(mytopic)`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeRegex,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.hasTopics[0].topic . mytopic))
          (REPOSEARCH
            (repoOpts.hasTopics[0].topic . mytopic)
            (repoNamePatterns . [])))))))
`),
		}, {
			query:      `repo:has.tag(tag) foo`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeRegex,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . regex)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (SEQUENTIAL
            (ensureUnique . false)
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.hasKVPs[0].key . ^tag$)
              (PARTIALREPOS
                (ZOEKTREPOSUBSETTEXTSEARCH
                  (fileMatchLimit . 10000)
                  (select . )
                  (zoektQueryRegexps . ["(?i)foo"])
                  (query . substr:"foo")
                  (type . text))))
            (REPOPAGER
              (containsRefGlobs . false)
              (repoOpts.hasKVPs[0].key . ^tag$)
              (PARTIALREPOS
                (SEARCHERTEXTSEARCH
                  (useFullDeadline . true)
                  (patternInfo . TextPatternInfo{/foo/,filematchlimit:10000})
                  (numRepos . 0)
                  (pathRegexps . ["(?i)foo"])
                  (indexed . false))))
            (REPOSEARCH
              (repoOpts.repoFilters . [foo])
              (repoOpts.hasKVPs[0].key . ^tag$)
              (repoNamePatterns . ["(?i)foo"])))
          NOOP
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.hasKVPs[0].key . ^tag$))
          NOOP)))))
`),
		}, {
			query:      `(...)`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeStructural,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . structural)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          REPOSCOMPUTEEXCLUDED
          (REPOPAGER
            (containsRefGlobs . false)
            (PARTIALREPOS
              (STRUCTURALSEARCH
                (useFullDeadline . true)
                (useIndex . yes)
                (patternInfo.query . "(:[_])")
                (patternInfo.isStructural . true)
                (patternInfo.fileMatchLimit . 10000)))))))))
`),
		},
		{
			query:      `context:global repo:sourcegraph/.* are all things possible? lang:go`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeCodyContext,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . codycontext)
    NOOP))
`),
		},
		{
			query:      `context:global repo:sourcegraph/.* what is symf? lang:go`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeCodyContext,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . codycontext)
    (CODYCONTEXTSEARCH
      (patterns . ["readme","symf"])
      (codeCount . 12)
      (textCount . 3))))
`),
		},
		// The next query shows an unexpected way that a query is
		// translated into a global zoekt query, all depending on if context:
		// is specified (which it normally is). We expect to just have one
		// global zoekt query, but with context we do not. Recording this test
		// to capture the current inefficiency.
		{
			query:      `context:global (foo AND bar AND baz) OR "foo bar baz"`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeKeyword,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . keyword)
    (OR
      (TIMEOUT
        (timeout . 20s)
        (LIMIT
          (limit . 10000)
          (PARALLEL
            (ZOEKTGLOBALTEXTSEARCH
              (fileMatchLimit . 10000)
              (select . )
              (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
              (includePrivate . true)
              (globalZoektQueryRegexps . ["(?i)foo","(?i)bar","(?i)baz"])
              (query . (and substr:"foo" substr:"bar" substr:"baz"))
              (type . text)
              (repoOpts.searchContextSpec . global))
            (REPOSCOMPUTEEXCLUDED
              (repoOpts.searchContextSpec . global))
            (AND
              (LIMIT
                (limit . 40000)
                (REPOSEARCH
                  (repoOpts.repoFilters . [foo])
                  (repoOpts.searchContextSpec . global)
                  (repoNamePatterns . ["(?i)foo"])))
              (LIMIT
                (limit . 40000)
                (REPOSEARCH
                  (repoOpts.repoFilters . [bar])
                  (repoOpts.searchContextSpec . global)
                  (repoNamePatterns . ["(?i)bar"])))
              (LIMIT
                (limit . 40000)
                (REPOSEARCH
                  (repoOpts.repoFilters . [baz])
                  (repoOpts.searchContextSpec . global)
                  (repoNamePatterns . ["(?i)baz"])))))))
      (TIMEOUT
        (timeout . 20s)
        (LIMIT
          (limit . 10000)
          (PARALLEL
            (SEQUENTIAL
              (ensureUnique . false)
              (ZOEKTGLOBALTEXTSEARCH
                (fileMatchLimit . 10000)
                (select . )
                (repoScope . ["(and branch=\"HEAD\" rawConfig:RcOnlyPublic|RcNoForks|RcNoArchived)"])
                (includePrivate . true)
                (globalZoektQueryRegexps . ["(?i)foo bar baz"])
                (query . substr:"foo bar baz")
                (type . text))
              (REPOSEARCH
                (repoOpts.repoFilters . [foo bar baz])
                (repoNamePatterns . ["(?i)foo bar baz"])))
            REPOSCOMPUTEEXCLUDED
            NOOP))))))
`),
		},

		// This test shows that we can handle languages not in Linguist
		{
			query:      `context:global repo:sourcegraph/.* lang:magik`,
			protocol:   search.Streaming,
			searchType: query.SearchTypeKeyword,
			want: autogold.Expect(`
(LOG
  (ALERT
    (features . error decoding features)
    (protocol . Streaming)
    (onSourcegraphDotCom . true)
    (query . )
    (originalQuery . )
    (patternType . keyword)
    (TIMEOUT
      (timeout . 20s)
      (LIMIT
        (limit . 10000)
        (PARALLEL
          (REPOPAGER
            (containsRefGlobs . false)
            (repoOpts.repoFilters . [sourcegraph/.*])
            (repoOpts.searchContextSpec . global)
            (PARTIALREPOS
              (ZOEKTREPOSUBSETTEXTSEARCH
                (fileMatchLimit . 10000)
                (select . )
                (zoektQueryRegexps . ["(?i)(?im:\\.MAGIK$)"])
                (query . file_regex:"(?i:\\.MAGIK)(?m:$)")
                (type . text))))
          (REPOPAGER
            (containsRefGlobs . false)
            (repoOpts.repoFilters . [sourcegraph/.*])
            (repoOpts.searchContextSpec . global)
            (PARTIALREPOS
              (SEARCHERTEXTSEARCH
                (useFullDeadline . true)
                (patternInfo . TextPatternInfo{//,filematchlimit:10000,lang:magik,f:"(?i)\\.magik$"})
                (numRepos . 0)
                (pathRegexps . ["(?i)\\.magik$"])
                (indexed . false))))
          (REPOSCOMPUTEEXCLUDED
            (repoOpts.repoFilters . [sourcegraph/.*])
            (repoOpts.searchContextSpec . global))
          NOOP)))))
`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			plan, err := query.Pipeline(query.Init(tc.query, tc.searchType))
			require.NoError(t, err)

			inputs := &search.Inputs{
				UserSettings:        &schema.Settings{},
				PatternType:         tc.searchType,
				Protocol:            tc.protocol,
				Features:            &search.Features{},
				OnSourcegraphDotCom: true,
			}

			j, err := NewPlanJob(inputs, plan)
			require.NoError(t, err)

			tc.want.Equal(t, sPrintSexpMax(j))
		})
	}
}

func TestToEvaluateJob(t *testing.T) {
	test := func(input string, protocol search.Protocol) string {
		q, _ := query.ParseLiteral(input)
		inputs := &search.Inputs{
			UserSettings:        &schema.Settings{},
			PatternType:         query.SearchTypeLiteral,
			Protocol:            protocol,
			OnSourcegraphDotCom: true,
		}

		b, _ := query.ToBasicQuery(q)
		j, _ := toFlatJobs(inputs, b)
		return "\n" + printer.SexpPretty(j) + "\n"
	}

	autogold.Expect(`
(REPOSEARCH
  (repoOpts.repoFilters . [foo])
  (repoNamePatterns . ["(?i)foo"]))
`).Equal(t, test("foo", search.Streaming))

	autogold.Expect(`
(REPOSEARCH
  (repoOpts.repoFilters . [foo])
  (repoNamePatterns . ["(?i)foo"]))
`).Equal(t, test("foo", search.Batch))
}

func TestToTextPatternInfo(t *testing.T) {
	cases := []struct {
		input  string
		output autogold.Value
		feat   search.Features
	}{{
		input:  `type:repo archived`,
		output: autogold.Expect(`{"Query":{"Value":"archived","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":false,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `type:repo archived archived:yes`,
		output: autogold.Expect(`{"Query":{"Value":"archived","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":false,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `type:repo sgtest/mux`,
		output: autogold.Expect(`{"Query":{"Value":"sgtest/mux","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":false,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `type:repo sgtest/mux fork:yes`,
		output: autogold.Expect(`{"Query":{"Value":"sgtest/mux","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":false,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `"func main() {\n" patterntype:regexp type:file`,
		output: autogold.Expect(`{"Query":{"Value":"func main() {\n","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `"func main() {\n" -repo:go-diff patterntype:regexp type:file`,
		output: autogold.Expect(`{"Query":{"Value":"func main() {\n","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/go-diff$ String case:yes type:file`,
		output: autogold.Expect(`{"Query":{"Value":"String","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":true,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":true,"PatternMatchesContent":true,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/java-langserver$@v1 void sendPartialResult(Object requestId, JsonPatch jsonPatch); patterntype:literal type:file`,
		output: autogold.Expect(`{"Query":{"Value":"void sendPartialResult(Object requestId, JsonPatch jsonPatch);","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/java-langserver$@v1 void sendPartialResult(Object requestId, JsonPatch jsonPatch); patterntype:literal count:1 type:file`,
		output: autogold.Expect(`{"Query":{"Value":"void sendPartialResult(Object requestId, JsonPatch jsonPatch);","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":1,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/java-langserver$ \nimport index:only patterntype:regexp type:file`,
		output: autogold.Expect(`{"Query":{"Value":"\\nimport","IsNegated":false,"IsRegExp":true,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"only","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/java-langserver$ \nimport index:no patterntype:regexp type:file`,
		output: autogold.Expect(`{"Query":{"Value":"\\nimport","IsNegated":false,"IsRegExp":true,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"no","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/java-langserver$ doesnot734734743734743exist`,
		output: autogold.Expect(`{"Query":{"Value":"doesnot734734743734743exist","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/sourcegraph-typescript$ type:commit test`,
		output: autogold.Expect(`{"Query":{"Value":"test","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":false,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/go-diff$ type:diff main`,
		output: autogold.Expect(`{"Query":{"Value":"main","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":false,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/go-diff$ repohascommitafter:"2019-01-01" test patterntype:literal`,
		output: autogold.Expect(`{"Query":{"Value":"test","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `^func.*$ patterntype:regexp index:only type:file`,
		output: autogold.Expect(`{"Query":{"Value":"^func.*$","IsNegated":false,"IsRegExp":true,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"only","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `fork:only patterntype:regexp FORK_SENTINEL`,
		output: autogold.Expect(`{"Query":{"Value":"FORK_SENTINEL","IsNegated":false,"IsRegExp":true,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `\bfunc\b lang:go type:file patterntype:regexp`,
		output: autogold.Expect(`{"Query":{"Value":"\\bfunc\\b","IsNegated":false,"IsRegExp":true,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":["(?i)\\.go$"],"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":false,"Languages":["go"]}`),
	}, {
		input:  `no results for { ... } raises alert repo:^github\.com/sgtest/go-diff$`,
		output: autogold.Expect(`{"Query":{"Value":"no results for { ... } raises alert","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/go-diff$ patternType:regexp \ and /`,
		output: autogold.Expect(`{"Query":{"Value":"(?:\\ and).*?(?:/)","IsNegated":false,"IsRegExp":true,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/go-diff$ (not .svg) patterntype:literal`,
		output: autogold.Expect(`{"Query":{"Value":".svg","IsNegated":true,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/sourcegraph-typescript$ (Fetches OR file:language-server.ts)`,
		output: autogold.Expect(`{"Query":{"Value":"Fetches","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/sourcegraph-typescript$ ((file:^renovate\.json extends) or file:progress.ts createProgressProvider)`,
		output: autogold.Expect(`{"Query":{"Value":"extends","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":["^renovate\\.json"],"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/sourcegraph-typescript$ (type:diff or type:commit) author:felix yarn`,
		output: autogold.Expect(`{"Query":{"Value":"yarn","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":false,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/sourcegraph-typescript$ (type:diff or type:commit) subscription after:"june 11 2019" before:"june 13 2019"`,
		output: autogold.Expect(`{"Query":{"Value":"subscription","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":false,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `(repo:^github\.com/sgtest/go-diff$@garo/lsif-indexing-campaign:test-already-exist-pr or repo:^github\.com/sgtest/sourcegraph-typescript$) file:README.md #`,
		output: autogold.Expect(`{"Query":{"Value":"#","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":["README.md"],"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `(repo:^github\.com/sgtest/sourcegraph-typescript$ or repo:^github\.com/sgtest/go-diff$) package diff provides`,
		output: autogold.Expect(`{"Query":{"Value":"package diff provides","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:contains.file(path:noexist.go) test`,
		output: autogold.Expect(`{"Query":{"Value":"test","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:contains.file(path:go.mod) count:100 fmt`,
		output: autogold.Expect(`{"Query":{"Value":"fmt","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":100,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `type:commit LSIF`,
		output: autogold.Expect(`{"Query":{"Value":"LSIF","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":false,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `repo:contains.file(path:diff.pb.go) type:commit LSIF`,
		output: autogold.Expect(`{"Query":{"Value":"LSIF","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":false,"PatternMatchesPath":false,"Languages":null}`),
	}, {
		input:  `repo:go-diff patterntype:literal HunkNoChunksize select:repo`,
		output: autogold.Expect(`{"Query":{"Value":"HunkNoChunksize","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":["repo"],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:go-diff patterntype:literal HunkNoChunksize select:file`,
		output: autogold.Expect(`{"Query":{"Value":"HunkNoChunksize","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":["file"],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:go-diff patterntype:literal HunkNoChunksize select:content`,
		output: autogold.Expect(`{"Query":{"Value":"HunkNoChunksize","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":["content"],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:go-diff patterntype:literal HunkNoChunksize`,
		output: autogold.Expect(`{"Query":{"Value":"HunkNoChunksize","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:go-diff patterntype:literal HunkNoChunksize select:commit`,
		output: autogold.Expect(`{"Query":{"Value":"HunkNoChunksize","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":["commit"],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `foo\d "bar*" patterntype:regexp`,
		output: autogold.Expect(`{"Query":{"Value":"(?:foo\\d).*?(?:bar\\*)","IsNegated":false,"IsRegExp":true,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `patterntype:regexp // literal slash`,
		output: autogold.Expect(`{"Query":{"Value":"(?://).*?(?:literal).*?(?:slash)","IsNegated":false,"IsRegExp":true,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:contains.path(Dockerfile)`,
		output: autogold.Expect("Error"),
	}, {
		input:  `repohasfile:Dockerfile`,
		output: autogold.Expect("Error"),
	}, {
		input:  `repo:^github\.com/sgtest/go-diff$ make(:[1]) index:only patterntype:structural count:3`,
		output: autogold.Expect(`{"Query":{"Value":"make(:[1])","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":true,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":3,"Index":"only","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/go-diff$ make(:[1]) lang:go rule:'where "backcompat" == "backcompat"' patterntype:structural`,
		output: autogold.Expect(`{"Query":{"Value":"make(:[1])","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":true,"CombyRule":"where \"backcompat\" == \"backcompat\"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":["(?i)\\.go$"],"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":["go"]}`),
	}, {
		input:  `repo:^github\.com/sgtest/go-diff$@adde71 make(:[1]) index:no patterntype:structural count:3`,
		output: autogold.Expect(`{"Query":{"Value":"make(:[1])","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":true,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":3,"Index":"no","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/sourcegraph-typescript$ file:^README\.md "basic :[_] access :[_]" patterntype:structural`,
		output: autogold.Expect(`{"Query":{"Value":"\"basic :[_] access :[_]\"","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":true,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":["^README\\.md"],"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":null}`),
	}, {
		input:  `repo:^github\.com/sgtest/sourcegraph-typescript$ lang:typescript "basic :[_] access :[_]" patterntype:structural`,
		feat:   search.Features{ContentBasedLangFilters: true},
		output: autogold.Expect(`{"Query":{"Value":"\"basic :[_] access :[_]\"","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":true,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":["TypeScript"],"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":true,"Languages":["typescript"]}`),
	}, {
		input:  `sgtest lang:magik type:file`,
		feat:   search.Features{ContentBasedLangFilters: true},
		output: autogold.Expect(`{"Query":{"Value":"sgtest","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":null,"ExcludePaths":"","IncludeLangs":["Magik"],"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":false,"Languages":["magik"]}`),
	}, {
		input:  `sgtest lang:magik type:file`,
		feat:   search.Features{ContentBasedLangFilters: false},
		output: autogold.Expect(`{"Query":{"Value":"sgtest","IsNegated":false,"IsRegExp":false,"Boost":false},"IsStructuralPat":false,"CombyRule":"","IsCaseSensitive":false,"FileMatchLimit":30,"Index":"yes","Select":[],"IncludePaths":["(?i)\\.magik$"],"ExcludePaths":"","IncludeLangs":null,"ExcludeLangs":null,"PathPatternsAreCaseSensitive":false,"PatternMatchesContent":true,"PatternMatchesPath":false,"Languages":["magik"]}`),
	}}

	test := func(input string, feat search.Features) string {
		searchType := overrideSearchType(input, query.SearchTypeLiteral)
		plan, err := query.Pipeline(query.Init(input, searchType))
		if err != nil {
			return "Error"
		}
		if len(plan) == 0 {
			return "Empty"
		}
		b := plan[0]
		resultTypes := computeResultTypes(b, query.SearchTypeLiteral, defaultResultTypes)
		p, err := toTextPatternInfo(b, resultTypes, &feat, limits.DefaultMaxSearchResults)
		if err != nil {
			return "Error"
		}
		v, _ := json.Marshal(p)
		return string(v)
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			tc.output.Equal(t, test(tc.input, tc.feat))
		})
	}
}

func TestToTextPatternInfoBoost(t *testing.T) {
	basic := query.Basic{Pattern: query.Operator{
		Kind: query.And,
		Operands: []query.Node{
			query.Pattern{
				Value:      "lorem ipsum",
				Annotation: query.Annotation{Labels: query.Boost | query.Literal},
			},
			query.Pattern{
				Value:      "dolor sit",
				Annotation: query.Annotation{Labels: query.Literal},
			}},
	}}

	patternInfo, err := toTextPatternInfo(basic, defaultResultTypes, &search.Features{}, limits.DefaultMaxSearchResults)
	require.NoError(t, err)

	want := &protocol.AndNode{
		Children: []protocol.QueryNode{
			&protocol.PatternNode{Value: "lorem ipsum", Boost: true},
			&protocol.PatternNode{Value: "dolor sit"},
		},
	}

	if cmp.Diff(want, patternInfo.Query) != "" {
		t.Fatalf("unexpected query: %v", patternInfo.Query)
	}
}

func TestToSymbolSearchRequest(t *testing.T) {
	cases := []struct {
		input   string
		output  autogold.Value
		feat    search.Features
		wantErr bool
	}{{
		input:  `repo:go-diff patterntype:literal HunkNoChunksize select:symbol  file:^README\.md `,
		output: autogold.Expect(`{"RegexpPattern":"HunkNoChunksize","IsCaseSensitive":false,"IncludePatterns":["^README\\.md"],"ExcludePattern":"","IncludeLangs":null,"ExcludeLangs":null}`),
	}, {
		input:  `repo:go-diff patterntype:literal type:symbol HunkNoChunksize select:symbol -file:^README\.md `,
		output: autogold.Expect(`{"RegexpPattern":"HunkNoChunksize","IsCaseSensitive":false,"IncludePatterns":null,"ExcludePattern":"^README\\.md","IncludeLangs":null,"ExcludeLangs":null}`),
	}, {
		input:  `repo:go-diff type:symbol`,
		output: autogold.Expect(`{"RegexpPattern":"","IsCaseSensitive":false,"IncludePatterns":null,"ExcludePattern":"","IncludeLangs":null,"ExcludeLangs":null}`),
	}, {
		input:   `type:symbol NOT option`,
		output:  autogold.Expect("null"),
		wantErr: true,
	}, {
		input:  `repo:go-diff type:symbol HunkNoChunksize lang:Julia -lang:R`,
		output: autogold.Expect(`{"RegexpPattern":"HunkNoChunksize","IsCaseSensitive":false,"IncludePatterns":["(?i)\\.jl$"],"ExcludePattern":"(?i)(?:\\.r$)|(?:\\.rd$)|(?:\\.rsx$)|(?:(^|/)\\.Rprofile$)|(?:(^|/)expr-dist$)","IncludeLangs":null,"ExcludeLangs":null}`),
	}, {
		input:  `repo:go-diff type:symbol HunkNoChunksize lang:Julia -lang:R`,
		feat:   search.Features{ContentBasedLangFilters: true},
		output: autogold.Expect(`{"RegexpPattern":"HunkNoChunksize","IsCaseSensitive":false,"IncludePatterns":null,"ExcludePattern":"","IncludeLangs":["Julia"],"ExcludeLangs":["R"]}`),
	}}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			plan, err := query.Pipeline(query.Init(tc.input, query.SearchTypeLiteral))
			if err != nil {
				t.Fatal(err)
			}

			b := plan[0]
			var pattern *query.Pattern
			if p, ok := b.Pattern.(query.Pattern); ok {
				pattern = &p
			}

			f := query.Flat{Parameters: b.Parameters, Pattern: pattern}
			r, err := toSymbolSearchRequest(f, &tc.feat)

			if (err != nil) != tc.wantErr {
				t.Fatalf("mismatch error = %v, wantErr %v", err, tc.wantErr)
			}
			v, _ := json.Marshal(r)
			tc.output.Equal(t, string(v))
		})
	}
}

func overrideSearchType(input string, searchType query.SearchType) query.SearchType {
	q, err := query.Parse(input, query.SearchTypeLiteral)
	q = query.LowercaseFieldNames(q)
	if err != nil {
		// If parsing fails, return the default search type. Any actual
		// parse errors will be raised by subsequent parser calls.
		return searchType
	}
	query.VisitField(q, "patterntype", func(value string, _ bool, _ query.Annotation) {
		switch value {
		case "regex", "regexp":
			searchType = query.SearchTypeRegex
		case "literal":
			searchType = query.SearchTypeLiteral
		case "structural":
			searchType = query.SearchTypeStructural
		}
	})
	return searchType
}

func Test_computeResultTypes(t *testing.T) {
	test := func(input string, searchType query.SearchType) string {
		plan, _ := query.Pipeline(query.Init(input, searchType))
		b := plan[0]
		resultTypes := computeResultTypes(b, searchType, defaultResultTypes)
		return resultTypes.String()
	}

	t.Run("standard, only search file content when type not set", func(t *testing.T) {
		autogold.Expect("file").Equal(t, test("path:foo content:bar", query.SearchTypeStandard))
	})

	t.Run("standard, plain pattern searches repo path file content", func(t *testing.T) {
		autogold.Expect("file, path, repo").Equal(t, test("path:foo bar", query.SearchTypeStandard))
	})

	t.Run("keyword, only search file content when type not set", func(t *testing.T) {
		autogold.Expect("file").Equal(t, test("path:foo content:bar", query.SearchTypeKeyword))
	})

	t.Run("keyword, plain pattern searches repo path file content", func(t *testing.T) {
		autogold.Expect("file, path, repo").Equal(t, test("path:foo bar", query.SearchTypeKeyword))
	})

	t.Run("keyword, only search file content with negation", func(t *testing.T) {
		autogold.Expect("file").Equal(t, test("path:foo content:bar -content:baz", query.SearchTypeKeyword))
	})
}

func TestRepoSubsetTextSearch(t *testing.T) {
	searcher.MockSearchFilesInRepo = func(ctx context.Context, repo types.MinimalRepo, gitserverRepo api.RepoName, rev string, info *search.TextPatternInfo, fetchTimeout time.Duration, stream streaming.Sender) (limitHit bool, err error) {
		repoName := repo.Name
		switch repoName {
		case "foo/one":
			stream.Send(streaming.SearchEvent{
				Results: []result.Match{&result.FileMatch{
					File: result.File{
						Repo:     repo,
						InputRev: &rev,
						Path:     "main.go",
					},
				}},
			})
			return false, nil
		case "foo/two":
			stream.Send(streaming.SearchEvent{
				Results: []result.Match{&result.FileMatch{
					File: result.File{
						Repo:     repo,
						InputRev: &rev,
						Path:     "main.go",
					},
				}},
			})
			return false, nil
		case "foo/empty":
			return false, nil
		case "foo/cloning":
			return false, &gitdomain.RepoNotExistError{Repo: repoName, CloneInProgress: true}
		case "foo/missing":
			return false, &gitdomain.RepoNotExistError{Repo: repoName}
		case "foo/missing-database":
			return false, &errcode.Mock{Message: "repo not found: foo/missing-database", IsNotFound: true}
		case "foo/timedout":
			return false, context.DeadlineExceeded
		case "foo/no-rev":
			// TODO we do not specify a rev when searching "foo/no-rev", so it
			// is treated as an empty repository. We need to test the fatal
			// case of trying to search a revision which doesn't exist.
			return false, &gitdomain.RevisionNotFoundError{Repo: repoName, Spec: "missing"}
		default:
			return false, errors.New("Unexpected repo")
		}
	}
	defer func() { searcher.MockSearchFilesInRepo = nil }()

	zoekt := &searchbackend.FakeStreamer{}

	q, err := query.ParseLiteral("foo")
	if err != nil {
		t.Fatal(err)
	}
	repoRevs := makeRepositoryRevisions("foo/one", "foo/two", "foo/empty", "foo/cloning", "foo/missing", "foo/missing-database", "foo/timedout", "foo/no-rev")

	patternInfo := &search.TextPatternInfo{
		FileMatchLimit: limits.DefaultMaxSearchResults,
		Query: &protocol.PatternNode{
			Value: "foo",
		},
	}

	matches, common, err := runRepoSubsetTextSearch(
		context.Background(),
		logtest.Scoped(t),
		patternInfo,
		repoRevs,
		q,
		zoekt,
		endpoint.Static("test"),
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Errorf("expected two results, got %d", len(matches))
	}
	repoNames := map[api.RepoID]string{}
	for _, rr := range repoRevs {
		repoNames[rr.Repo.ID] = string(rr.Repo.Name)
	}
	assertReposStatus(t, repoNames, common.Status, map[string]search.RepoStatus{
		"foo/cloning":          search.RepoStatusCloning,
		"foo/missing":          search.RepoStatusMissing,
		"foo/missing-database": search.RepoStatusMissing,
		"foo/timedout":         search.RepoStatusTimedOut,
	})

	// If we specify a rev and it isn't found, we fail the whole search since
	// that should be checked earlier.
	_, _, err = runRepoSubsetTextSearch(
		context.Background(),
		logtest.Scoped(t),
		patternInfo,
		makeRepositoryRevisions("foo/no-rev@dev"),
		q,
		zoekt,
		endpoint.Static("test"),
		false,
	)
	if !errors.HasType[*gitdomain.RevisionNotFoundError](err) {
		t.Fatalf("searching non-existent rev expected to fail with RevisionNotFoundError got: %v", err)
	}
}

func TestSearchFilesInReposStream(t *testing.T) {
	searcher.MockSearchFilesInRepo = func(ctx context.Context, repo types.MinimalRepo, gitserverRepo api.RepoName, rev string, info *search.TextPatternInfo, fetchTimeout time.Duration, stream streaming.Sender) (limitHit bool, err error) {
		repoName := repo.Name
		switch repoName {
		case "foo/one":
			stream.Send(streaming.SearchEvent{
				Results: []result.Match{&result.FileMatch{
					File: result.File{
						Repo:     repo,
						InputRev: &rev,
						Path:     "main.go",
					},
				}},
			})
			return false, nil
		case "foo/two":
			stream.Send(streaming.SearchEvent{
				Results: []result.Match{&result.FileMatch{
					File: result.File{
						Repo:     repo,
						InputRev: &rev,
						Path:     "main.go",
					},
				}},
			})
			return false, nil
		case "foo/three":
			stream.Send(streaming.SearchEvent{
				Results: []result.Match{&result.FileMatch{
					File: result.File{
						Repo:     repo,
						InputRev: &rev,
						Path:     "main.go",
					},
				}},
			})
			return false, nil
		default:
			return false, errors.New("Unexpected repo")
		}
	}
	defer func() { searcher.MockSearchFilesInRepo = nil }()

	zoekt := &searchbackend.FakeStreamer{}

	q, err := query.ParseLiteral("foo")
	if err != nil {
		t.Fatal(err)
	}

	patternInfo := &search.TextPatternInfo{
		FileMatchLimit: limits.DefaultMaxSearchResults,
		Query: &protocol.PatternNode{
			Value: "foo",
		},
	}

	matches, _, err := runRepoSubsetTextSearch(
		context.Background(),
		logtest.Scoped(t),
		patternInfo,
		makeRepositoryRevisions("foo/one", "foo/two", "foo/three"),
		q,
		zoekt,
		endpoint.Static("test"),
		false,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(matches) != 3 {
		t.Errorf("expected three results, got %d", len(matches))
	}
}

func assertReposStatus(t *testing.T, repoNames map[api.RepoID]string, got search.RepoStatusMap, want map[string]search.RepoStatus) {
	t.Helper()
	gotM := map[string]search.RepoStatus{}
	got.Iterate(func(id api.RepoID, mask search.RepoStatus) {
		name := repoNames[id]
		if name == "" {
			name = fmt.Sprintf("UNKNOWNREPO{ID=%d}", id)
		}
		gotM[name] = mask
	})
	if diff := cmp.Diff(want, gotM); diff != "" {
		t.Errorf("RepoStatusMap mismatch (-want +got):\n%s", diff)
	}
}

func TestSearchFilesInRepos_multipleRevsPerRepo(t *testing.T) {
	searcher.MockSearchFilesInRepo = func(ctx context.Context, repo types.MinimalRepo, gitserverRepo api.RepoName, rev string, info *search.TextPatternInfo, fetchTimeout time.Duration, stream streaming.Sender) (limitHit bool, err error) {
		repoName := repo.Name
		switch repoName {
		case "foo":
			stream.Send(streaming.SearchEvent{
				Results: []result.Match{&result.FileMatch{
					File: result.File{
						Repo:     repo,
						CommitID: api.CommitID(rev),
						Path:     "main.go",
					},
				}},
			})
			return false, nil
		default:
			panic("unexpected repo")
		}
	}
	defer func() { searcher.MockSearchFilesInRepo = nil }()

	zoekt := &searchbackend.FakeStreamer{}

	q, err := query.ParseLiteral("foo")
	if err != nil {
		t.Fatal(err)
	}

	patternInfo := &search.TextPatternInfo{
		FileMatchLimit: limits.DefaultMaxSearchResults,
		Query: &protocol.PatternNode{
			Value: "foo",
		},
	}

	repos := makeRepositoryRevisions("foo@master:mybranch:branch3:branch4")

	matches, _, err := runRepoSubsetTextSearch(
		context.Background(),
		logtest.Scoped(t),
		patternInfo,
		repos,
		q,
		zoekt,
		endpoint.Static("test"),
		false,
	)
	if err != nil {
		t.Fatal(err)
	}

	matchKeys := make([]result.Key, len(matches))
	for i, match := range matches {
		matchKeys[i] = match.Key()
	}
	slices.SortFunc(matchKeys, result.Key.Compare)

	wantResultKeys := []result.Key{
		{Repo: "foo", Commit: "branch3", Path: "main.go"},
		{Repo: "foo", Commit: "branch4", Path: "main.go"},
		{Repo: "foo", Commit: "master", Path: "main.go"},
		{Repo: "foo", Commit: "mybranch", Path: "main.go"},
	}
	require.Equal(t, wantResultKeys, matchKeys)
}

func TestZoektQueryPatternsAsRegexps(t *testing.T) {
	tests := []struct {
		name  string
		input zoektquery.Q
		want  []*regexp.Regexp
	}{
		{
			name:  "literal substring query",
			input: &zoektquery.Substring{Pattern: "foobar"},
			want:  []*regexp.Regexp{regexp.MustCompile(`(?i)foobar`)},
		},
		{
			name:  "regex query",
			input: &zoektquery.Regexp{Regexp: &syntax.Regexp{Op: syntax.OpLiteral, Name: "foobar"}},
			want:  []*regexp.Regexp{regexp.MustCompile(`(?i)` + zoektquery.Regexp{Regexp: &syntax.Regexp{Op: syntax.OpLiteral, Name: "foobar"}}.Regexp.String())},
		},
		{
			name: "and query",
			input: zoektquery.NewAnd([]zoektquery.Q{
				&zoektquery.Substring{Pattern: "foobar"},
				&zoektquery.Substring{Pattern: "baz"},
			}...),
			want: []*regexp.Regexp{
				regexp.MustCompile(`(?i)foobar`),
				regexp.MustCompile(`(?i)baz`),
			},
		},
		{
			name: "or query",
			input: zoektquery.NewOr([]zoektquery.Q{
				&zoektquery.Substring{Pattern: "foobar"},
				&zoektquery.Substring{Pattern: "baz"},
			}...),
			want: []*regexp.Regexp{
				regexp.MustCompile(`(?i)foobar`),
				regexp.MustCompile(`(?i)baz`),
			},
		},
		{
			name: "literal and regex",
			input: zoektquery.NewAnd([]zoektquery.Q{
				&zoektquery.Substring{Pattern: "foobar"},
				&zoektquery.Regexp{Regexp: &syntax.Regexp{Op: syntax.OpLiteral, Name: "python"}},
			}...),
			want: []*regexp.Regexp{
				regexp.MustCompile(`(?i)foobar`),
				regexp.MustCompile(`(?i)` + zoektquery.Regexp{Regexp: &syntax.Regexp{Op: syntax.OpLiteral, Name: "python"}}.Regexp.String()),
			},
		},
		{
			name: "literal or regex",
			input: zoektquery.NewOr([]zoektquery.Q{
				&zoektquery.Substring{Pattern: "foobar"},
				&zoektquery.Regexp{Regexp: &syntax.Regexp{Op: syntax.OpLiteral, Name: "python"}},
			}...),
			want: []*regexp.Regexp{
				regexp.MustCompile(`(?i)foobar`),
				regexp.MustCompile(`(?i)` + zoektquery.Regexp{Regexp: &syntax.Regexp{Op: syntax.OpLiteral, Name: "python"}}.Regexp.String()),
			},
		},
		{
			name:  "respect case sensitivity setting",
			input: &zoektquery.Substring{Pattern: "foo", CaseSensitive: true},
			want:  []*regexp.Regexp{regexp.MustCompile(regexp.QuoteMeta("foo"))},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := zoektQueryPatternsAsRegexps(tc.input)
			require.Equal(t, tc.want, got)
		})
	}
}

func makeRepositoryRevisions(repos ...string) []*search.RepositoryRevisions {
	r := make([]*search.RepositoryRevisions, len(repos))
	for i, repospec := range repos {
		repoRevs, err := query.ParseRepositoryRevisions(repospec)
		if err != nil {
			panic(errors.Errorf("unexpected error parsing repo spec %s", repospec))
		}

		revs := make([]string, 0, len(repoRevs.Revs))
		for _, revSpec := range repoRevs.Revs {
			revs = append(revs, revSpec.RevSpec)
		}
		if len(revs) == 0 {
			// treat empty list as HEAD
			revs = []string{""}
		}
		r[i] = &search.RepositoryRevisions{Repo: mkRepos(repoRevs.Repo)[0], Revs: revs}
	}
	return r
}

func mkRepos(names ...string) []types.MinimalRepo {
	var repos []types.MinimalRepo
	for _, name := range names {
		sum := md5.Sum([]byte(name))
		id := api.RepoID(binary.BigEndian.Uint64(sum[:]))
		if id < 0 {
			id = -(id / 2)
		}
		if id == 0 {
			id++
		}
		repos = append(repos, types.MinimalRepo{ID: id, Name: api.RepoName(name)})
	}
	return repos
}

// runRepoSubsetTextSearch is a convenience function that simulates the RepoSubsetTextSearch job.
func runRepoSubsetTextSearch(
	ctx context.Context,
	logger log.Logger,
	patternInfo *search.TextPatternInfo,
	repos []*search.RepositoryRevisions,
	q query.Q,
	zoekt *searchbackend.FakeStreamer,
	searcherURLs *endpoint.Map,
	useFullDeadline bool,
) ([]*result.FileMatch, streaming.Stats, error) {
	searcherArgs := &search.SearcherParameters{
		PatternInfo:     patternInfo,
		UseFullDeadline: useFullDeadline,
	}

	agg := streaming.NewAggregatingStream()

	indexed, unindexed, err := zoektutil.PartitionRepos(
		context.Background(),
		logger,
		repos,
		zoekt,
		search.TextRequest,
		query.Yes,
		query.ContainsRefGlobs(q),
	)
	if err != nil {
		return nil, streaming.Stats{}, err
	}

	g, ctx := errgroup.WithContext(ctx)

	b, err := query.ToBasicQuery(q)
	if err != nil {
		return nil, streaming.Stats{}, err
	}

	fieldTypes, _ := q.StringValues(query.FieldType)
	var resultTypes result.Types
	if len(fieldTypes) == 0 {
		resultTypes = result.TypeFile | result.TypePath | result.TypeRepo
	} else {
		for _, t := range fieldTypes {
			resultTypes = resultTypes.With(result.TypeFromString[t])
		}
	}

	typ := search.TextRequest
	zoektQuery, err := zoektutil.QueryToZoektQuery(b, resultTypes, &search.Features{}, typ)
	if err != nil {
		return nil, streaming.Stats{}, err
	}

	zoektParams := &search.ZoektParameters{
		FileMatchLimit:  patternInfo.FileMatchLimit,
		Typ:             typ,
		Select:          patternInfo.Select,
		NumContextLines: 0,
	}

	zoektJob := &zoektutil.RepoSubsetTextSearchJob{
		Repos:       indexed,
		Query:       zoektQuery,
		Typ:         typ,
		ZoektParams: zoektParams,
		Since:       nil,
	}

	// Run literal and regexp searches on indexed repositories.
	g.Go(func() error {
		_, err := zoektJob.Run(ctx, job.RuntimeClients{
			Logger: logger,
			Zoekt:  zoekt,
		}, agg)
		return err
	})

	// Concurrently run searcher for all unindexed repos regardless whether text or regexp.
	g.Go(func() error {
		searcherJob := &searcher.TextSearchJob{
			PatternInfo:     searcherArgs.PatternInfo,
			Repos:           unindexed,
			Indexed:         false,
			UseFullDeadline: searcherArgs.UseFullDeadline,
			NumContextLines: searcherArgs.NumContextLines,
		}

		_, err := searcherJob.Run(ctx, job.RuntimeClients{
			Logger:       logger,
			SearcherURLs: searcherURLs,
			Zoekt:        zoekt,
		}, agg)
		return err
	})

	err = g.Wait()

	fms, fmErr := matchesToFileMatches(agg.Results)
	if fmErr != nil && err == nil {
		err = errors.Wrap(fmErr, "searchFilesInReposBatch failed to convert results")
	}
	return fms, agg.Stats, err
}

func matchesToFileMatches(matches []result.Match) ([]*result.FileMatch, error) {
	fms := make([]*result.FileMatch, 0, len(matches))
	for _, match := range matches {
		fm, ok := match.(*result.FileMatch)
		if !ok {
			return nil, errors.Errorf("expected only file match results")
		}
		fms = append(fms, fm)
	}
	return fms, nil
}
