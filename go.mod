module github.com/sourcegraph/sourcegraph

go 1.17

require (
	cloud.google.com/go/kms v1.1.0
	cloud.google.com/go/monitoring v1.2.0
	cloud.google.com/go/profiler v0.1.2
	cloud.google.com/go/pubsub v1.17.1
	cloud.google.com/go/storage v1.19.0
	github.com/Masterminds/semver v1.5.0
	github.com/NYTimes/gziphandler v1.1.1
	github.com/PuerkitoBio/rehttp v1.1.0
	github.com/RoaringBitmap/roaring v0.9.4
	github.com/aws/aws-sdk-go-v2 v1.13.0
	github.com/aws/aws-sdk-go-v2/config v1.13.1
	github.com/aws/aws-sdk-go-v2/credentials v1.8.0
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.9.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.15.0
	github.com/aws/aws-sdk-go-v2/service/codecommit v1.11.0
	github.com/aws/aws-sdk-go-v2/service/kms v1.14.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.24.1
	github.com/aws/smithy-go v1.10.0
	github.com/beevik/etree v1.1.0
	github.com/boj/redistore v0.0.0-20180917114910-cd5dcc76aeff
	github.com/buildkite/go-buildkite/v3 v3.0.1
	github.com/cespare/xxhash v1.1.0
	github.com/cespare/xxhash/v2 v2.1.2
	github.com/charmbracelet/glamour v0.5.0
	github.com/coreos/go-oidc v2.2.1+incompatible
	github.com/coreos/go-semver v0.3.0
	github.com/crewjam/saml v0.4.6
	github.com/davecgh/go-spew v1.1.1
	github.com/daviddengcn/go-colortext v1.0.0
	github.com/derision-test/glock v1.0.0
	github.com/derision-test/go-mockgen v1.1.4
	github.com/dghubble/gologin v2.2.0+incompatible
	github.com/dgraph-io/ristretto v0.1.0
	github.com/dineshappavoo/basex v0.0.0-20170425072625-481a6f6dc663
	github.com/distribution/distribution/v3 v3.0.0-20220128175647-b60926597a1b
	github.com/dnaeon/go-vcr v1.2.0
	github.com/docker/docker-credential-helpers v0.6.4
	github.com/fatih/color v1.13.0
	github.com/fatih/structs v1.1.0
	github.com/felixge/fgprof v0.9.2
	github.com/felixge/httpsnoop v1.0.2
	github.com/fsnotify/fsnotify v1.5.1
	github.com/gen2brain/beeep v0.0.0-20210529141713-5586760f0cc1
	github.com/getsentry/raven-go v0.2.0
	github.com/getsentry/sentry-go v0.12.0
	github.com/ghodss/yaml v1.0.0
	github.com/gitchander/permutation v0.0.0-20210517125447-a5d73722e1b1
	github.com/go-enry/go-enry/v2 v2.8.0
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-openapi/strfmt v0.21.1
	github.com/go-redsync/redsync v1.4.2
	github.com/gobwas/glob v0.2.3
	github.com/golang/gddo v0.0.0-20210115222349-20d68f94ee1f
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da
	github.com/gomodule/oauth1 v0.2.0
	github.com/gomodule/redigo v2.0.0+incompatible
	github.com/google/go-cmp v0.5.7
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-github/v28 v28.1.1
	github.com/google/go-github/v31 v31.0.0
	github.com/google/go-querystring v1.1.0
	github.com/google/uuid v1.3.0
	github.com/google/zoekt v0.0.0-20211108135652-f8e8ada171c7
	github.com/gorilla/context v1.1.1
	github.com/gorilla/csrf v1.7.1
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v1.2.1
	github.com/goware/urlx v0.3.1
	github.com/grafana-tools/sdk v0.0.0-20220203092117-edae16afa87b
	github.com/grafana/regexp v0.0.0-20220202152701-6a046c4caf32
	github.com/graph-gophers/graphql-go v1.3.0
	github.com/graphql-go/graphql v0.8.0
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79
	github.com/hashicorp/golang-lru v0.5.4
	github.com/hexops/autogold v1.3.0
	github.com/hexops/valast v1.4.1
	github.com/honeycombio/libhoney-go v1.15.8
	github.com/inconshreveable/log15 v0.0.0-20201112154412-8562bdadbbac
	github.com/jackc/pgconn v1.10.1
	github.com/jackc/pgx/v4 v4.14.1
	github.com/jmoiron/sqlx v1.3.4
	github.com/joho/godotenv v1.4.0
	github.com/jordan-wright/email v4.0.1-0.20210109023952-943e75fe5223+incompatible
	github.com/json-iterator/go v1.1.12
	github.com/karrick/godirwalk v1.16.1
	github.com/keegancsmith/rpc v1.3.0
	github.com/keegancsmith/sqlf v1.1.1
	github.com/keegancsmith/tmpfriend v0.0.0-20180423180255-86e88902a513
	github.com/kr/text v0.2.0
	github.com/kylelemons/godebug v1.1.0
	github.com/lib/pq v1.10.4
	github.com/machinebox/graphql v0.2.2
	github.com/mattn/go-sqlite3 v1.14.11
	github.com/mcuadros/go-version v0.0.0-20190830083331-035f6764e8d2
	github.com/microcosm-cc/bluemonday v1.0.17
	github.com/mitchellh/hashstructure v1.1.0
	github.com/moby/buildkit v0.9.3
	github.com/montanaflynn/stats v0.6.6
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f
	github.com/neelance/parallel v0.0.0-20160708114440-4de9ce63d14c
	github.com/opencontainers/go-digest v1.0.0
	github.com/opentracing-contrib/go-stdlib v1.0.0
	github.com/opentracing/opentracing-go v1.2.0
	github.com/peterbourgon/ff v1.7.1
	github.com/peterbourgon/ff/v3 v3.1.2
	github.com/peterhellberg/link v1.1.0
	github.com/prometheus/alertmanager v0.23.0
	github.com/prometheus/client_golang v1.12.1
	github.com/prometheus/common v0.32.1
	github.com/pseudomuto/protoc-gen-doc v1.5.0
	github.com/qustavo/sqlhooks/v2 v2.1.0
	github.com/rjeczalik/notify v0.9.2
	github.com/robert-nix/ansihtml v1.0.0
	github.com/russellhaering/gosaml2 v0.6.0
	github.com/russellhaering/goxmldsig v1.1.1
	github.com/schollz/progressbar/v3 v3.8.5
	github.com/sebdah/goldie/v2 v2.5.3
	github.com/segmentio/fasthash v1.0.3
	github.com/segmentio/ksuid v1.0.4
	github.com/sergi/go-diff v1.2.0
	github.com/shurcooL/github_flavored_markdown v0.0.0-20210228213109-c3a9aa474629
	github.com/shurcooL/httpgzip v0.0.0-20190720172056-320755c1c1b0
	github.com/slack-go/slack v0.10.1
	github.com/smacker/go-tree-sitter v0.0.0-20220209044044-0d3022e933c3
	github.com/snabb/sitemap v1.0.0
	github.com/sourcegraph/ctxvfs v0.0.0-20180418081416-2b65f1b1ea81
	github.com/sourcegraph/go-ctags v0.0.0-20220221141751-78951a22ec08
	github.com/sourcegraph/go-diff v0.6.1
	github.com/sourcegraph/go-jsonschema v0.0.0-20211011105148-2e30f7bacbe1
	github.com/sourcegraph/go-langserver v2.0.1-0.20181108233942-4a51fa2e1238+incompatible
	github.com/sourcegraph/go-lsp v0.0.0-20200429204803-219e11d77f5d
	github.com/sourcegraph/go-rendezvous v0.0.0-20210910070954-ef39ade5591d
	github.com/sourcegraph/jsonx v0.0.0-20200629203448-1a936bd500cf
	github.com/sourcegraph/sourcegraph/enterprise/dev/ci/images v0.0.0-20220203145655-4d2a39d3038a
	github.com/sourcegraph/sourcegraph/lib v0.0.0-20220203145655-4d2a39d3038a
	github.com/stretchr/testify v1.7.0
	github.com/stripe/stripe-go v70.15.0+incompatible
	github.com/stvp/tempredis v0.0.0-20181119212430-b82af8480203
	github.com/temoto/robotstxt v1.1.2
	github.com/throttled/throttled/v2 v2.9.0
	github.com/tidwall/gjson v1.14.0
	github.com/tj/go-naturaldate v1.3.0
	github.com/tomnomnom/linkheader v0.0.0-20180905144013-02ca5825eb80
	github.com/uber/gonduit v0.13.0
	github.com/uber/jaeger-client-go v2.30.0+incompatible
	github.com/uber/jaeger-lib v2.4.1+incompatible
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/xeonx/timeago v1.0.0-rc4
	github.com/xhit/go-str2duration/v2 v2.0.0
	go.etcd.io/bbolt v1.3.6
	go.uber.org/atomic v1.9.0
	go.uber.org/automaxprocs v1.4.0
	go.uber.org/ratelimit v0.2.0
	golang.org/x/crypto v0.0.0-20220315160706-3147a52a75dd
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158
	golang.org/x/time v0.0.0-20211116232009-f0f3c7e86c11
	golang.org/x/tools v0.1.9
	google.golang.org/api v0.66.0
	google.golang.org/genproto v0.0.0-20220202230416-2a053f022f0d
	google.golang.org/protobuf v1.27.1
	gopkg.in/DataDog/dd-trace-go.v1 v1.36.2
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.23.3
	k8s.io/apimachinery v0.23.3
	k8s.io/client-go v0.23.3
	sigs.k8s.io/kustomize/kyaml v0.13.3
)

