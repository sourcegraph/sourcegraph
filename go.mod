module github.com/sourcegraph/sourcegraph

go 1.22

toolchain go1.22.1

// Permanent replace directives
// ============================
// These entries indicate permanent replace directives due to significant changes from upstream
// or intentional forks.
replace (
	// We use a fork of Alertmanager to allow prom-wrapper to better manipulate Alertmanager configuration.
	// See https://sourcegraph.com/docs/dev/background-information/observability/prometheus
	github.com/prometheus/alertmanager => github.com/sourcegraph/alertmanager v0.21.1-0.20211110092431-863f5b1ee51b
	// We publish 'dev/ci/images' as a package for import in other tooling.
	// When developing Sourcegraph itself, this replace uses the local package instead of a pushed version.
	github.com/sourcegraph/sourcegraph/dev/ci/images => ./dev/ci/images
	// We publish 'lib' as a package for import in other tooling.
	// When developing Sourcegraph itself, this replace uses the local package instead of a pushed version.
	github.com/sourcegraph/sourcegraph/lib => ./lib
	// We publish 'lib/managedservicesplatform' as a sub-package for import
	// private services developed in other repositories, and to avoid bloating
	// the more lightweight 'lib' package.
	// When developing Sourcegraph itself, this replace uses the local package instead of a pushed version.
	github.com/sourcegraph/sourcegraph/lib/managedservicesplatform => ./lib/managedservicesplatform
	// We publish 'monitoring' as a package for import in other tooling.
	// When developing Sourcegraph itself, this replace uses the local package instead of a pushed version.
	github.com/sourcegraph/sourcegraph/monitoring => ./monitoring
)

// Temporary replace directives
// ============================
// These entries indicate temporary replace directives due to a pending pull request upstream
// or issues with specific versions.
replace (
	// Pending: https://github.com/derision-test/go-mockgen/pull/50
	github.com/derision-test/go-mockgen/v2 => github.com/strum355/go-mockgen/v2 v2.0.0-20240306201845-b2e8d553a343
	// Pending: https://github.com/ghodss/yaml/pull/65
	github.com/ghodss/yaml => github.com/sourcegraph/yaml v1.0.1-0.20200714132230-56936252f152
	// Dependency declares incorrect, old version of redigo, so we must override it: https://github.com/boj/redistore/blob/master/go.mod
	// Pending: https://github.com/boj/redistore/pull/64
	github.com/gomodule/redigo => github.com/gomodule/redigo v1.8.9
	// Pending: Renamed to github.com/google/gnostic. Transitive deps still use the old name (kubernetes/kubernetes).
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.5.5
	// Pending a release cut of https://github.com/prometheus/alertmanager/pull/3010
	github.com/prometheus/common => github.com/prometheus/common v0.32.1
	// Pending: https://github.com/shurcooL/httpgzip/pull/9
	github.com/shurcooL/httpgzip => github.com/sourcegraph/httpgzip v0.0.0-20211015085752-0bad89b3b4df
)

// Status unclear replace directives
// =================================
// These entries indicate replace directives that are defined for unknown reasons.
replace (
	github.com/golang/lint => golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f
	github.com/mattn/goreman => github.com/sourcegraph/goreman v0.1.2-0.20180928223752-6e9a2beb830d
	github.com/russross/blackfriday => github.com/russross/blackfriday v1.5.2
)

