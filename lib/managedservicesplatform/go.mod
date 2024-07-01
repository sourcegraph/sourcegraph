module github.com/sourcegraph/sourcegraph/lib/managedservicesplatform

go 1.22.4

replace github.com/sourcegraph/sourcegraph/lib => ../

require (
	cloud.google.com/go/bigquery v1.59.1
	cloud.google.com/go/cloudsqlconn v1.5.1
	cloud.google.com/go/profiler v0.4.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.48.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.24.0
	github.com/getsentry/sentry-go v0.25.0
	github.com/go-logr/logr v1.4.2
	github.com/go-logr/stdr v1.2.2
	github.com/google/uuid v1.6.0
	github.com/hexops/autogold/v2 v2.2.1
	github.com/jackc/pgx/v5 v5.5.5
	github.com/openfga/api/proto v0.0.0-20240529184453-5b0b4941f3e0
	github.com/openfga/language/pkg/go v0.0.0-20240409225820-a53ea2892d6d
	github.com/openfga/openfga v1.5.4
	github.com/pressly/goose/v3 v3.20.0
	github.com/prometheus/client_golang v1.19.0
	github.com/redis/go-redis/v9 v9.5.3
	github.com/sourcegraph/conc v0.3.1-0.20240108182409-4afefce20f9b
	github.com/sourcegraph/log v0.0.0-20231018134238-fbadff7458bb
	github.com/sourcegraph/sourcegraph/lib v0.0.0-20231122233253-1f857814717c
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/contrib/detectors/gcp v1.24.0
	go.opentelemetry.io/contrib/propagators/jaeger v1.24.0
	go.opentelemetry.io/contrib/propagators/ot v1.24.0
	go.opentelemetry.io/otel v1.26.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.25.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.25.0
	go.opentelemetry.io/otel/exporters/prometheus v0.46.0
	go.opentelemetry.io/otel/metric v1.26.0
	go.opentelemetry.io/otel/sdk v1.25.0
	go.opentelemetry.io/otel/sdk/metric v1.25.0
	go.opentelemetry.io/otel/trace v1.26.0
	go.uber.org/zap v1.27.0
	google.golang.org/api v0.169.0
	gorm.io/driver/postgres v1.5.4
	gorm.io/gorm v1.25.6
)

require (
	cloud.google.com/go v0.112.1 // indirect
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	cloud.google.com/go/iam v1.1.6 // indirect
	cloud.google.com/go/monitoring v1.18.0 // indirect
	cloud.google.com/go/trace v1.10.5 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.21.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.48.0 // indirect
	github.com/Masterminds/squirrel v1.5.4 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/apache/arrow/go/v14 v14.0.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cockroachdb/errors v1.11.1 // indirect
	github.com/cockroachdb/logtags v0.0.0-20230118201751-21c54148d20b // indirect
	github.com/cockroachdb/redact v1.1.5 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.4 // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-redsync/redsync/v4 v4.13.0 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/cel-go v0.20.1 // indirect
	github.com/google/flatbuffers v23.5.26+incompatible // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/pprof v0.0.0-20230602150820-91b7bce49751 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.2 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.1.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hexops/gotextdiff v1.0.3 // indirect
	github.com/hexops/valast v1.4.4 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.14.3 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.3 // indirect
	github.com/jackc/pgservicefile v0.0.0-20231201235250-de7065d80cb9 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/karlseguin/ccache/v3 v3.0.5 // indirect
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/natefinch/wrap v0.2.0 // indirect
	github.com/nightlyone/lockfile v1.0.0 // indirect
	github.com/oklog/ulid/v2 v2.1.0 // indirect
	github.com/pelletier/go-toml/v2 v2.1.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.0 // indirect
	github.com/prometheus/common v0.48.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/sethvargo/go-retry v0.2.4 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.18.2 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.50.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.51.0 // indirect
	go.opentelemetry.io/proto/otlp v1.2.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/mock v0.4.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.24.0 // indirect
	golang.org/x/exp v0.0.0-20240409090435-93d18d7e34b8 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/oauth2 v0.20.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	golang.org/x/xerrors v0.0.0-20231012003039-104605ab7028 // indirect
	google.golang.org/genproto v0.0.0-20240213162025-012b6fc9bca9 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/grpc v1.64.0 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	mvdan.cc/gofumpt v0.5.0 // indirect
)

// Pending: https://github.com/shurcooL/httpgzip/pull/9
replace github.com/openfga/openfga => github.com/sourcegraph/openfga v0.0.0-20240614204729-de6b563022de