require (
	cloud.google.com/go/compute v1.1.0 // indirect
	cloud.google.com/go/iam v0.1.1 // indirect
	github.com/BurntSushi/toml v0.4.1 // indirect
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.0.0-20211129110424-6491aa3bf583 // indirect
	github.com/DataDog/datadog-go v4.8.2+incompatible // indirect
	github.com/DataDog/datadog-go/v5 v5.0.2 // indirect
	github.com/DataDog/gostackparse v0.5.0 // indirect
	github.com/DataDog/sketches-go v1.0.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/alecthomas/chroma v0.10.0 // indirect
	github.com/cockroachdb/errors v1.8.9 // indirect
	github.com/containerd/typeurl v1.0.2 // indirect
	github.com/dlclark/regexp2 v1.4.0 // indirect
	github.com/docker/docker v20.10.7+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v0.6.3 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/muesli/termenv v0.9.0 // indirect
	github.com/mwitkow/go-proto-validators v0.3.2 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/philhofer/fwd v1.1.1 // indirect
	github.com/pseudomuto/protokit v0.2.0 // indirect
	github.com/tinylib/msgp v1.1.2 // indirect
	github.com/yuin/goldmark v1.4.4 // indirect
	github.com/yuin/goldmark-emoji v1.0.1 // indirect
	sigs.k8s.io/json v0.0.0-20211208200746-9f7c6b3444d2 // indirect
)