require (
	cloud.google.com/go/bigquery v1.57.1
	cloud.google.com/go/kms v1.15.5
	cloud.google.com/go/monitoring v1.17.0
	cloud.google.com/go/profiler v0.4.0
	cloud.google.com/go/pubsub v1.33.0
	cloud.google.com/go/secretmanager v1.11.4
	cloud.google.com/go/storage v1.30.1
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.45.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.21.0
	github.com/Khan/genqlient v0.5.0
	github.com/Masterminds/semver v1.5.0
	github.com/NYTimes/gziphandler v1.1.1
	github.com/PuerkitoBio/rehttp v1.1.0
	github.com/RoaringBitmap/roaring v1.3.0
	github.com/XSAM/otelsql v0.27.0
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/amit7itz/goset v1.0.1
	github.com/aws/aws-sdk-go-v2 v1.17.6
	github.com/aws/aws-sdk-go-v2/config v1.18.16
	github.com/aws/aws-sdk-go-v2/credentials v1.13.16
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.11.56
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.15.0
	github.com/aws/aws-sdk-go-v2/service/codecommit v1.11.0
	github.com/aws/aws-sdk-go-v2/service/kms v1.14.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.30.6
	github.com/aws/smithy-go v1.13.5
	github.com/beevik/etree v1.3.0
	github.com/buildkite/go-buildkite/v3 v3.0.1
	github.com/cespare/xxhash/v2 v2.2.0
	github.com/coreos/go-oidc v2.2.1+incompatible
	github.com/coreos/go-semver v0.3.0
	github.com/crewjam/saml v0.4.14
	github.com/davecgh/go-spew v1.1.1
	github.com/daviddengcn/go-colortext v1.0.0
	github.com/derision-test/glock v1.0.0
	github.com/dgraph-io/ristretto v0.1.1
	github.com/distribution/distribution/v3 v3.0.0-20220128175647-b60926597a1b
	github.com/dnaeon/go-vcr v1.2.0
	github.com/docker/docker-credential-helpers v0.8.1
	github.com/fatih/color v1.16.0
	github.com/felixge/fgprof v0.9.3
	github.com/felixge/httpsnoop v1.0.4
	github.com/fsnotify/fsnotify v1.7.0
	github.com/gen2brain/beeep v0.0.0-20210529141713-5586760f0cc1
	github.com/getsentry/sentry-go v0.25.0
	github.com/ghodss/yaml v1.0.0
	github.com/gitchander/permutation v0.0.0-20210517125447-a5d73722e1b1
	github.com/go-enry/go-enry/v2 v2.8.4
	github.com/go-git/go-git/v5 v5.11.0
	github.com/go-openapi/strfmt v0.21.3
	github.com/gobwas/glob v0.2.3
	github.com/gofrs/uuid v4.2.0+incompatible
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/golang/gddo v0.0.0-20210115222349-20d68f94ee1f
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da
	github.com/gomodule/oauth1 v0.2.0
	github.com/gomodule/redigo v2.0.0+incompatible
	github.com/google/go-cmp v0.6.0
	github.com/google/go-querystring v1.1.0
	github.com/google/uuid v1.5.0
	github.com/gorilla/context v1.1.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v1.2.1
	github.com/goware/urlx v0.3.1
	github.com/grafana/regexp v0.0.0-20221123153739-15dc172cd2db
	github.com/graph-gophers/graphql-go v1.5.0
	github.com/graphql-go/graphql v0.8.1
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79
	github.com/hashicorp/golang-lru/v2 v2.0.2
	github.com/hexops/valast v1.4.4
	github.com/honeycombio/libhoney-go v1.15.8
	github.com/inconshreveable/log15 v0.0.0-20201112154412-8562bdadbbac
	github.com/jackc/pgconn v1.14.0
	github.com/jackc/pgx/v4 v4.18.1
	github.com/joho/godotenv v1.5.1
	github.com/jordan-wright/email v4.0.1-0.20210109023952-943e75fe5223+incompatible
	github.com/json-iterator/go v1.1.12
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/keegancsmith/rpc v1.3.0
	github.com/keegancsmith/sqlf v1.1.1
	github.com/keegancsmith/tmpfriend v0.0.0-20180423180255-86e88902a513
	github.com/klauspost/cpuid/v2 v2.2.4
	github.com/kljensen/snowball v0.6.0
	github.com/kr/text v0.2.0
	github.com/lib/pq v1.10.7
	github.com/machinebox/graphql v0.2.2
	github.com/mattn/go-sqlite3 v1.14.16
	github.com/microcosm-cc/bluemonday v1.0.26
	github.com/mitchellh/hashstructure v1.1.0
	github.com/moby/buildkit v0.12.5
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f
	github.com/opencontainers/go-digest v1.0.0
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/peterbourgon/ff v1.7.1
	github.com/peterbourgon/ff/v3 v3.3.2
	github.com/peterhellberg/link v1.1.0
	github.com/prometheus/alertmanager v0.24.0
	github.com/prometheus/client_golang v1.18.0
	github.com/prometheus/common v0.45.0
	github.com/qustavo/sqlhooks/v2 v2.1.0
	github.com/rafaeljusto/redigomock/v3 v3.1.2
	github.com/rjeczalik/notify v0.9.3
	github.com/russellhaering/gosaml2 v0.9.1
	github.com/russellhaering/goxmldsig v1.4.0
	github.com/schollz/progressbar/v3 v3.13.1
	github.com/segmentio/fasthash v1.0.3
	github.com/segmentio/ksuid v1.0.4
	github.com/sergi/go-diff v1.3.1
	github.com/shurcooL/httpgzip v0.0.0-20190720172056-320755c1c1b0
	github.com/slack-go/slack v0.10.1
	github.com/smacker/go-tree-sitter v0.0.0-20231219031718-233c2f923ac7
	github.com/sourcegraph/go-ctags v0.0.0-20231024141911-299d0263dc95
	github.com/sourcegraph/go-diff v0.6.2-0.20221123165719-f8cd299c40f3
	github.com/sourcegraph/go-jsonschema v0.0.0-20221230021921-34aaf28fc4ac
	github.com/sourcegraph/go-langserver v2.0.1-0.20181108233942-4a51fa2e1238+incompatible
	github.com/sourcegraph/go-lsp v0.0.0-20200429204803-219e11d77f5d
	github.com/sourcegraph/go-rendezvous v0.0.0-20210910070954-ef39ade5591d
	github.com/sourcegraph/jsonx v0.0.0-20200629203448-1a936bd500cf
	github.com/sourcegraph/log v0.0.0-20231018134238-fbadff7458bb
	github.com/sourcegraph/run v0.12.0
	github.com/sourcegraph/sourcegraph/dev/ci/images v0.0.0-20220203145655-4d2a39d3038a
	github.com/stretchr/testify v1.8.4
	github.com/temoto/robotstxt v1.1.2
	github.com/throttled/throttled/v2 v2.12.0
	github.com/tidwall/gjson v1.14.0
	github.com/tj/go-naturaldate v1.3.0
	github.com/tomnomnom/linkheader v0.0.0-20180905144013-02ca5825eb80
	github.com/uber/gonduit v0.13.0
	github.com/uber/jaeger-client-go v2.30.0+incompatible
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/urfave/cli/v2 v2.25.7
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/xeonx/timeago v1.0.0-rc4
	github.com/yuin/gopher-lua v0.0.0-20210529063254-f4c35e4016d9
	go.opentelemetry.io/collector v0.92.0 // indirect
	go.opentelemetry.io/collector/component v0.92.0
	go.opentelemetry.io/collector/exporter v0.92.0
	go.opentelemetry.io/collector/exporter/otlpexporter v0.92.0
	go.opentelemetry.io/collector/exporter/otlphttpexporter v0.92.0
	go.opentelemetry.io/collector/receiver v0.92.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.92.0
	go.opentelemetry.io/contrib/detectors/gcp v1.22.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.46.1
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.47.0
	go.opentelemetry.io/contrib/propagators/jaeger v1.22.0
	go.opentelemetry.io/contrib/propagators/ot v1.22.0
	go.opentelemetry.io/otel v1.22.0
	go.opentelemetry.io/otel/bridge/opentracing v1.22.0 // indirect
	go.opentelemetry.io/otel/exporters/jaeger v1.17.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.22.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.22.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.22.0
	go.opentelemetry.io/otel/metric v1.22.0
	go.opentelemetry.io/otel/sdk v1.22.0
	go.opentelemetry.io/otel/sdk/metric v1.22.0
	go.opentelemetry.io/otel/trace v1.22.0
	go.opentelemetry.io/proto/otlp v1.1.0
	go.uber.org/atomic v1.11.0
	go.uber.org/automaxprocs v1.5.2
	go.uber.org/ratelimit v0.2.0
	go.uber.org/zap v1.26.0
	golang.org/x/crypto v0.19.0
	golang.org/x/net v0.21.0
	golang.org/x/oauth2 v0.16.0
	golang.org/x/sync v0.6.0
	golang.org/x/sys v0.17.0
	golang.org/x/time v0.5.0
	golang.org/x/tools v0.18.0
	gonum.org/v1/gonum v0.13.0
	google.golang.org/api v0.156.0
	google.golang.org/genproto v0.0.0-20240116215550-a9fa1716bcac // indirect
	google.golang.org/protobuf v1.32.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.26.2
	k8s.io/apimachinery v0.26.2
	k8s.io/client-go v0.26.2
	k8s.io/utils v0.0.0-20230220204549-a5ecb0141aa5
	layeh.com/gopher-luar v1.0.10
	sigs.k8s.io/kustomize/kyaml v0.13.3
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai v0.4.1
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.9.1
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.4.0
	github.com/TwiN/go-away v1.6.13
	github.com/aws/constructs-go/constructs/v10 v10.2.69
	github.com/aws/jsii-runtime-go v1.84.0
	github.com/bazelbuild/bazel-gazelle v0.35.0
	github.com/derision-test/go-mockgen/v2 v2.0.1
	github.com/dghubble/gologin/v2 v2.4.0
	github.com/edsrzf/mmap-go v1.1.0
	github.com/go-json-experiment/json v0.0.0-20231102232822-2e55bd4e08b0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/go-redsync/redsync/v4 v4.8.1
	github.com/google/go-containerregistry v0.16.1
	github.com/google/go-github/v48 v48.2.0
	github.com/google/go-github/v55 v55.0.0
	github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus v1.0.0-rc.0
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0
	github.com/hashicorp/cronexpr v1.1.1
	github.com/hashicorp/go-tfe v1.32.1
	github.com/hashicorp/terraform-cdk-go/cdktf v0.17.3
	github.com/invopop/jsonschema v0.12.0
	github.com/jackc/pgerrcode v0.0.0-20220416144525-469b46aa5efa
	github.com/mroth/weightedrand/v2 v2.0.1
	github.com/nxadm/tail v1.4.11
	github.com/oschwald/maxminddb-golang v1.12.0
	github.com/pkoukk/tiktoken-go v0.1.6
	github.com/prometheus/statsd_exporter v0.22.7
	github.com/qdrant/go-client v1.4.1
	github.com/sourcegraph/managed-services-platform-cdktf/gen/cloudflare v0.0.0-20230822024612-edb48c530722
	github.com/sourcegraph/managed-services-platform-cdktf/gen/google v0.0.0-20231222002716-51aa712b9bcc
	github.com/sourcegraph/managed-services-platform-cdktf/gen/google_beta v0.0.0-20231222002716-51aa712b9bcc
	github.com/sourcegraph/managed-services-platform-cdktf/gen/nobl9 v0.0.0-20240131130226-cb88836e6df8
	github.com/sourcegraph/managed-services-platform-cdktf/gen/opsgenie v0.0.0-20231230001101-a13188f9c749
	github.com/sourcegraph/managed-services-platform-cdktf/gen/postgresql v0.0.0-20231220215815-b87ebb3e8c47
	github.com/sourcegraph/managed-services-platform-cdktf/gen/random v0.0.0-20230822024612-edb48c530722
	github.com/sourcegraph/managed-services-platform-cdktf/gen/sentry v0.0.0-20240109225336-01188650a68d
	github.com/sourcegraph/managed-services-platform-cdktf/gen/slack v0.0.0-20240103014439-025608ddf849
	github.com/sourcegraph/managed-services-platform-cdktf/gen/tfe v0.0.0-20231218231056-4749baca142f
	github.com/sourcegraph/scip v0.3.3
	github.com/sourcegraph/sourcegraph/lib v0.0.0-20231122233253-1f857814717c
	github.com/sourcegraph/sourcegraph/lib/managedservicesplatform v0.0.0-00010101000000-000000000000
	github.com/sourcegraph/sourcegraph/monitoring v0.0.0-00010101000000-000000000000
	github.com/vektah/gqlparser/v2 v2.4.5
	github.com/vvakame/gcplogurl v0.2.0
	go.opentelemetry.io/collector/config/confighttp v0.92.0
	go.opentelemetry.io/collector/config/configtelemetry v0.92.0
	go.opentelemetry.io/collector/config/configtls v0.92.0
	go.opentelemetry.io/otel/exporters/prometheus v0.44.1-0.20231201153405-6027c1ae76f2
	google.golang.org/genproto/googleapis/api v0.0.0-20240125205218-1f4bbc51befe
	gorm.io/gorm v1.25.5
)

