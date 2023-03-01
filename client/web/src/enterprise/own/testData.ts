import { CodeownersIngestedFile } from './RepositoryOwnPageContents'

export const testCodeOwnersIngestedFile: CodeownersIngestedFile = {
    contents: `# Sourcegraph uses CODENOTIFY to make individuals or groups aware of changes that are happening in code they care about,
# without explicitly requiring those engineers to "own" the code.
# This is a test CODEOWNERS file generated from all the CODENOTIFY files in this repository. It is not intended as a
# move to use CODEOWNERS again, but rather to dogfood our Own product within Sourcegraph (hence the \`.test\` naming)

/.github/workflows/codenotify.yml @unknwon
/.github/workflows/licenses-check.yml          @bobheadxi
/.github/workflows/licenses-update.yml         @bobheadxi
/.github/workflows/renovate-downstream.yml     @bobheadxi
/.github/workflows/renovate-downstream.json    @bobheadxi

/client/branded/src/search-ui/components/** @limitedmage @fkling

/client/jetbrains/** @vdavid @philipp-spiess

/client/shared/src/search/**/* @fkling

/client/shared/src/search/query/**/* @fkling

/client/web/src/enterprise/batches/**/* @eseliger @courier-new @BolajiOlajide

/client/web/src/enterprise/code-monitoring/**/* @limitedmage

/client/web/src/enterprise/codeintel/**/* @efritz

/client/web/src/enterprise/executors/**/* @efritz @eseliger

/client/web/src/integration/batches* @eseliger @courier-new @BolajiOlajide
/client/web/src/integration/search* @limitedmage
/client/web/src/integration/code-monitoring* @limitedmage

/client/web/src/search/**/* @limitedmage @fkling

/cmd/blobstore/**/* @slimsag

/cmd/frontend/graphqlbackend/observability.go @bobheadxi
/cmd/frontend/graphqlbackend/site_monitoring.go @bobheadxi
/cmd/frontend/graphqlbackend/*search*.go @keegancsmith
/cmd/frontend/graphqlbackend/*zoekt*.go @keegancsmith
/cmd/frontend/graphqlbackend/batches.go @eseliger @courier-new
/cmd/frontend/graphqlbackend/insights.go @sourcegraph/code-insights-backend
/cmd/frontend/graphqlbackend/codeintel.go @efritz
/cmd/frontend/graphqlbackend/oobmigrations.go @efritz

/cmd/frontend/internal/search/** @keegancsmith
/cmd/frontend/internal/search/**/* @camdencheek

/cmd/gitserver/server/* @indradhanush @sashaostrikov

/cmd/repo-updater/* @indradhanush @sashaostrikov

/cmd/searcher/**/* @keegancsmith

/cmd/server/internal/goreman/** @keegancsmith

/cmd/server/internal/goremancmd/** @keegancsmith

/cmd/symbols/** @keegancsmith

/cmd/worker/**/* @efritz

/dev/authtest/**/* @unknwon

/dev/codeintel-qa/**/* @efritz

/dev/depgraph/**/* @efritz

/dev/gqltest/**/* @unknwon

/doc/**/admin/** @sourcegraph/delivery
/doc/dev/background-information/adding_ping_data.md @ebrodymoore @dadlerj

/doc/batch_changes/**/* @eseliger @courier-new

/doc/code_navigation/**/* @efritz

/doc/code_search/**/* @rvantonder

/doc/dev/adr/**/* @unknwon

/doc/dev/background-information/codeintel/**/* @efritz

/docker-images/cadvisor/**/* @bobheadxi

/docker-images/grafana/**/* @bobheadxi

/docker-images/postgres-12-alpine/**/* @sourcegraph/delivery

/docker-images/prometheus/**/* @bobheadxi

/enterprise/cmd/executor/**/* @efritz

/enterprise/cmd/frontend/internal/auth/**/* @unknwon

/enterprise/cmd/frontend/internal/authz/**/* @unknwon

/enterprise/cmd/frontend/internal/codeintel/**/* @efritz

/enterprise/cmd/frontend/internal/executorqueue/**/* @efritz @eseliger

/enterprise/cmd/frontend/internal/licensing/**/* @unknwon

/enterprise/cmd/migrator/**/* @efritz

/enterprise/cmd/precise-code-intel-worker/**/* @efritz

/enterprise/cmd/repo-updater/**/* @indradhanush

/enterprise/cmd/repo-updater/internal/authz/**/* @unknwon

/enterprise/cmd/worker/**/* @efritz

/enterprise/cmd/worker/internal/batches/**/* @eseliger

/enterprise/cmd/worker/internal/executorqueue/**/* @efritz @eseliger

/enterprise/cmd/worker/internal/executors/**/* @efritz

/enterprise/dev/ci/**/* @bobheadxi

/enterprise/internal/authz/**/* @unknwon

/enterprise/internal/batches/**/* @eseliger

/enterprise/internal/cloud/**/* @unknwon @michaellzc

/enterprise/internal/codeintel/**/* @efritz @Strum355

/enterprise/internal/database/external_services* @unknwon
/enterprise/internal/database/perms_store* @unknwon

/enterprise/internal/executor/**/* @efritz

/enterprise/internal/insights/**/* @sourcegraph/code-insights-backend

/enterprise/internal/license/**/* @unknwon

/enterprise/internal/licensing/**/* @unknwon

/internal/authz/**/* @unknwon

/internal/codeintel/**/* @efritz

/internal/codeintel/dependencies/**/* @mrnugget

/internal/database/external* @eseliger
/internal/database/namespaces* @eseliger
/internal/database/repos* @eseliger
/internal/database/permissions* @BolajiOlajide
/internal/database/user_roles* @BolajiOlajide
/internal/database/roles* @BolajiOlajide
/internal/database/role_permissions* @BolajiOlajide

/internal/database/basestore/**/* @efritz

/internal/database/batch/**/* @efritz

/internal/database/connections/**/* @efritz

/internal/database/dbconn/**/* @efritz

/internal/database/dbtest/**/* @efritz

/internal/database/dbutil/**/* @efritz

/internal/database/locker/**/* @efritz

/internal/database/migration/**/* @efritz

/internal/database/postgresdsn/**/* @efritz

/internal/debugserver/** @keegancsmith

/internal/diskcache/** @keegancsmith

/internal/endpoint/** @keegancsmith

/internal/env/baseconfig.go @efritz

/internal/extsvc/**/* @eseliger

/internal/extsvc/auth/**/* @unknwon

/internal/gitserver/* @indradhanush @sashaostrikov

/internal/goroutine/**/* @efritz

/internal/gqltestutil/**/* @unknwon

/internal/honey/** @keegancsmith

/internal/httpcli/** @keegancsmith

/internal/lazyregexp/** @keegancsmith

/internal/luasandbox/**/* @efritz

/internal/mutablelimiter/** @keegancsmith

/internal/observation/**/* @sourcegraph/dev-experience

/internal/oobmigration/**/* @efritz

/internal/rcache/** @keegancsmith

/internal/redispool/** @keegancsmith

/internal/repos/* @indradhanush @sashaostrikov

/internal/search/**/* @keegancsmith @camdencheek

/internal/src-cli/**/* @eseliger @BolajiOlajide @courier-new

/internal/symbols/** @keegancsmith

/internal/sysreq/** @keegancsmith

/internal/trace/** @keegancsmith
/internal/trace/**/* @sourcegraph/dev-experience

/internal/tracer/** @keegancsmith
/internal/tracer/**/* @sourcegraph/dev-experience

/internal/usagestats/batches.go @eseliger
/internal/usagestats/batches_test.go @eseliger
/internal/usagestats/*codeintel*.go @efritz

/internal/vcs/** @keegancsmith

/internal/workerutil/**/* @efritz

/lib/codeintel/**/* @efritz

/lib/errors/**/* @sourcegraph/dev-experience

/lib/servicecatalog/**/* @sourcegraph/cloud @sourcegraph/security

/monitoring/**/* @bobheadxi @slimsag @sourcegraph/delivery
/monitoring/frontend.go @efritz
/monitoring/precise_code_intel_* @efritz @sourcegraph/code-intelligence
`,
    updatedAt: '2021-03-15T19:39:11Z',
}