require (
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/bufbuild/buf v0.56.0
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/gofrs/uuid v4.0.0+incompatible
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jhump/protoreflect v1.9.1-0.20210817181203-db1a327a393e // indirect
	github.com/klauspost/pgzip v1.2.5 // indirect
	github.com/pkg/profile v1.6.0 // indirect
	github.com/spf13/cobra v1.2.1 // indirect
	github.com/twitchtv/twirp v8.1.0+incompatible // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.20.0 // indirect
)

require (
	cloud.google.com/go v0.100.2 // indirect
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20220113124808-70ae35bab23f // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/alecthomas/kingpin v2.2.6+incompatible // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/andres-erbsen/clock v0.0.0-20160526145045-9e14626cd129 // indirect
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.10.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.11.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.9.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.14.0 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.2.1
	github.com/bmatcuk/doublestar v1.3.4 // indirect
	github.com/certifi/gocertifi v0.0.0-20210507211836-431795d63e8d // indirect
	github.com/cockroachdb/logtags v0.0.0-20211118104740-dabe8e521a4f // indirect
	github.com/cockroachdb/redact v1.1.3
	github.com/dave/jennifer v1.4.1 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/facebookgo/limitgroup v0.0.0-20150612190941-6abd8d71ec01 // indirect
	github.com/facebookgo/muster v0.0.0-20150708232844-fd3d7953fd52 // indirect
	github.com/go-enry/go-oniguruma v1.2.1 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.3.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-openapi/analysis v0.21.2 // indirect
	github.com/go-openapi/errors v0.20.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.6 // indirect
	github.com/go-openapi/loads v0.21.1 // indirect
	github.com/go-openapi/runtime v0.22.0 // indirect
	github.com/go-openapi/spec v0.20.4 // indirect
	github.com/go-openapi/swag v0.21.1 // indirect
	github.com/go-openapi/validate v0.20.3 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt/v4 v4.2.0
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20220128192902-513e8ac6eea1 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	github.com/googleapis/gnostic v0.6.6 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/gosimple/slug v1.12.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.0 // indirect
	github.com/hexops/gotextdiff v1.0.3 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.2.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200714003250-2b9c44734f2b // indirect
	github.com/jackc/pgtype v1.9.1 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/karlseguin/typed v1.1.8 // indirect
	github.com/kevinburke/ssh_config v1.1.0 // indirect
	github.com/klauspost/compress v1.14.2 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattermost/xml-roundtrip-validator v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/nightlyone/lockfile v1.0.0 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/rs/xid v1.3.0 // indirect
	github.com/russross/blackfriday v1.6.0 // indirect
	github.com/shurcooL/go-goon v0.0.0-20210110234559-7585751d9a17 // indirect
	github.com/shurcooL/highlight_diff v0.0.0-20181222201841-111da2e7d480 // indirect
	github.com/shurcooL/highlight_go v0.0.0-20191220051317-782971ddf21b // indirect
	github.com/shurcooL/octicon v0.0.0-20191102190552-cbb32d6a785c // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/snabb/diagio v1.0.0 // indirect
	github.com/sourcegraph/annotate v0.0.0-20160123013949-f4cad6c6324d // indirect
	github.com/sourcegraph/syntaxhighlight v0.0.0-20170531221838-bd320f5d308e // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/src-d/gcfg v1.4.0 // indirect
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/vmihailenco/msgpack/v5 v5.3.5 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/xanzy/ssh-agent v0.3.1 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/zenazn/goji v1.0.1 // indirect
	go.mongodb.org/mongo-driver v1.8.3 // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/mod v0.5.1
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/grpc v1.44.0 // indirect
	gopkg.in/alexcesaro/statsd.v2 v2.0.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/kube-openapi v0.0.0-20220124234850-424119656bbf // indirect
	k8s.io/utils v0.0.0-20220127004650-9b3446523e65
	mvdan.cc/gofumpt v0.2.1 // indirect
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/HdrHistogram/hdrhistogram-go v1.1.2 // indirect
	github.com/aws/aws-sdk-go v1.42.45 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.2.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.2.0 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-kit/log v0.2.0 // indirect
	github.com/go-logr/logr v1.2.2 // indirect
	github.com/go-toast/toast v0.0.0-20190211030409-01e6764cf0a4 // indirect
	github.com/godbus/dbus/v5 v5.0.6 // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-github/v41 v41.0.0
	github.com/gopherjs/gopherjs v0.0.0-20220104163920-15ed2e8cf2bd // indirect
	github.com/gopherjs/gopherwasm v1.1.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/hashicorp/go-hclog v0.16.2 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/prometheus/common/sigv4 v0.1.0 // indirect
	github.com/tadvi/systray v0.0.0-20190226123456-11a2b8fa57af // indirect
	github.com/xlab/treeprint v1.1.0 // indirect
	k8s.io/klog/v2 v2.40.1 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
)