require (
	cloud.google.com/go/cloudsqlconn v1.5.1 // indirect
	cloud.google.com/go/compute/metadata v0.2.4-0.20230617002413-005d2dfb6b68 // indirect
	cloud.google.com/go/trace v1.10.4 // indirect
	dario.cat/mergo v1.0.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.5.1 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.1.1 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.21.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.45.0 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/agnivade/levenshtein v1.1.1 // indirect
	github.com/alexflint/go-arg v1.4.2 // indirect
	github.com/alexflint/go-scalar v1.0.0 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/apache/arrow/go/v12 v12.0.0 // indirect
	github.com/apache/thrift v0.16.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.0.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.1.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.14.5 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/bazelbuild/buildtools v0.0.0-20231115204819-d4c9dccdfbb1 // indirect
	github.com/bufbuild/connect-go v1.9.0 // indirect
	github.com/bufbuild/connect-opentelemetry-go v0.4.0 // indirect
	github.com/bufbuild/protocompile v0.5.1 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/cockroachdb/apd/v2 v2.0.1 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/containerd/typeurl/v2 v2.1.1 // indirect
	github.com/creack/pty v1.1.21 // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/dghubble/sling v1.4.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/di-wu/parser v0.2.2 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/cli v25.0.2+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/emicklei/go-restful/v3 v3.10.1 // indirect
	github.com/fullstorydev/grpcurl v1.8.6 // indirect
	github.com/go-chi/chi/v5 v5.0.10 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gofrs/uuid/v5 v5.0.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.0.0 // indirect
	github.com/google/flatbuffers v2.0.8+incompatible // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/gosimple/slug v1.12.0 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/grafana-tools/sdk v0.0.0-20220919052116-6562121319fc // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-hclog v1.2.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-slug v0.12.1 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/jsonapi v0.0.0-20210826224640-ee7dae0fb22d // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.2 // indirect
	github.com/jackc/pgx/v5 v5.5.0 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/asmfmt v1.3.2 // indirect
	github.com/knadh/koanf/v2 v2.0.1 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/matryer/is v1.2.0 // indirect
	github.com/minio/asm2plan9s v0.0.0-20200509001527-cdd76441f9d8 // indirect
	github.com/minio/c2goasm v0.0.0-20190812172519-36a3d3bbc4f3 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/mpvl/unique v0.0.0-20150818121801-cbe035fff7de // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/onsi/ginkgo/v2 v2.11.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc6 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20221212215047-62379fc7944b // indirect
	github.com/prometheus/prometheus v0.40.5 // indirect
	github.com/rickb777/date v1.14.3 // indirect
	github.com/rickb777/plural v1.2.2 // indirect
	github.com/shirou/gopsutil/v3 v3.23.12 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/skeema/knownhosts v1.2.1 // indirect
	github.com/smartystreets/assertions v1.13.0 // indirect
	github.com/tetratelabs/wazero v1.3.0 // indirect
	github.com/vbatts/tar-split v0.11.3 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	github.com/zenazn/goji v1.0.1 // indirect
	go.opentelemetry.io/collector/config/configauth v0.92.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v0.92.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.92.0 // indirect
	go.opentelemetry.io/collector/config/confignet v0.92.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v0.92.0 // indirect
	go.opentelemetry.io/collector/config/configretry v0.92.0 // indirect
	go.opentelemetry.io/collector/config/internal v0.92.0 // indirect
	go.opentelemetry.io/collector/confmap v0.92.0 // indirect
	go.opentelemetry.io/collector/consumer v0.92.0 // indirect
	go.opentelemetry.io/collector/extension v0.92.0 // indirect
	go.opentelemetry.io/collector/extension/auth v0.92.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.1 // indirect
	go.uber.org/goleak v1.3.0 // indirect
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616 // indirect
	golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240125205218-1f4bbc51befe // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gotest.tools/v3 v3.5.1 // indirect
)