// Permanent replace directives
// ============================
// These entries indicate permanent replace directives due to significant changes from upstream
// or intentional forks.
replace (
	// We maintain our own fork of Zoekt. Update with ./dev/zoekt/update
	github.com/google/zoekt => github.com/sourcegraph/zoekt v0.0.0-20220329183736-10d20196f75e
	// We use a fork of Alertmanager to allow prom-wrapper to better manipulate Alertmanager configuration.
	// See https://docs.sourcegraph.com/dev/background-information/observability/prometheus
	github.com/prometheus/alertmanager => github.com/sourcegraph/alertmanager v0.21.1-0.20211110092431-863f5b1ee51b
	// We publish 'enterprise/dev/ci/images' as a package for import in other tooling.
	// When developing Sourcegraph itself, this replace uses the local package instead of a pushed version.
	github.com/sourcegraph/sourcegraph/enterprise/dev/ci/images => ./enterprise/dev/ci/images
	// We publish 'lib' as a package for import in other tooling.
	// When developing Sourcegraph itself, this replace uses the local package instead of a pushed version.
	github.com/sourcegraph/sourcegraph/lib => ./lib
)

// Temporary replace directives
// ============================
// These entries indicate temporary replace directives due to a pending pull request upstream
// or issues with specific versions.
replace (
	// Pending: https://github.com/ghodss/yaml/pull/65
	github.com/ghodss/yaml => github.com/sourcegraph/yaml v1.0.1-0.20200714132230-56936252f152
	// Pending: Renamed to github.com/google/gnostic. Transitive deps still use the old name (kubernetes/kubernetes).
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.5.5
	// Pending: https://github.com/shurcooL/httpgzip/pull/9
	github.com/shurcooL/httpgzip => github.com/sourcegraph/httpgzip v0.0.0-20211015085752-0bad89b3b4df
)

// Status unclear replace directives
// =================================
// These entries indicate replace directives that are defined for unknown reasons.
replace (
	github.com/dghubble/gologin => github.com/sourcegraph/gologin v1.0.2-0.20181110030308-c6f1b62954d8
	github.com/golang/lint => golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f
	github.com/mattn/goreman => github.com/sourcegraph/goreman v0.1.2-0.20180928223752-6e9a2beb830d
	github.com/russellhaering/gosaml2 => github.com/sourcegraph/gosaml2 v0.6.1-0.20210128133756-84151d087b10
	github.com/russross/blackfriday => github.com/russross/blackfriday v1.5.2
	golang.org/x/oauth2 => github.com/sourcegraph/oauth2 v0.0.0-20210825125341-77c1d99ece3c
)