require (
	bitbucket.org/creachadair/shell v0.0.7 // indirect
	cloud.google.com/go v0.111.0 // indirect
	cloud.google.com/go/compute v1.23.3 // indirect
	cloud.google.com/go/iam v1.1.5 // indirect
	cuelang.org/go v0.4.3
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/ProtonMail/go-crypto v1.0.0 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/alecthomas/chroma v0.10.0 // indirect
	github.com/alecthomas/chroma/v2 v2.4.0
	github.com/alecthomas/kingpin v2.2.6+incompatible // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137
	github.com/andres-erbsen/clock v0.0.0-20160526145045-9e14626cd129 // indirect
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d // indirect
	github.com/aws/aws-sdk-go v1.44.128
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.4.10
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.24 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.24 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.31 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.9.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.13.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.12.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.18.6 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/becheran/wildmatch-go v1.0.0
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.8.0
	github.com/bmatcuk/doublestar v1.3.4
	github.com/boj/redistore v0.0.0-20180917114910-cd5dcc76aeff
	github.com/bufbuild/buf v1.25.0 // indirect
	github.com/c2h5oh/datasize v0.0.0-20220606134207-859f65c6625b
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/charmbracelet/glamour v0.5.0 // indirect
	github.com/cockroachdb/errors v1.11.1
	github.com/cockroachdb/logtags v0.0.0-20230118201751-21c54148d20b // indirect
	github.com/cockroachdb/redact v1.1.5
	github.com/coreos/go-iptables v0.6.0
	github.com/cpuguy83/go-md2man/v2 v2.0.3 // indirect
	github.com/dave/jennifer v1.6.1 // indirect
	github.com/di-wu/xsd-datetime v1.0.0
	github.com/djherbis/buffer v1.2.0 // indirect
	github.com/djherbis/nio/v3 v3.0.1 // indirect
	github.com/dlclark/regexp2 v1.10.0 // indirect
	github.com/docker/docker v25.0.2+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1
	github.com/elimity-com/scim v0.0.0-20220121082953-15165b1a61c8
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.2 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/facebookgo/limitgroup v0.0.0-20150612190941-6abd8d71ec01 // indirect
	github.com/facebookgo/muster v0.0.0-20150708232844-fd3d7953fd52 // indirect
	github.com/frankban/quicktest v1.14.4
	github.com/fullstorydev/grpcui v1.3.1
	github.com/go-enry/go-oniguruma v1.2.1 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.5.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.4.1
	github.com/go-logr/stdr v1.2.2
	github.com/go-openapi/analysis v0.21.4 // indirect
	github.com/go-openapi/errors v0.20.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/loads v0.21.2 // indirect
	github.com/go-openapi/runtime v0.24.2 // indirect
	github.com/go-openapi/spec v0.20.7 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-openapi/validate v0.22.0 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/go-toast/toast v0.0.0-20190211030409-01e6764cf0a4 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/golang/glog v1.1.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20230705174524-200ffdc848b8 // indirect
	github.com/googleapis/gax-go/v2 v2.12.0
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/gopherjs/gopherwasm v1.1.0 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.1 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.4 // indirect
	github.com/hashicorp/go-version v1.6.0
	github.com/hexops/autogold/v2 v2.2.1
	github.com/hexops/gotextdiff v1.0.3
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/itchyny/gojq v0.12.14 // indirect
	github.com/itchyny/timefmt-go v0.1.5 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgtype v1.14.0
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jdxcode/netrc v0.0.0-20221124155335-4616370d1a84 // indirect
	github.com/jhump/protoreflect v1.15.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/k3a/html2text v1.1.0
	github.com/karlseguin/typed v1.1.8 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.17.5 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/kr/pretty v0.3.1
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattermost/xml-roundtrip-validator v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20220423185008-bf980b35cac4
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/mostynb/go-grpc-compression v1.2.2 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/muesli/termenv v0.12.0 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/mwitkow/go-proto-validators v0.3.2 // indirect
	github.com/nightlyone/lockfile v1.0.0 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/olekukonko/tablewriter v0.0.5
	github.com/opsgenie/opsgenie-go-sdk-v2 v1.2.22
	github.com/pandatix/go-cvss v0.5.2
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/profile v1.7.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	github.com/prometheus/client_model v0.5.0
	github.com/prometheus/common/sigv4 v0.1.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/pseudomuto/protoc-gen-doc v1.5.1
	github.com/pseudomuto/protokit v0.2.1 // indirect
	github.com/rivo/uniseg v0.4.6 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/rs/cors v1.10.1 // indirect
	github.com/rs/xid v1.5.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06
	github.com/scim2/filter-parser/v2 v2.2.0
	github.com/sourcegraph/conc v0.3.1-0.20240108182409-4afefce20f9b
	github.com/sourcegraph/mountinfo v0.0.0-20240201124957-b314c0befab1
	github.com/sourcegraph/zoekt v0.0.0-20240212021124-1c158f9b8661
	github.com/spf13/cobra v1.8.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/tadvi/systray v0.0.0-20190226123456-11a2b8fa57af // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/vmihailenco/msgpack/v5 v5.3.5 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8
	github.com/xanzy/go-gitlab v0.86.0
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xlab/treeprint v1.1.0 // indirect
	github.com/yuin/goldmark v1.5.6
	github.com/yuin/goldmark-emoji v1.0.1 // indirect
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20220924101305-151362477c87
	go.bobheadxi.dev/streamline v1.3.2
	go.mongodb.org/mongo-driver v1.11.3 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector/pdata v1.0.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20230713183714-613f0c0eb8a1
	golang.org/x/mod v0.15.0
	golang.org/x/term v0.17.0 // indirect
	golang.org/x/text v0.14.0
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/grpc v1.61.0
	gopkg.in/alexcesaro/statsd.v2 v2.0.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gorm.io/driver/postgres v1.5.4
	k8s.io/klog/v2 v2.90.1 // indirect
	k8s.io/kube-openapi v0.0.0-20221012153701-172d655c2280 // indirect
	mvdan.cc/gofumpt v0.5.0 // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
)
