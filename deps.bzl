"Bazel go dependencies"

load("@bazel_gazelle//:deps.bzl", "go_repository")

def go_dependencies():
    """The go dependencies in this macro are auto-updated by gazelle

    To update run,

        bazel run //:gazelle-update-repos
    """
    go_repository(
        name = "build_buf_gen_go_bufbuild_protovalidate_protocolbuffers_go",
        build_file_proto_mode = "disable_global",
        importpath = "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go",
        sum = "h1:tdpHgTbmbvEIARu+bixzmleMi14+3imnpoFXz+Qzjp4=",
        version = "v1.31.0-20230802163732-1c33ebd9ecfa.1",
    )
    go_repository(
        name = "cc_mvdan_gofumpt",
        build_file_proto_mode = "disable_global",
        importpath = "mvdan.cc/gofumpt",
        sum = "h1:JVf4NN1mIpHogBj7ABpgOyZc65/UUOkKQFkoURsz4MM=",
        version = "v0.4.0",
    )
    go_repository(
        name = "cc_mvdan_xurls_v2",
        build_file_proto_mode = "disable_global",
        importpath = "mvdan.cc/xurls/v2",
        sum = "h1:tzxjVAj+wSBmDcF6zBB7/myTy3gX9xvi8Tyr28AuQgc=",
        version = "v2.4.0",
    )
    go_repository(
        name = "co_honnef_go_tools",
        build_file_proto_mode = "disable_global",
        importpath = "honnef.co/go/tools",
        sum = "h1:UoveltGrhghAA7ePc+e+QYDHXrBps2PqFZiHkGR/xK8=",
        version = "v0.0.1-2020.1.4",
    )
    go_repository(
        name = "com_gitea_go_chi_binding",
        build_file_proto_mode = "disable_global",
        importpath = "gitea.com/go-chi/binding",
        sum = "h1:MMSPgnVULVwV9kEBgvyEUhC9v/uviZ55hPJEMjpbNR4=",
        version = "v0.0.0-20221013104517-b29891619681",
    )
    go_repository(
        name = "com_gitea_go_chi_cache",
        build_file_proto_mode = "disable_global",
        importpath = "gitea.com/go-chi/cache",
        sum = "h1:E0npuTfDW6CT1yD8NMDVc1SK6IeRjfmRL2zlEsCEd7w=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_gitea_go_chi_captcha",
        build_file_proto_mode = "disable_global",
        importpath = "gitea.com/go-chi/captcha",
        sum = "h1:J/1i8u40TbcLP/w2w2KCkgW2PelIqYkD5UOwu8IOvVU=",
        version = "v0.0.0-20211013065431-70641c1a35d5",
    )
    go_repository(
        name = "com_gitea_go_chi_session",
        build_file_proto_mode = "disable_global",
        importpath = "gitea.com/go-chi/session",
        sum = "h1:tJQRXgZigkLeeW9LPlps9G9aMoE6LAmqigLA+wxmd1Q=",
        version = "v0.0.0-20211218221615-e3605d8b28b8",
    )
    go_repository(
        name = "com_gitea_lunny_dingtalk_webhook",
        build_file_proto_mode = "disable_global",
        importpath = "gitea.com/lunny/dingtalk_webhook",
        sum = "h1:+wWBi6Qfruqu7xJgjOIrKVQGiLUZdpKYCZewJ4clqhw=",
        version = "v0.0.0-20171025031554-e3534c89ef96",
    )
    go_repository(
        name = "com_gitea_lunny_levelqueue",
        build_file_proto_mode = "disable_global",
        importpath = "gitea.com/lunny/levelqueue",
        sum = "h1:Zc3RQWC2xOVglLciQH/ZIC5IqSk3Jn96LflGQLv18Rg=",
        version = "v0.4.2-0.20220729054728-f020868cc2f7",
    )
    go_repository(
        name = "com_github_42wim_sshsig",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/42wim/sshsig",
        sum = "h1:r3qt8PCHnfjOv9PN3H+XXKmDA1dfFMIN1AislhlA/ps=",
        version = "v0.0.0-20211121163825-841cf5bbc121",
    )
    go_repository(
        name = "com_github_99designs_gqlgen",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/99designs/gqlgen",
        sum = "h1:yczvlwMsfcVu/JtejqfrLwXuSP0yZFhmcss3caEvHw8=",
        version = "v0.17.2",
    )
    go_repository(
        name = "com_github_acomagu_bufpipe",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/acomagu/bufpipe",
        sum = "h1:e3H4WUzM3npvo5uv95QuJM3cQspFNtFBzvJ2oNjKIDQ=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_adalogics_go_fuzz_headers",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/AdaLogics/go-fuzz-headers",
        sum = "h1:bvDV9vkmnHYOMsOr4WLk+Vo07yKIzd94sVoIqshQ4bU=",
        version = "v0.0.0-20230811130428-ced1acdcaa24",
    )
    go_repository(
        name = "com_github_agext_levenshtein",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/agext/levenshtein",
        sum = "h1:YB2fHEn0UJagG8T1rrWknE3ZQzWM06O8AMAatNn7lmo=",
        version = "v1.2.3",
    )
    go_repository(
        name = "com_github_agnivade_levenshtein",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/agnivade/levenshtein",
        sum = "h1:QY8M92nrzkmr798gCo3kmMyqXFzdQVpxLlGPRBij0P8=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_ajstarks_svgo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ajstarks/svgo",
        sum = "h1:slYM766cy2nI3BwyRiyQj/Ud48djTMtMebDqepE95rw=",
        version = "v0.0.0-20211024235047-1546f124cd8b",
    )
    go_repository(
        name = "com_github_alecthomas_assert_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alecthomas/assert/v2",
        sum = "h1:f6L/b7KE2bfA+9O4FL3CM/xJccDEwPVYd5fALBiuwvw=",
        version = "v2.2.0",
    )
    go_repository(
        name = "com_github_alecthomas_chroma",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alecthomas/chroma",
        sum = "h1:7XDcGkCQopCNKjZHfYrNLraA+M7e0fMiJ/Mfikbfjek=",
        version = "v0.10.0",
    )
    go_repository(
        name = "com_github_alecthomas_chroma_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alecthomas/chroma/v2",
        sum = "h1:Loe2ZjT5x3q1bcWwemqyqEi8p11/IV/ncFCeLYDpWC4=",
        version = "v2.4.0",
    )
    go_repository(
        name = "com_github_alecthomas_kingpin",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alecthomas/kingpin",
        sum = "h1:5svnBTFgJjZvGKyYBtMB0+m5wvrbUHiqye8wRJMlnYI=",
        version = "v2.2.6+incompatible",
    )
    go_repository(
        name = "com_github_alecthomas_repr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alecthomas/repr",
        sum = "h1:ENn2e1+J3k09gyj2shc0dHr/yjaWSHRlrJ4DPMevDqE=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_alecthomas_template",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alecthomas/template",
        sum = "h1:JYp7IbQjafoB+tBA3gMyHYHrpOtNuDiK/uB5uXxq5wM=",
        version = "v0.0.0-20190718012654-fb15b899a751",
    )
    go_repository(
        name = "com_github_alecthomas_units",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alecthomas/units",
        sum = "h1:s6gZFSlWYmbqAuRjVTiNNhvNRfY2Wxp9nhfyel4rklc=",
        version = "v0.0.0-20211218093645-b94a6e3cc137",
    )
    go_repository(
        name = "com_github_alexflint_go_arg",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alexflint/go-arg",
        sum = "h1:lDWZAXxpAnZUq4qwb86p/3rIJJ2Li81EoMbTMujhVa0=",
        version = "v1.4.2",
    )
    go_repository(
        name = "com_github_alexflint_go_scalar",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alexflint/go-scalar",
        sum = "h1:NGupf1XV/Xb04wXskDFzS0KWOLH632W/EO4fAFi+A70=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_amit7itz_goset",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/amit7itz/goset",
        sum = "h1:LTn5swPM1a0vFr3yluIQHjvNTfbp7z87baV9x2ugS+4=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_andres_erbsen_clock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/andres-erbsen/clock",
        sum = "h1:MzBOUgng9orim59UnfUTLRjMpd09C5uEVQ6RPGeCaVI=",
        version = "v0.0.0-20160526145045-9e14626cd129",
    )
    go_repository(
        name = "com_github_andreyvit_diff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/andreyvit/diff",
        sum = "h1:bvNMNQO63//z+xNgfBlViaCIJKLlCJ6/fmUseuG0wVQ=",
        version = "v0.0.0-20170406064948-c7f18ee00883",
    )
    go_repository(
        name = "com_github_andybalholm_brotli",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/andybalholm/brotli",
        sum = "h1:8uQZIdzKmjc/iuPu7O2ioW48L81FgatrcpfFmiq/cCs=",
        version = "v1.0.5",
    )
    go_repository(
        name = "com_github_andybalholm_cascadia",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/andybalholm/cascadia",
        sum = "h1:nhxRkql1kdYCc8Snf7D5/D3spOX+dBgjA6u8x004T2c=",
        version = "v1.3.1",
    )
    go_repository(
        name = "com_github_andygrunwald_go_gerrit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/andygrunwald/go-gerrit",
        sum = "h1:q9HI3vudtbNNvaZl+l0oM7cQ07OES2x7ysiVwZpk89E=",
        version = "v0.0.0-20230628115649-c44fe2fbf2ca",
    )
    go_repository(
        name = "com_github_anmitsu_go_shlex",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/anmitsu/go-shlex",
        sum = "h1:9AeTilPcZAjCFIImctFaOjnTIavg87rW78vTPkQqLI8=",
        version = "v0.0.0-20200514113438-38f4b401e2be",
    )
    go_repository(
        name = "com_github_antihax_optional",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/antihax/optional",
        sum = "h1:xK2lYat7ZLaVVcIuj82J8kIro4V6kDe0AUDFboUCwcg=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_antlr_antlr4_runtime_go_antlr_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/antlr/antlr4/runtime/Go/antlr/v4",
        sum = "h1:goHVqTbFX3AIo0tzGr14pgfAW2ZfPChKO21Z9MGf/gk=",
        version = "v4.0.0-20230512164433-5d1fd1a340c9",
    )
    go_repository(
        name = "com_github_antonmedv_expr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/antonmedv/expr",
        sum = "h1:uzMxTbpHpOqV20RrNvBKHGojNwdRpcrgoFtgF4J8xtg=",
        version = "v1.10.5",
    )
    go_repository(
        name = "com_github_aokoli_goutils",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aokoli/goutils",
        sum = "h1:7fpzNGoJ3VA8qcrm++XEE1QUe0mIwNeLa02Nwq7RDkg=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_apache_arrow_go_v12",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/apache/arrow/go/v12",
        sum = "h1:xtZE63VWl7qLdB0JObIXvvhGjoVNrQ9ciIHG2OK5cmc=",
        version = "v12.0.0",
    )
    go_repository(
        name = "com_github_apache_thrift",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/apache/thrift",
        sum = "h1:qEy6UW60iVOlUy+b9ZR0d5WzUWYGOo4HfopoyBaNmoY=",
        version = "v0.16.0",
    )
    go_repository(
        name = "com_github_apparentlymart_go_versions",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/apparentlymart/go-versions",
        sum = "h1:ECIpSn0adcYNsBfSRwdDdz9fWlL+S/6EUd9+irwkBgU=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_arbovm_levenshtein",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/arbovm/levenshtein",
        sum = "h1:jfIu9sQUG6Ig+0+Ap1h4unLjW6YQJpKZVmUzxsD4E/Q=",
        version = "v0.0.0-20160628152529-48b4e1c0c4d0",
    )
    go_repository(
        name = "com_github_armon_circbuf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/armon/circbuf",
        sum = "h1:7Ip0wMmLHLRJdrloDxZfhMm0xrLXZS8+COSu2bXmEQs=",
        version = "v0.0.0-20190214190532-5111143e8da2",
    )
    go_repository(
        name = "com_github_armon_consul_api",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/armon/consul-api",
        sum = "h1:G1bPvciwNyF7IUmKXNt9Ak3m6u9DE1rF+RmtIkBpVdA=",
        version = "v0.0.0-20180202201655-eb2c6b5be1b6",
    )
    go_repository(
        name = "com_github_armon_go_metrics",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/armon/go-metrics",
        sum = "h1:FR+drcQStOe+32sYyJYyZ7FIdgoGGBnwLl+flodp8Uo=",
        version = "v0.3.10",
    )
    go_repository(
        name = "com_github_armon_go_radix",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/armon/go-radix",
        sum = "h1:F4z6KzEeeQIMeLFa97iZU6vupzoecKdU5TX24SNppXI=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_armon_go_socks5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/armon/go-socks5",
        sum = "h1:0CwZNZbxp69SHPdPJAN/hZIm0C4OItdklCFmMRWYpio=",
        version = "v0.0.0-20160902184237-e75332964ef5",
    )
    go_repository(
        name = "com_github_asaskevich_govalidator",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/asaskevich/govalidator",
        sum = "h1:Byv0BzEl3/e6D5CLfI0j/7hiIEtvGVFPCZ7Ei2oq8iQ=",
        version = "v0.0.0-20210307081110-f21760c49a8d",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go",
        sum = "h1:X34pX5t0LIZXjBY11yf9JKMP3c1aZgirh+5PjtaZyJ4=",
        version = "v1.44.128",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2",
        sum = "h1:wyC6p9Yfq6V2y98wfDsj6OnNQa4w2BLGCLIxzNhwOGY=",
        version = "v1.17.4",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_aws_protocol_eventstream",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream",
        sum = "h1:SdK4Ppk5IzLs64ZMvr6MrSficMtjY2oS0WOORXTlxwU=",
        version = "v1.4.1",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_config",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/config",
        sum = "h1:fKs/I4wccmfrNRO9rdrbMO1NgLxct6H9rNMiPdBxHWw=",
        version = "v1.18.12",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_credentials",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/credentials",
        sum = "h1:Cb+HhuEnV19zHRaYYVglwvdHGMJWbdsyP4oHhw04xws=",
        version = "v1.13.12",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_feature_ec2_imds",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/feature/ec2/imds",
        sum = "h1:3aMfcTmoXtTZnaT86QlVaYh+BRMbvrrmZwIQ5jWqCZQ=",
        version = "v1.12.22",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_feature_s3_manager",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/feature/s3/manager",
        sum = "h1:JL7cY85hyjlgfA29MMyAlItX+JYIH9XsxgMBS7jtlqA=",
        version = "v1.11.10",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_internal_configsources",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/internal/configsources",
        sum = "h1:r+XwaCLpIvCKjBIYy/HVZujQS9tsz5ohHG3ZIe0wKoE=",
        version = "v1.1.28",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_internal_endpoints_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/internal/endpoints/v2",
        sum = "h1:7AwGYXDdqRQYsluvKFmWoqpcOQJ4bH634SkYf3FNj/A=",
        version = "v2.4.22",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_internal_ini",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/internal/ini",
        sum = "h1:J4xhFd6zHhdF9jPP0FQJ6WknzBboGMBNjKOv4iTuw4A=",
        version = "v1.3.29",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_internal_v4a",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/internal/v4a",
        sum = "h1:C21IDZCm9Yu5xqjb3fKmxDoYvJXtw1DNlOmLZEIlY1M=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_appconfig",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/appconfig",
        sum = "h1:5fez51yE//mtmaEkh9JTAcLl4xg60Ha86pE+FIqinGc=",
        version = "v1.4.2",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_cloudwatch",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/cloudwatch",
        sum = "h1:5WstmcviZ9X/h5nORkGT4akyLmWjrLxE9s8oKkFhkD4=",
        version = "v1.15.0",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_codecommit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/codecommit",
        sum = "h1:iEqboXubLCABYFoClUyX/Bv8DfhmV39hPKdRbs21/kI=",
        version = "v1.11.0",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_internal_accept_encoding",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding",
        sum = "h1:T4pFel53bkHjL2mMo+4DKE6r6AuoZnM0fg7k1/ratr4=",
        version = "v1.9.1",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_internal_checksum",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/internal/checksum",
        sum = "h1:9LSZqt4v1JiehyZTrQnRFf2mY/awmyYNNY/b7zqtduU=",
        version = "v1.1.5",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_internal_presigned_url",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/internal/presigned-url",
        sum = "h1:LjFQf8hFuMO22HkV5VWGLBvmCLBCLPivUAmpdpnp4Vs=",
        version = "v1.9.22",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_internal_s3shared",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/internal/s3shared",
        sum = "h1:RE/DlZLYrz1OOmq8F28IXHLksuuvlpzUbvJ+SESCZBI=",
        version = "v1.13.4",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_kms",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/kms",
        sum = "h1:A8FMqkP+OlnSiVY+2QakwqW0fAGnE18TqPig/T7aJU0=",
        version = "v1.14.0",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_s3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/s3",
        sum = "h1:LCQKnopq2t4oQS3VKivlYTzAHCTJZZoQICM9fny7KHY=",
        version = "v1.26.9",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_sso",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/sso",
        sum = "h1:lQKN/LNa3qqu2cDOQZybP7oL4nMGGiFqob0jZJaR8/4=",
        version = "v1.12.1",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_sts",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/sts",
        sum = "h1:s49mSnsBZEXjfGBkRfmK+nPqzT7Lt3+t2SmAKNyHblw=",
        version = "v1.18.3",
    )
    go_repository(
        name = "com_github_aws_constructs_go_constructs_v10",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/constructs-go/constructs/v10",
        sum = "h1:XW89VuKlwwzgU77Nyzj30SPn2SFtDYkzfF+QN84b5bQ=",
        version = "v10.2.69",
    )
    go_repository(
        name = "com_github_aws_jsii_runtime_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/jsii-runtime-go",
        sum = "h1:ZvOKkGKQBZC8qrlM3qi2hXnbnqI64o3WtVw5ZVd/q9s=",
        version = "v1.84.0",
    )
    go_repository(
        name = "com_github_aws_smithy_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/smithy-go",
        sum = "h1:hgz0X/DX0dGqTYpGALqXJoRKRj5oQ7150i5FdTePzO8=",
        version = "v1.13.5",
    )
    go_repository(
        name = "com_github_aybabtme_iocontrol",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aybabtme/iocontrol",
        sum = "h1:0NmehRCgyk5rljDQLKUO+cRJCnduDyn11+zGZIc9Z48=",
        version = "v0.0.0-20150809002002-ad15bcfc95a0",
    )
    go_repository(
        name = "com_github_aymerick_douceur",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aymerick/douceur",
        sum = "h1:Mv+mAeH1Q+n9Fr+oyamOlAkUNPWPlA8PPGR0QAaYuPk=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go",
        sum = "h1:HzKLt3kIwMm4KeJYTdx9EbjRYTySD/t8i1Ee/W5EGXw=",
        version = "v65.0.0+incompatible",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_ai_azopenai",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai",
        sum = "h1:x7fb22Q43h2DRFCvp9rAua8PoV3gwtl1bK5+pihnihA=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_azcore",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/azcore",
        sum = "h1:9kDVnTz3vbfweTqAUmk/a/pH5pWFCHtvRpHYC0G/dcA=",
        version = "v1.8.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_azidentity",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/azidentity",
        sum = "h1:BMAjVKJM0U/CYF27gA0ZMmXGkOcvfFtD0oHVZ1TIPRI=",
        version = "v1.4.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_internal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/internal",
        sum = "h1:sXr+ck84g/ZlZUOZiNELInmMgOsuGwdjjVkEIde0OtY=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_storage_azblob",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob",
        sum = "h1:QSdcrd/UFJv6Bp/CfoVf2SrENpFn9P6Yh8yb+xNhYMM=",
        version = "v0.4.1",
    )
    go_repository(
        name = "com_github_azure_go_ansiterm",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-ansiterm",
        sum = "h1:UQHMgLO+TxOElx5B5HZ4hJQsoJ/PvUvKRhJHDQXO8P8=",
        version = "v0.0.0-20210617225240-d185dfc1b5a1",
    )
    go_repository(
        name = "com_github_azure_go_autorest",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest",
        sum = "h1:V5VMDjClD3GiElqLWO7mz2MxNAK/vTfRHdAubSIPRgs=",
        version = "v14.2.0+incompatible",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest",
        sum = "h1:ndAExarwr5Y+GaHE6VCaY1kyS/HwwGGyuimVhWsHOEM=",
        version = "v0.11.28",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_adal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/adal",
        sum = "h1:jjQnVFXPfekaqb8vIsv2G1lxshoW+oGv4MDlhRtnYZk=",
        version = "v0.9.21",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_date",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/date",
        sum = "h1:7gUk1U5M/CQbp9WoqinNzJar+8KY+LPI6wiWrP/myHw=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_mocks",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/mocks",
        sum = "h1:K0laFcLE6VLTOwNgSxaGbUcLPuGXlNkbVvq4cW4nIHk=",
        version = "v0.4.1",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_to",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/to",
        sum = "h1:oXVqrxakqqV1UZdSazDOPOLvOIz+XA683u8EctwboHk=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_validation",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/validation",
        sum = "h1:AgyqjAd94fwNAoTjl/WQXg4VvFeRFpO+UhNyRXqF1ac=",
        version = "v0.3.1",
    )
    go_repository(
        name = "com_github_azure_go_autorest_logger",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/logger",
        sum = "h1:IG7i4p/mDa2Ce4TRyAO8IHnVhAVF3RFU+ZtXWSmf4Tg=",
        version = "v0.2.1",
    )
    go_repository(
        name = "com_github_azure_go_autorest_tracing",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/tracing",
        sum = "h1:TYi4+3m5t6K48TGI9AUdb+IzbnSxvnvUMfuitfgcfuo=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_azure_go_ntlmssp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-ntlmssp",
        sum = "h1:NeAW1fUYUEWhft7pkxDf6WoUvEZJ/uOKsvtpjLnn8MU=",
        version = "v0.0.0-20220621081337-cb9428e4ac1e",
    )
    go_repository(
        name = "com_github_azuread_microsoft_authentication_library_for_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/AzureAD/microsoft-authentication-library-for-go",
        sum = "h1:WpB/QDNLpMw72xHJc34BNNykqSOeEJDAWkhf0u12/Jk=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_bahlo_generic_list_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bahlo/generic-list-go",
        sum = "h1:5sz/EEAK+ls5wF+NeqDpk5+iNdMDXrh3z3nPnH1Wvgk=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_becheran_wildmatch_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/becheran/wildmatch-go",
        sum = "h1:mE3dGGkTmpKtT4Z+88t8RStG40yN9T+kFEGj2PZFSzA=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_beevik_etree",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/beevik/etree",
        sum = "h1:T0xke/WvNtMoCqgzPhkX2r4rjY3GDZFi+FjpRZY2Jbs=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_benbjohnson_clock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/benbjohnson/clock",
        sum = "h1:ip6w0uFQkncKQ979AypyG0ER7mqUSBdKLOgAle/AT8A=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_beorn7_perks",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/beorn7/perks",
        sum = "h1:VlbKKnNfV8bJzeqoa4cOKqO6bYr3WgKZxO8Z16+hsOM=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_bgentry_speakeasy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bgentry/speakeasy",
        sum = "h1:ByYyxL9InA1OWqxJqqp2A5pYHUrCiAL6K3J+LKSsQkY=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_bitly_go_simplejson",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bitly/go-simplejson",
        sum = "h1:6IH+V8/tVMab511d5bn4M7EwGXZf9Hj6i2xSwkNEM+Y=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_bits_and_blooms_bitset",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bits-and-blooms/bitset",
        sum = "h1:FD+XqgOZDUxxZ8hzoBFuV9+cGWY9CslN6d5MS5JVb4c=",
        version = "v1.8.0",
    )
    go_repository(
        name = "com_github_bketelsen_crypt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bketelsen/crypt",
        sum = "h1:w/jqZtC9YD4DS/Vp9GhWfWcCpuAL58oTnLoI8vE9YHU=",
        version = "v0.0.4",
    )
    go_repository(
        name = "com_github_blevesearch_bleve_index_api",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/bleve_index_api",
        sum = "h1:mtlzsyJjMIlDngqqB1mq8kPryUMIuEVVbRbJHOWEexU=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_blevesearch_bleve_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/bleve/v2",
        sum = "h1:1wuR7eB8Fk9UaCaBUfnQt5V7zIpi4VDok9ExN7Rl+/8=",
        version = "v2.3.5",
    )
    go_repository(
        name = "com_github_blevesearch_geo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/geo",
        sum = "h1:0NybEduqE5fduFRYiUKF0uqybAIFKXYjkBdXKYn7oA4=",
        version = "v0.1.15",
    )
    go_repository(
        name = "com_github_blevesearch_go_porterstemmer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/go-porterstemmer",
        sum = "h1:GtmsqID0aZdCSNiY8SkuPJ12pD4jI+DdXTAn4YRcHCo=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_blevesearch_gtreap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/gtreap",
        sum = "h1:2JWigFrzDMR+42WGIN/V2p0cUvn4UP3C4Q5nmaZGW8Y=",
        version = "v0.1.1",
    )
    go_repository(
        name = "com_github_blevesearch_mmap_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/mmap-go",
        sum = "h1:OVhDhT5B/M1HNPpYPBKIEJaD0F3Si+CrEKULGCDPWmc=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_blevesearch_scorch_segment_api_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/scorch_segment_api/v2",
        sum = "h1:2UzpR2dR5DvSZk8tVJkcQ7D5xhoK/UBelYw8ttBHrRQ=",
        version = "v2.1.3",
    )
    go_repository(
        name = "com_github_blevesearch_segment",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/segment",
        sum = "h1:5lG7yBCx98or7gK2cHMKPukPZ/31Kag7nONpoBt22Ac=",
        version = "v0.9.0",
    )
    go_repository(
        name = "com_github_blevesearch_snowballstem",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/snowballstem",
        sum = "h1:lMQ189YspGP6sXvZQ4WZ+MLawfV8wOmPoD/iWeNXm8s=",
        version = "v0.9.0",
    )
    go_repository(
        name = "com_github_blevesearch_upsidedown_store_api",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/upsidedown_store_api",
        sum = "h1:1SYRwyoFLwG3sj0ed89RLtM15amfX2pXlYbFOnF8zNU=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_blevesearch_vellum",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/vellum",
        sum = "h1:PL+NWVk3dDGPCV0hoDu9XLLJgqU4E5s/dOeEJByQ2uQ=",
        version = "v1.0.9",
    )
    go_repository(
        name = "com_github_blevesearch_zapx_v11",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/zapx/v11",
        sum = "h1:50jET4HUJ6eCqGxdhUt+mjybMvEX2MWyqLGtCx3yUgc=",
        version = "v11.3.6",
    )
    go_repository(
        name = "com_github_blevesearch_zapx_v12",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/zapx/v12",
        sum = "h1:G304NHBLgQeZ+IHK/XRCM0nhHqAts8MEvHI6LhoDNM4=",
        version = "v12.3.6",
    )
    go_repository(
        name = "com_github_blevesearch_zapx_v13",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/zapx/v13",
        sum = "h1:vavltQHNdjQezhLZs5nIakf+w/uOa1oqZxB58Jy/3Ig=",
        version = "v13.3.6",
    )
    go_repository(
        name = "com_github_blevesearch_zapx_v14",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/zapx/v14",
        sum = "h1:b9lub7TvcwUyJxK/cQtnN79abngKxsI7zMZnICU0WhE=",
        version = "v14.3.6",
    )
    go_repository(
        name = "com_github_blevesearch_zapx_v15",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/zapx/v15",
        sum = "h1:VSswg/ysDxHgitcNkpUNtaTYS4j3uItpXWLAASphl6k=",
        version = "v15.3.6",
    )
    go_repository(
        name = "com_github_bmatcuk_doublestar",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bmatcuk/doublestar",
        sum = "h1:gPypJ5xD31uhX6Tf54sDPUOBXTqKH4c9aPY66CyQrS0=",
        version = "v1.3.4",
    )
    go_repository(
        name = "com_github_boj_redistore",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/boj/redistore",
        sum = "h1:RmdPFa+slIr4SCBg4st/l/vZWVe9QJKMXGO60Bxbe04=",
        version = "v0.0.0-20180917114910-cd5dcc76aeff",
    )
    go_repository(
        name = "com_github_boombuler_barcode",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/boombuler/barcode",
        sum = "h1:NDBbPmhS+EqABEs5Kg3n/5ZNjy73Pz7SIV+KCeqyXcs=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_bradfitz_gomemcache",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bradfitz/gomemcache",
        sum = "h1:L/QXpzIa3pOvUGt1D1lA5KjYhPBAN/3iWdP7xeFS9F0=",
        version = "v0.0.0-20190913173617-a41fca850d0b",
    )
    go_repository(
        name = "com_github_bradleyjkemp_cupaloy_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bradleyjkemp/cupaloy/v2",
        sum = "h1:knToPYa2xtfg42U3I6punFEjaGFKWQRXJwj0JTv4mTs=",
        version = "v2.6.0",
    )
    go_repository(
        name = "com_github_bshuster_repo_logrus_logstash_hook",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bshuster-repo/logrus-logstash-hook",
        sum = "h1:e+C0SB5R1pu//O4MQ3f9cFuPGoOVeF2fE4Og9otCc70=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_bsm_ginkgo_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bsm/ginkgo/v2",
        sum = "h1:ItPMPH90RbmZJt5GtkcNvIRuGEdwlBItdNVoyzaNQao=",
        version = "v2.7.0",
    )
    go_repository(
        name = "com_github_bsm_gomega",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bsm/gomega",
        sum = "h1:LhQm+AFcgV2M0WyKroMASzAzCAJVpAxQXv4SaI9a69Y=",
        version = "v1.26.0",
    )
    go_repository(
        name = "com_github_bufbuild_buf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bufbuild/buf",
        sum = "h1:GqE3a8CMmcFvWPzuY3Mahf9Kf3S9XgZ/ORpfYFzO+90=",
        version = "v1.4.0",
    )
    go_repository(
        name = "com_github_bufbuild_protovalidate_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bufbuild/protovalidate-go",
        sum = "h1:pJr07sYhliyfj/STAM7hU4J3FKpVeLVKvOBmOTN8j+s=",
        version = "v0.2.1",
    )
    go_repository(
        name = "com_github_buger_jsonparser",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/buger/jsonparser",
        sum = "h1:2PnMjfWD7wBILjqQbt530v576A/cAbQvEW9gGIpYMUs=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_bugsnag_bugsnag_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bugsnag/bugsnag-go",
        sum = "h1:rFt+Y/IK1aEZkEHchZRSq9OQbsSzIT/OrI8YFFmRIng=",
        version = "v0.0.0-20141110184014-b1d153021fcd",
    )
    go_repository(
        name = "com_github_bugsnag_osext",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bugsnag/osext",
        sum = "h1:otBG+dV+YK+Soembjv71DPz3uX/V/6MMlSyD9JBQ6kQ=",
        version = "v0.0.0-20130617224835-0dd3f918b21b",
    )
    go_repository(
        name = "com_github_bugsnag_panicwrap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bugsnag/panicwrap",
        sum = "h1:nvj0OLI3YqYXer/kZD8Ri1aaunCxIEsOst1BVJswV0o=",
        version = "v0.0.0-20151223152923-e2c28503fcd0",
    )
    go_repository(
        name = "com_github_buildkite_go_buildkite_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/buildkite/go-buildkite/v3",
        sum = "h1:5kX1fFDj3Co7cP6cqZKuW1VoCJz3u4cOx6wfdCeM4ZA=",
        version = "v3.0.1",
    )
    go_repository(
        name = "com_github_buildkite_terminal_to_html_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/buildkite/terminal-to-html/v3",
        sum = "h1:chdLUSpiOj/A4v3dzxyOqixXI6aw7IDA6Dk77FXsvNU=",
        version = "v3.7.0",
    )
    go_repository(
        name = "com_github_burntsushi_toml",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/BurntSushi/toml",
        sum = "h1:o7IhLm0Msx3BaB+n3Ag7L8EVlByGnpq14C4YWiu/gL8=",
        version = "v1.3.2",
    )
    go_repository(
        name = "com_github_burntsushi_xgb",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/BurntSushi/xgb",
        sum = "h1:1BDTz0u9nC3//pOCMdNH+CiXJVYJh5UQNCOBG7jbELc=",
        version = "v0.0.0-20160522181843-27f122750802",
    )
    go_repository(
        name = "com_github_bwesterb_go_ristretto",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bwesterb/go-ristretto",
        sum = "h1:1w53tCkGhCQ5djbat3+MH0BAQ5Kfgbt56UZQ/JMzngw=",
        version = "v1.2.3",
    )
    go_repository(
        name = "com_github_c2h5oh_datasize",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/c2h5oh/datasize",
        sum = "h1:6+ZFm0flnudZzdSE0JxlhR2hKnGPcNB35BjQf4RYQDY=",
        version = "v0.0.0-20220606134207-859f65c6625b",
    )
    go_repository(
        name = "com_github_caddyserver_certmagic",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/caddyserver/certmagic",
        sum = "h1:o30seC1T/dBqBCNNGNHWwj2i5/I/FMjBbTAhjADP3nE=",
        version = "v0.17.2",
    )
    go_repository(
        name = "com_github_cenkalti_backoff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cenkalti/backoff",
        sum = "h1:tNowT99t7UNflLxfYYSlKYsBpXdEet03Pg2g16Swow4=",
        version = "v2.2.1+incompatible",
    )
    go_repository(
        name = "com_github_cenkalti_backoff_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cenkalti/backoff/v4",
        sum = "h1:y4OZtCnogmCPw98Zjyt5a6+QwPLGkiQsYW5oUqylYbM=",
        version = "v4.2.1",
    )
    go_repository(
        name = "com_github_census_instrumentation_opencensus_proto",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/census-instrumentation/opencensus-proto",
        sum = "h1:iKLQ0xPNFxR/2hzXZMrBo8f1j86j5WHzznCCQxV/b8g=",
        version = "v0.4.1",
    )
    go_repository(
        name = "com_github_cespare_xxhash",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cespare/xxhash",
        sum = "h1:a6HrQnmkObjyL+Gs60czilIUGqrzKutQD6XZog3p+ko=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_cespare_xxhash_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cespare/xxhash/v2",
        sum = "h1:DC2CZ1Ep5Y4k3ZQ899DldepgrayRUGE6BBZ/cd9Cj44=",
        version = "v2.2.0",
    )
    go_repository(
        name = "com_github_charmbracelet_glamour",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/charmbracelet/glamour",
        sum = "h1:wu15ykPdB7X6chxugG/NNfDUbyyrCLV9XBalj5wdu3g=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_chi_middleware_proxy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/chi-middleware/proxy",
        sum = "h1:4HaXUp8o2+bhHr1OhVy+VjN0+L7/07JDcn6v7YrTjrQ=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_chromedp_cdproto",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/chromedp/cdproto",
        sum = "h1:lg5k1KAxmknil6Z19LaaeiEs5Pje7hPzRfyWSSnWLP0=",
        version = "v0.0.0-20210706234513-2bc298e8be7f",
    )
    go_repository(
        name = "com_github_chromedp_chromedp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/chromedp/chromedp",
        sum = "h1:FvgJICfjvXtDX+miuMUY0NHuY8zQvjS/TcEQEG6Ldzs=",
        version = "v0.7.3",
    )
    go_repository(
        name = "com_github_chromedp_sysutil",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/chromedp/sysutil",
        sum = "h1:+ZxhTpfpZlmchB58ih/LBHX52ky7w2VhQVKQMucy3Ic=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_chzyer_logex",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/chzyer/logex",
        sum = "h1:Swpa1K6QvQznwJRcfTfQJmTE72DqScAa40E+fbHEXEE=",
        version = "v1.1.10",
    )
    go_repository(
        name = "com_github_chzyer_readline",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/chzyer/readline",
        sum = "h1:upd/6fQk4src78LMRzh5vItIt361/o4uq553V8B5sGI=",
        version = "v1.5.1",
    )
    go_repository(
        name = "com_github_chzyer_test",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/chzyer/test",
        sum = "h1:q763qf9huN11kDQavWsoZXJNW3xEE4JJyHa5Q25/sd8=",
        version = "v0.0.0-20180213035817-a1ea475d72b1",
    )
    go_repository(
        name = "com_github_client9_misspell",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/client9/misspell",
        sum = "h1:ta993UF76GwbvJcIo3Y68y/M3WxlpEHPWIGDkJYwzJI=",
        version = "v0.3.4",
    )
    go_repository(
        name = "com_github_cloudflare_cfssl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cloudflare/cfssl",
        sum = "h1:aIOUjpeuDJOpWjVJFP2ByplF53OgqG8I1S40Ggdlk3g=",
        version = "v1.6.1",
    )
    go_repository(
        name = "com_github_cloudflare_circl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cloudflare/circl",
        patches = [
            "//third_party/com_github_cloudflare_circl:math_fp25519_BUILD_bazel.patch",  # keep
            "//third_party/com_github_cloudflare_circl:math_fp448_BUILD_bazel.patch",  # keep
            "//third_party/com_github_cloudflare_circl:dh_x25519_BUILD_bazel.patch",  # keep
            "//third_party/com_github_cloudflare_circl:dh_x448_BUILD_bazel.patch",  # keep
        ],
        sum = "h1:fE/Qz0QdIGqeWfnwq0RE0R7MI51s0M2E4Ga9kq5AEMs=",
        version = "v1.3.3",
    )
    go_repository(
        name = "com_github_cloudykit_fastprinter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/CloudyKit/fastprinter",
        sum = "h1:sR+/8Yb4slttB4vD+b9btVEnWgL3Q00OBTzVT8B9C0c=",
        version = "v0.0.0-20200109182630-33d98a066a53",
    )
    go_repository(
        name = "com_github_cloudykit_jet_v6",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/CloudyKit/jet/v6",
        sum = "h1:EpcZ6SR9n28BUGtNJSvlBqf90IpjeFr36Tizxhn/oME=",
        version = "v6.2.0",
    )
    go_repository(
        name = "com_github_cncf_udpa_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cncf/udpa/go",
        sum = "h1:QQ3GSy+MqSHxm/d8nCtnAiZdYFd45cYZPs8vOOIYKfk=",
        version = "v0.0.0-20220112060539-c52dc94e7fbe",
    )
    go_repository(
        name = "com_github_cncf_xds_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cncf/xds/go",
        sum = "h1:/inchEIKaYC1Akx+H+gqO04wryn5h75LSazbRlnya1k=",
        version = "v0.0.0-20230607035331-e9ce68804cb4",
    )
    go_repository(
        name = "com_github_cockroachdb_apd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/apd",
        sum = "h1:3LFP3629v+1aKXU5Q37mxmRxX/pIu1nijXydLShEq5I=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_cockroachdb_apd_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/apd/v2",
        sum = "h1:y1Rh3tEU89D+7Tgbw+lp52T6p/GJLpDmNvr10UWqLTE=",
        version = "v2.0.1",
    )
    go_repository(
        name = "com_github_cockroachdb_datadriven",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/datadriven",
        sum = "h1:H9MtNqVoVhvd9nCBwOyDjUEdZCREqbIdCJD93PBm/jA=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_cockroachdb_errors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/errors",
        sum = "h1:xSEW75zKaKCWzR3OfxXUxgrk/NtT4G1MiOv5lWZazG8=",
        version = "v1.11.1",
    )
    go_repository(
        name = "com_github_cockroachdb_logtags",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/logtags",
        sum = "h1:r6VH0faHjZeQy818SGhaone5OnYfxFR/+AzdY3sf5aE=",
        version = "v0.0.0-20230118201751-21c54148d20b",
    )
    go_repository(
        name = "com_github_cockroachdb_redact",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/redact",
        sum = "h1:u1PMllDkdFfPWaNGMyLD1+so+aq3uUItthCFqzwPJ30=",
        version = "v1.1.5",
    )
    go_repository(
        name = "com_github_codegangsta_inject",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/codegangsta/inject",
        sum = "h1:sDMmm+q/3+BukdIpxwO365v/Rbspp2Nt5XntgQRXq8Q=",
        version = "v0.0.0-20150114235600-33e0aa1cb7c0",
    )
    go_repository(
        name = "com_github_containerd_cgroups",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/cgroups",
        sum = "h1:jN/mbWBEaz+T1pi5OFtnkQ+8qnmEbAr1Oo1FRm5B0dA=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_containerd_console",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/console",
        sum = "h1:lIr7SlA5PxZyMV30bDW0MGbiOPXwc63yRuCP0ARubLw=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_containerd_containerd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/containerd",
        sum = "h1:+itjwpdqXpzHB/QAiWc/BZCjjVfcNgw69w/oIeF4Oy0=",
        version = "v1.6.20",
    )
    go_repository(
        name = "com_github_containerd_continuity",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/continuity",
        sum = "h1:nisirsYROK15TAMVukJOUyGJjz4BNQJBVsNvAXZJ/eg=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_containerd_fifo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/fifo",
        sum = "h1:6PirWBr9/L7GDamKr+XM0IeUFXu5mf3M/BPpH9gaLBU=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_containerd_fuse_overlayfs_snapshotter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/fuse-overlayfs-snapshotter",
        sum = "h1:Xy9Tkx0tk/SsMfLDFc69wzqSrxQHYEFELHBO/Z8XO3M=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_containerd_go_cni",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/go-cni",
        sum = "h1:el5WPymG5nRRLQF1EfB97FWob4Tdc8INg8RZMaXWZlo=",
        version = "v1.1.6",
    )
    go_repository(
        name = "com_github_containerd_go_runc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/go-runc",
        sum = "h1:oU+lLv1ULm5taqgV/CJivypVODI4SUz1znWjv3nNYS0=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_containerd_nydus_snapshotter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/nydus-snapshotter",
        sum = "h1:b8WahTrPkt3XsabjG2o/leN4fw3HWZYr+qxo/Z8Mfzk=",
        version = "v0.3.1",
    )
    go_repository(
        name = "com_github_containerd_stargz_snapshotter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/stargz-snapshotter",
        sum = "h1:3zr1/IkW1aEo6cMYTQeZ4L2jSuCN+F4kgGfjnuowe4U=",
        version = "v0.13.0",
    )
    go_repository(
        name = "com_github_containerd_stargz_snapshotter_estargz",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/stargz-snapshotter/estargz",
        sum = "h1:OqlDCK3ZVUO6C3B/5FSkDwbkEETK84kQgEeFwDC+62k=",
        version = "v0.14.3",
    )
    go_repository(
        name = "com_github_containerd_ttrpc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/ttrpc",
        sum = "h1:NoRHS/z8UiHhpY1w0xcOqoJDGf2DHyzXrF0H4l5AE8c=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_containerd_typeurl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/typeurl",
        sum = "h1:Chlt8zIieDbzQFzXzAeBEF92KhExuE4p9p92/QmY7aY=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_containernetworking_cni",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containernetworking/cni",
        sum = "h1:ky20T7c0MvKvbMOwS/FrlbNwjEoqJEUUYfsL4b0mc4k=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_coreos_bbolt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/bbolt",
        sum = "h1:wZwiHHUieZCquLkDL0B8UhzreNWsPHooDAG3q34zk0s=",
        version = "v1.3.2",
    )
    go_repository(
        name = "com_github_coreos_etcd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/etcd",
        sum = "h1:jFneRYjIvLMLhDLCzuTuU4rSJUjRplcJQ7pD7MnhC04=",
        version = "v3.3.10+incompatible",
    )
    go_repository(
        name = "com_github_coreos_go_iptables",
        # We need to explictly turn this on, because the repository
        # has a script named "build"  at the root which makes tricks
        # gazelle into thinking it already has a Buildfile
        build_file_generation = "on",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/go-iptables",
        sum = "h1:is9qnZMPYjLd8LYqmm/qlE+wwEgJIkTYdhV3rfZo4jk=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_coreos_go_oidc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/go-oidc",
        sum = "h1:mh48q/BqXqgjVHpy2ZY7WnWAbenxRjsz9N1i1YxjHAk=",
        version = "v2.2.1+incompatible",
    )
    go_repository(
        name = "com_github_coreos_go_semver",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/go-semver",
        sum = "h1:wkHLiw0WNATZnSG7epLsujiMCgPAc9xhjJ4tgnAxmfM=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_coreos_go_systemd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/go-systemd",
        sum = "h1:Wf6HqHfScWJN9/ZjdUKyjop4mf3Qdd+1TvvltAvM3m8=",
        version = "v0.0.0-20190321100706-95778dfbb74e",
    )
    go_repository(
        name = "com_github_coreos_go_systemd_v22",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/go-systemd/v22",
        sum = "h1:y9YHcjnjynCd/DVbg5j9L/33jQM3MxJlbj/zWskzfGU=",
        version = "v22.4.0",
    )
    go_repository(
        name = "com_github_coreos_pkg",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/pkg",
        sum = "h1:lBNOc5arjvs8E5mO2tbpBpLoyyu8B6e44T7hJy6potg=",
        version = "v0.0.0-20180928190104-399ea9e2e55f",
    )
    go_repository(
        name = "com_github_couchbase_go_couchbase",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/couchbase/go-couchbase",
        sum = "h1:6fX3IYPArCLhzUcyJS6wezHY8nqSvqUy9NrkMwkXSeg=",
        version = "v0.0.0-20210224140812-5740cd35f448",
    )
    go_repository(
        name = "com_github_couchbase_gomemcached",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/couchbase/gomemcached",
        sum = "h1:GKLSnC6RyTXkJ9vyd0q44doU8rfe34PpKkQ+c1bsWmA=",
        version = "v0.1.2",
    )
    go_repository(
        name = "com_github_couchbase_goutils",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/couchbase/goutils",
        sum = "h1:4KDlx3vjalrHD/EfsjCpV91HNX3JPaIqRtt83zZ7x+Y=",
        version = "v0.0.0-20210118111533-e33d3ffb5401",
    )
    go_repository(
        name = "com_github_cpuguy83_go_md2man_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cpuguy83/go-md2man/v2",
        sum = "h1:qMCsGGgs+MAzDFyp9LpAe1Lqy/fY/qCovCm0qnXZOBM=",
        version = "v2.0.3",
    )
    go_repository(
        name = "com_github_creack_pty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/creack/pty",
        sum = "h1:n56/Zwd5o6whRC5PMGretI4IdRLlmBXYNjScPaBgsbY=",
        version = "v1.1.18",
    )
    go_repository(
        name = "com_github_crewjam_httperr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/crewjam/httperr",
        sum = "h1:b2BfXR8U3AlIHwNeFFvZ+BV1LFvKLlzMjzaTnZMybNo=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_crewjam_saml",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/crewjam/saml",
        sum = "h1:TYHggH/hwP7eArqiXSJUvtOPNzQDyQ7vwmwEqlFWhMc=",
        version = "v0.4.13",
    )
    go_repository(
        # This is no longer used but we keep it for backwards compatability
        # tests while on 5.0.x.
        name = "com_github_crewjam_saml_samlidp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/crewjam/saml/samlidp",
        sum = "h1:13Ix7LoUJ0Yu5F+s6Aw8Afc8x+n98RSJNGHpxEbcYus=",
        version = "v0.0.0-20221211125903-d951aa2d145a",
    )  # keep
    go_repository(
        name = "com_github_danieljoos_wincred",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/danieljoos/wincred",
        sum = "h1:QLdCxFs1/Yl4zduvBdcHB8goaYk9RARS2SgLLRuAyr0=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_datadog_zstd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/DataDog/zstd",
        sum = "h1:+K/VEwIAaPcHiMtQvpLD4lqW7f0Gk3xdYZmI1hD+CXo=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_dave_jennifer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dave/jennifer",
        sum = "h1:T4T/67t6RAA5AIV6+NP8Uk/BIsXgDoqEowgycdQQLuk=",
        version = "v1.6.1",
    )
    go_repository(
        name = "com_github_davecgh_go_spew",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/davecgh/go-spew",
        sum = "h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_daviddengcn_go_colortext",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/daviddengcn/go-colortext",
        sum = "h1:ANqDyC0ys6qCSvuEK7l3g5RaehL/Xck9EX8ATG8oKsE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_dchest_uniuri",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dchest/uniuri",
        sum = "h1:koIcOUdrTIivZgSLhHQvKgqdWZq5d7KdMEWF1Ud6+5g=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_denisenkom_go_mssqldb",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/denisenkom/go-mssqldb",
        sum = "h1:1OcPn5GBIobjWNd+8yjfHNIaFX14B1pWI3F9HZy5KXw=",
        version = "v0.12.2",
    )
    go_repository(
        name = "com_github_dennwc_varint",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dennwc/varint",
        sum = "h1:kGNFFSSw8ToIy3obO/kKr8U9GZYUAxQEVuix4zfDWzE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_denverdino_aliyungo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/denverdino/aliyungo",
        sum = "h1:p6poVbjHDkKa+wtC8frBMwQtT3BmqGYBjzMwJ63tuR4=",
        version = "v0.0.0-20190125010748-a747050bb1ba",
    )
    go_repository(
        name = "com_github_derision_test_glock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/derision-test/glock",
        sum = "h1:b6sViZG+Cm6QtdpqbfWEjaBVbzNPntIS4GzsxpS+CmM=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_derision_test_go_mockgen",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/derision-test/go-mockgen",
        sum = "h1:Wo7vx8f7QdBT4jxfgTvs2t9VpyFlvvqzjXoE3m7MQBU=",
        version = "v1.3.8-0.20240105000756-fb9effb23d90",
    )
    go_repository(
        name = "com_github_dghubble_go_twitter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dghubble/go-twitter",
        sum = "h1:XQu6o3AwJx/jsg9LZ41uIeUdXK5be099XFfFn6H9ikk=",
        version = "v0.0.0-20221104224141-912508c3888b",
    )
    go_repository(
        name = "com_github_dghubble_gologin_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dghubble/gologin/v2",
        sum = "h1:Ga0dxZ2C/8MrMtC0qFLIg1K7cVjZQWSbTj/MIgFqMAg=",
        version = "v2.4.0",
    )
    go_repository(
        name = "com_github_dghubble_oauth1",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dghubble/oauth1",
        sum = "h1:pwcinOZy8z6XkNxvPmUDY52M7RDPxt0Xw1zgZ6Cl5JA=",
        version = "v0.7.2",
    )
    go_repository(
        name = "com_github_dghubble_sling",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dghubble/sling",
        sum = "h1:AxjTubpVyozMvbBCtXcsWEyGGgUZutC5YGrfxPNVOcQ=",
        version = "v1.4.1",
    )
    go_repository(
        name = "com_github_dgraph_io_ristretto",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgraph-io/ristretto",
        sum = "h1:6CWw5tJNgpegArSHpNHJKldNeq03FQCwYvfMVWajOK8=",
        version = "v0.1.1",
    )
    go_repository(
        name = "com_github_dgrijalva_jwt_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgrijalva/jwt-go",
        sum = "h1:7qlOGliEKZXTDg6OTjfoBKDXWrumCAMpl/TFQ4/5kLM=",
        version = "v3.2.0+incompatible",
    )
    go_repository(
        name = "com_github_dgryski_go_farm",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgryski/go-farm",
        sum = "h1:tdlZCpZ/P9DhczCTSixgIKmwPv6+wP5DGjqLYw5SUiA=",
        version = "v0.0.0-20190423205320-6a90982ecee2",
    )
    go_repository(
        name = "com_github_dgryski_go_rendezvous",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgryski/go-rendezvous",
        sum = "h1:lO4WD4F/rVNCu3HqELle0jiPLLBs70cWOduZpkS1E78=",
        version = "v0.0.0-20200823014737-9f7001d12a5f",
    )
    go_repository(
        name = "com_github_dgryski_go_sip13",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgryski/go-sip13",
        sum = "h1:9cOfvEwjQxdwKuNDTQSaMKNRvwKwgZG+U4HrjeRKHso=",
        version = "v0.0.0-20200911182023-62edffca9245",
    )
    go_repository(
        name = "com_github_dgryski_trifles",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgryski/trifles",
        sum = "h1:fRzb/w+pyskVMQ+UbP35JkH8yB7MYb4q/qhBarqZE6g=",
        version = "v0.0.0-20200323201526-dd97f9abfb48",
    )
    go_repository(
        name = "com_github_di_wu_parser",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/di-wu/parser",
        sum = "h1:I9oHJ8spBXOeL7Wps0ffkFFFiXJf/pk7NX9lcAMqRMU=",
        version = "v0.2.2",
    )
    go_repository(
        name = "com_github_di_wu_xsd_datetime",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/di-wu/xsd-datetime",
        sum = "h1:vZoGNkbzpBNoc+JyfVLEbutNDNydYV8XwHeV7eUJoxI=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_digitalocean_godo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/digitalocean/godo",
        sum = "h1:SAEdw63xOMmzlwCeCWjLH1GcyDPUjbSAR1Bh7VELxzc=",
        version = "v1.88.0",
    )
    go_repository(
        name = "com_github_dimchansky_utfbom",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dimchansky/utfbom",
        sum = "h1:vV6w1AhK4VMnhBno/TPVCoK9U/LP0PkLCS9tbxHdi/U=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_distribution_distribution_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/distribution/distribution/v3",
        sum = "h1:ou+y/Ko923eBli6zJ/TeB2iH38PmytV2UkHJnVdaPtU=",
        version = "v3.0.0-20220128175647-b60926597a1b",
    )
    go_repository(
        name = "com_github_djherbis_buffer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/djherbis/buffer",
        sum = "h1:PH5Dd2ss0C7CRRhQCZ2u7MssF+No9ide8Ye71nPHcrQ=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_djherbis_nio_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/djherbis/nio/v3",
        sum = "h1:6wxhnuppteMa6RHA4L81Dq7ThkZH8SwnDzXDYy95vB4=",
        version = "v3.0.1",
    )
    go_repository(
        name = "com_github_dlclark_regexp2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dlclark/regexp2",
        sum = "h1:+/GIL799phkJqYW+3YbOd8LCcbHzT0Pbo8zl70MHsq0=",
        version = "v1.10.0",
    )
    go_repository(
        name = "com_github_dnaeon_go_vcr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dnaeon/go-vcr",
        sum = "h1:zHCHvJYTMh1N7xnV7zf1m1GPBF9Ad0Jk/whtQ1663qI=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_docker_cli",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/cli",
        sum = "h1:0+1VshNwBQzQAx9lOl+OYCTCEAD8fKs/qeXMx3O0wqM=",
        version = "v24.0.0+incompatible",
    )
    go_repository(
        name = "com_github_docker_distribution",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/distribution",
        sum = "h1:T3de5rq0dB1j30rp0sA2rER+m322EBzniBPB6ZIzuh8=",
        version = "v2.8.2+incompatible",
    )
    go_repository(
        name = "com_github_docker_docker",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/docker",
        sum = "h1:z4bf8HvONXX9Tde5lGBMQ7yCJgNahmJumdrStZAbeY4=",
        version = "v24.0.0+incompatible",
    )
    go_repository(
        name = "com_github_docker_docker_credential_helpers",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/docker-credential-helpers",
        sum = "h1:xtCHsjxogADNZcdv1pKUHXryefjlVRqWqIhk/uXJp0A=",
        version = "v0.7.0",
    )
    go_repository(
        name = "com_github_docker_go_connections",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/go-connections",
        sum = "h1:El9xVISelRB7BuFusrZozjnkIM5YnzCViNKohAFqRJQ=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_docker_go_events",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/go-events",
        sum = "h1:+pKlWGMw7gf6bQ+oDZB4KHQFypsfjYlq/C4rfL7D3g8=",
        version = "v0.0.0-20190806004212-e31b211e4f1c",
    )
    go_repository(
        name = "com_github_docker_go_metrics",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/go-metrics",
        sum = "h1:AgB/0SvBxihN0X8OR4SjsblXkbMvalQ8cjmtKQ2rQV8=",
        version = "v0.0.1",
    )
    go_repository(
        name = "com_github_docker_go_units",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/go-units",
        sum = "h1:69rxXcBk27SvSaaxTtLh/8llcHD8vYHT7WSdRZ/jvr4=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_docker_libtrust",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/libtrust",
        sum = "h1:ZClxb8laGDf5arXfYcAtECDFgAgHklGI8CxgjHnXKJ4=",
        version = "v0.0.0-20150114040149-fa567046d9b1",
    )
    go_repository(
        name = "com_github_docopt_docopt_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docopt/docopt-go",
        sum = "h1:bWDMxwH3px2JBh6AyO7hdCn/PkvCZXii8TGj7sbtEbQ=",
        version = "v0.0.0-20180111231733-ee0de3bc6815",
    )
    go_repository(
        name = "com_github_dsnet_compress",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dsnet/compress",
        sum = "h1:iFaUwBSo5Svw6L7HYpRu/0lE3e0BaElwnNO1qkNQxBY=",
        version = "v0.0.2-0.20210315054119-f66993602bf5",
    )
    go_repository(
        name = "com_github_duo_labs_webauthn",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/duo-labs/webauthn",
        sum = "h1:BaeJtFDlto/NjX9t730OebRRJf2P+t9YEDz3ur18824=",
        version = "v0.0.0-20220815211337-00c9fb5711f5",
    )
    go_repository(
        name = "com_github_dustin_go_humanize",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dustin/go-humanize",
        sum = "h1:GzkhY7T5VNhEkwH0PVJgjz+fX1rhBrR7pRT3mDkpeCY=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_editorconfig_editorconfig_core_go_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/editorconfig/editorconfig-core-go/v2",
        sum = "h1:kTcVMyCvFGQmTk0S5+R7GF+y7wMHkWm4CYS5BwYWN8U=",
        version = "v2.4.5",
    )
    go_repository(
        name = "com_github_edsrzf_mmap_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/edsrzf/mmap-go",
        sum = "h1:6EUwBLQ/Mcr1EYLE4Tn1VdW1A4ckqCQWZBw8Hr0kjpQ=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_eknkc_amber",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/eknkc/amber",
        sum = "h1:clC1lXBpe2kTj2VHdaIu9ajZQe4kcEY9j0NsnDDBZ3o=",
        version = "v0.0.0-20171010120322-cdade1c07385",
    )
    go_repository(
        name = "com_github_elazarl_goproxy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/elazarl/goproxy",
        sum = "h1:RIB4cRk+lBqKK3Oy0r2gRX4ui7tuhiZq2SuTtTCi0/0=",
        version = "v0.0.0-20221015165544-a0805db90819",
    )
    go_repository(
        name = "com_github_elimity_com_scim",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/elimity-com/scim",
        sum = "h1:6fUaAaX4Xe07LhVrHNmpfnlU41Nsto4skz4vhVqGwYk=",
        version = "v0.0.0-20220121082953-15165b1a61c8",
    )
    go_repository(
        name = "com_github_emicklei_go_restful",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/emicklei/go-restful",
        sum = "h1:H2pdYOb3KQ1/YsqVWoWNLQO+fusocsw354rqGTZtAgw=",
        version = "v0.0.0-20170410110728-ff4f55a20633",
    )
    go_repository(
        name = "com_github_emicklei_go_restful_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/emicklei/go-restful/v3",
        sum = "h1:eCZ8ulSerjdAiaNpF7GxXIE7ZCMo1moN1qX+S609eVw=",
        version = "v3.8.0",
    )
    go_repository(
        name = "com_github_emicklei_proto",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/emicklei/proto",
        sum = "h1:XbpwxmuOPrdES97FrSfpyy67SSCV/wBIKXqgJzh6hNw=",
        version = "v1.6.15",
    )
    go_repository(
        name = "com_github_emirpasic_gods",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/emirpasic/gods",
        sum = "h1:FXtiHYKDGKCW2KzwZKx0iC0PQmdlorYgdFG9jPXJ1Bc=",
        version = "v1.18.1",
    )
    go_repository(
        name = "com_github_envoyproxy_go_control_plane",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/envoyproxy/go-control-plane",
        sum = "h1:wSUXTlLfiAQRWs2F+p+EKOY9rUyis1MyGqJ2DIk5HpM=",
        version = "v0.11.1",
    )
    go_repository(
        name = "com_github_envoyproxy_protoc_gen_validate",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/envoyproxy/protoc-gen-validate",
        sum = "h1:QkIBuU5k+x7/QXPvPPnWXWlCdaBFApVqftFV6k087DA=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_ethantkoenig_rupture",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ethantkoenig/rupture",
        sum = "h1:6aAXghmvtnngMgQzy7SMGdicMvkV86V4n9fT0meE5E4=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_evanphx_json_patch",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/evanphx/json-patch",
        sum = "h1:jBYDEEiFBPxA0v50tFdvOzQQTCvpL6mnFh5mB2/l16U=",
        version = "v5.6.0+incompatible",
    )
    go_repository(
        name = "com_github_facebookgo_clock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/facebookgo/clock",
        sum = "h1:yDWHCSQ40h88yih2JAcL6Ls/kVkSE8GFACTGVnMPruw=",
        version = "v0.0.0-20150410010913-600d898af40a",
    )
    go_repository(
        name = "com_github_facebookgo_ensure",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/facebookgo/ensure",
        sum = "h1:8ISkoahWXwZR41ois5lSJBSVw4D0OV19Ht/JSTzvSv0=",
        version = "v0.0.0-20200202191622-63f1cf65ac4c",
    )
    go_repository(
        name = "com_github_facebookgo_limitgroup",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/facebookgo/limitgroup",
        sum = "h1:IeaD1VDVBPlx3viJT9Md8if8IxxJnO+x0JCGb054heg=",
        version = "v0.0.0-20150612190941-6abd8d71ec01",
    )
    go_repository(
        name = "com_github_facebookgo_muster",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/facebookgo/muster",
        sum = "h1:a4DFiKFJiDRGFD1qIcqGLX/WlUMD9dyLSLDt+9QZgt8=",
        version = "v0.0.0-20150708232844-fd3d7953fd52",
    )
    go_repository(
        name = "com_github_facebookgo_stack",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/facebookgo/stack",
        sum = "h1:JWuenKqqX8nojtoVVWjGfOF9635RETekkoH6Cc9SX0A=",
        version = "v0.0.0-20160209184415-751773369052",
    )
    go_repository(
        name = "com_github_facebookgo_subset",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/facebookgo/subset",
        sum = "h1:7HZCaLC5+BZpmbhCOZJ293Lz68O7PYrF2EzeiFMwCLk=",
        version = "v0.0.0-20200203212716-c811ad88dec4",
    )
    go_repository(
        name = "com_github_fatih_color",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fatih/color",
        sum = "h1:kOqh6YHBtK8aywxGerMG2Eq3H6Qgoqeo13Bk2Mv/nBs=",
        version = "v1.15.0",
    )
    go_repository(
        name = "com_github_fatih_structs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fatih/structs",
        sum = "h1:Q7juDM0QtcnhCpeyLGQKyg4TOIghuNXrkL32pHAUMxo=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_felixge_fgprof",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/felixge/fgprof",
        sum = "h1:VvyZxILNuCiUCSXtPtYmmtGvb65nqXh2QFWc0Wpf2/g=",
        version = "v0.9.3",
    )
    go_repository(
        name = "com_github_felixge_httpsnoop",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/felixge/httpsnoop",
        sum = "h1:s/nj+GCswXYzN5v2DpNMuMQYe+0DDwt5WVCU6CWBdXk=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_flosch_pongo2_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/flosch/pongo2/v4",
        sum = "h1:gv+5Pe3vaSVmiJvh/BZa82b7/00YUGm0PIyVVLop0Hw=",
        version = "v4.0.2",
    )
    go_repository(
        name = "com_github_form3tech_oss_jwt_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/form3tech-oss/jwt-go",
        sum = "h1:7ZaBxOI7TMoYBfyA3cQHErNNyAWIKUMIwqxEtgHOs5c=",
        version = "v3.2.3+incompatible",
    )
    go_repository(
        name = "com_github_frankban_quicktest",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/frankban/quicktest",
        sum = "h1:FJKSZTDHjyhriyC81FLQ0LY93eSai0ZyR/ZIkd3ZUKE=",
        version = "v1.14.3",
    )
    go_repository(
        name = "com_github_fsnotify_fsnotify",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fsnotify/fsnotify",
        sum = "h1:n+5WquG0fcWoWp6xPWfHdbskMCQaFnG6PfBrh1Ky4HY=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_fullstorydev_grpcui",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fullstorydev/grpcui",
        sum = "h1:lVXozTNkJJouBL+wpmvxMnltiwYp8mgyd0TRs93i6Rw=",
        version = "v1.3.1",
    )
    go_repository(
        name = "com_github_fullstorydev_grpcurl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fullstorydev/grpcurl",
        sum = "h1:WylAwnPauJIofYSHqqMTC1eEfUIzqzevXyogBxnQquo=",
        version = "v1.8.6",
    )
    go_repository(
        name = "com_github_fxamacker_cbor_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fxamacker/cbor/v2",
        sum = "h1:ri0ArlOR+5XunOP8CRUowT0pSJOwhW098ZCUyskZD88=",
        version = "v2.4.0",
    )
    go_repository(
        name = "com_github_garyburd_redigo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/garyburd/redigo",
        sum = "h1:Sk0u0gIncQaQD23zAoAZs2DNi2u2l5UTLi4CmCBL5v8=",
        version = "v1.1.1-0.20170914051019-70e1b1943d4f",
    )
    go_repository(
        name = "com_github_gen2brain_beeep",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gen2brain/beeep",
        sum = "h1:Xh9mvwEmhbdXlRSsgn+N0zj/NqnKvpeqL08oKDHln2s=",
        version = "v0.0.0-20210529141713-5586760f0cc1",
    )
    go_repository(
        name = "com_github_getkin_kin_openapi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/getkin/kin-openapi",
        sum = "h1:j77zg3Ec+k+r+GA3d8hBoXpAc6KX9TbBPrwQGBIy2sY=",
        version = "v0.76.0",
    )
    go_repository(
        name = "com_github_getsentry_sentry_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/getsentry/sentry-go",
        sum = "h1:q6Eo+hS+yoJlTO3uu/azhQadsD8V+jQn2D8VvX1eOyI=",
        version = "v0.25.0",
    )
    go_repository(
        name = "com_github_gfleury_go_bitbucket_v1",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gfleury/go-bitbucket-v1",
        sum = "h1:Ea58sAu8LX/NS7z3aeL+YX/7+FSd9jsxq6Ecpz7QEDM=",
        version = "v0.0.0-20230626192437-8d7be5866751",
    )
    go_repository(
        name = "com_github_ghodss_yaml",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ghodss/yaml",
        replace = "github.com/sourcegraph/yaml",
        sum = "h1:z/MpntplPaW6QW95pzcAR/72Z5TWDyDnSo0EOcyij9o=",
        version = "v1.0.1-0.20200714132230-56936252f152",
    )
    go_repository(
        name = "com_github_gin_contrib_sse",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gin-contrib/sse",
        sum = "h1:Y/yl/+YNO8GZSjAhjMsSuLt29uWRFHdHYUb5lYOV9qE=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_gin_gonic_gin",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gin-gonic/gin",
        sum = "h1:4+fr/el88TOO3ewCmQr8cx/CtZ/umlIRIs5M4NTNjf8=",
        version = "v1.8.1",
    )
    go_repository(
        name = "com_github_gitchander_permutation",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gitchander/permutation",
        sum = "h1:FUKJibWQu771xr/AwLn2/PbVp9AsgqfkObByTf8kJnI=",
        version = "v0.0.0-20210517125447-a5d73722e1b1",
    )
    go_repository(
        name = "com_github_gliderlabs_ssh",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gliderlabs/ssh",
        sum = "h1:OcaySEmAQJgyYcArR+gGGTHCyE7nvhEMTlYY+Dp8CpY=",
        version = "v0.3.5",
    )
    go_repository(
        name = "com_github_globalsign_mgo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/globalsign/mgo",
        sum = "h1:DujepqpGd1hyOd7aW59XpK7Qymp8iy83xq74fLr21is=",
        version = "v0.0.0-20181015135952-eeefdecb41b8",
    )
    go_repository(
        name = "com_github_go_ap_activitypub",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-ap/activitypub",
        sum = "h1:EUMB0x7u3de/ikGBtXiLxaJbmxgiqiAcM4yjW4whApM=",
        version = "v0.0.0-20220917143152-e4e7018838c0",
    )
    go_repository(
        name = "com_github_go_ap_errors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-ap/errors",
        sum = "h1:A48SbkWKEciiJMbbcPzaRj9aizPUABzXFvCM3LtGGf8=",
        version = "v0.0.0-20220917143055-4283ea5dae18",
    )
    go_repository(
        name = "com_github_go_ap_jsonld",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-ap/jsonld",
        sum = "h1:0tV3i8tE1NghMC4rXZXfD39KUbkKgIyLTsvOEmMOPCQ=",
        version = "v0.0.0-20220917142617-76bf51585778",
    )
    go_repository(
        name = "com_github_go_asn1_ber_asn1_ber",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-asn1-ber/asn1-ber",
        sum = "h1:vXT6d/FNDiELJnLb6hGNa309LMsrCoYFvpwHDF0+Y1A=",
        version = "v1.5.4",
    )
    go_repository(
        name = "com_github_go_chi_chi_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-chi/chi/v5",
        sum = "h1:rDTPXLDHGATaeHvVlLcR4Qe0zftYethFucbjVQ1PxU8=",
        version = "v5.0.7",
    )
    go_repository(
        name = "com_github_go_chi_cors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-chi/cors",
        sum = "h1:xEC8UT3Rlp2QuWNEr4Fs/c2EAGVKBwy/1vHx3bppil4=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_go_enry_go_enry_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-enry/go-enry/v2",
        sum = "h1:QrY3hx/RiqCJJRbdU0MOcjfTM1a586J0WSooqdlJIhs=",
        version = "v2.8.4",
    )
    go_repository(
        name = "com_github_go_enry_go_oniguruma",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-enry/go-oniguruma",
        sum = "h1:k8aAMuJfMrqm/56SG2lV9Cfti6tC4x8673aHCcBk+eo=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_go_errors_errors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-errors/errors",
        sum = "h1:J6MZopCL4uSllY1OfXM374weqZFFItUbrImctkmUxIA=",
        version = "v1.4.2",
    )
    go_repository(
        name = "com_github_go_fed_httpsig",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-fed/httpsig",
        sum = "h1:oRq/fiirun5HqlEWMLIcDmLpIELlG4iGbd0s8iqgPi8=",
        version = "v1.1.1-0.20201223112313-55836744818e",
    )
    go_repository(
        name = "com_github_go_fonts_liberation",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-fonts/liberation",
        sum = "h1:3BI2iaE7R/s6uUUtzNCjo3QijJu3aS4wmrMgfSpYQ+8=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_go_git_gcfg",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-git/gcfg",
        sum = "h1:+zs/tPmkDkHx3U66DAb0lQFJrpS6731Oaa12ikc+DiI=",
        version = "v1.5.1-0.20230307220236-3a3c6141e376",
    )
    go_repository(
        name = "com_github_go_git_go_billy_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-git/go-billy/v5",
        sum = "h1:Uwp5tDRkPr+l/TnbHOQzp+tmJfLceOlbVucgpTz8ix4=",
        version = "v5.4.1",
    )
    go_repository(
        name = "com_github_go_git_go_git_fixtures_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-git/go-git-fixtures/v4",
        sum = "h1:Pz0DHeFij3XFhoBRGUDPzSJ+w2UcK5/0JvF8DRI58r8=",
        version = "v4.3.2-0.20230305113008-0c11038e723f",
    )
    go_repository(
        name = "com_github_go_git_go_git_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-git/go-git/v5",
        sum = "h1:t9AudWVLmqzlo+4bqdf7GY+46SUuRsx59SboFxkq2aE=",
        version = "v5.7.0",
    )
    go_repository(
        name = "com_github_go_gl_glfw",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-gl/glfw",
        sum = "h1:QbL/5oDUmRBzO9/Z7Seo6zf912W/a6Sr4Eu0G/3Jho0=",
        version = "v0.0.0-20190409004039-e6da0acd62b1",
    )
    go_repository(
        name = "com_github_go_gl_glfw_v3_3_glfw",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-gl/glfw/v3.3/glfw",
        sum = "h1:WtGNWLvXpe6ZudgnXrq0barxBImvnnJoMEhXAzcbM0I=",
        version = "v0.0.0-20200222043503-6f7a984d4dc4",
    )
    go_repository(
        name = "com_github_go_kit_kit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-kit/kit",
        sum = "h1:dXFJfIHVvUcpSgDOV+Ne6t7jXri8Tfv2uOLHUZ2XNuo=",
        version = "v0.10.0",
    )
    go_repository(
        name = "com_github_go_kit_log",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-kit/log",
        sum = "h1:MRVx0/zhvdseW+Gza6N9rVzU/IVzaeE1SFI4raAhmBU=",
        version = "v0.2.1",
    )
    go_repository(
        name = "com_github_go_latex_latex",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-latex/latex",
        sum = "h1:NxXI5pTAtpEaU49bpLpQoDsu1zrteW/vxzTz8Cd2UAs=",
        version = "v0.0.0-20230307184459-12ec69307ad9",
    )
    go_repository(
        name = "com_github_go_ldap_ldap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-ldap/ldap",
        sum = "h1:kD5HQcAzlQ7yrhfn+h+MSABeAy/jAJhvIJ/QDllP44g=",
        version = "v3.0.2+incompatible",
    )
    go_repository(
        name = "com_github_go_ldap_ldap_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-ldap/ldap/v3",
        sum = "h1:qPjipEpt+qDa6SI/h1fzuGWoRUY+qqQ9sOZq67/PYUs=",
        version = "v3.4.4",
    )
    go_repository(
        name = "com_github_go_logfmt_logfmt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-logfmt/logfmt",
        sum = "h1:otpy5pqBCBZ1ng9RQ0dPu4PN7ba75Y/aA+UpowDyNVA=",
        version = "v0.5.1",
    )
    go_repository(
        name = "com_github_go_logr_logr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-logr/logr",
        sum = "h1:2y3SDp0ZXuc6/cjLSZ+Q3ir+QB9T/iG5yYRXqsagWSY=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_go_logr_stdr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-logr/stdr",
        sum = "h1:hSWxHoqTgW2S2qGc0LTAI563KZ5YKYRhT3MFKZMbjag=",
        version = "v1.2.2",
    )
    go_repository(
        name = "com_github_go_martini_martini",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-martini/martini",
        sum = "h1:xveKWz2iaueeTaUgdetzel+U7exyigDYBryyVfV/rZk=",
        version = "v0.0.0-20170121215854-22fa46961aab",
    )
    go_repository(
        name = "com_github_go_ole_go_ole",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-ole/go-ole",
        sum = "h1:/Fpf6oFPoeFik9ty7siob0G6Ke8QvQEuVcuChpwXzpY=",
        version = "v1.2.6",
    )
    go_repository(
        name = "com_github_go_openapi_analysis",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/analysis",
        sum = "h1:ZDFLvSNxpDaomuCueM0BlSXxpANBlFYiBvr+GXrvIHc=",
        version = "v0.21.4",
    )
    go_repository(
        name = "com_github_go_openapi_errors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/errors",
        sum = "h1:rz6kiC84sqNQoqrtulzaL/VERgkoCyB6WdEkc2ujzUc=",
        version = "v0.20.3",
    )
    go_repository(
        name = "com_github_go_openapi_inflect",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/inflect",
        sum = "h1:9jCH9scKIbHeV9m12SmPilScz6krDxKRasNNSNPXu/4=",
        version = "v0.19.0",
    )
    go_repository(
        name = "com_github_go_openapi_jsonpointer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/jsonpointer",
        sum = "h1:gZr+CIYByUqjcgeLXnQu2gHYQC9o73G2XUeOFYEICuY=",
        version = "v0.19.5",
    )
    go_repository(
        name = "com_github_go_openapi_jsonreference",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/jsonreference",
        sum = "h1:MYlu0sBgChmCfJxxUKZ8g1cPWFOB37YSZqewK7OKeyA=",
        version = "v0.20.0",
    )
    go_repository(
        name = "com_github_go_openapi_loads",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/loads",
        sum = "h1:r2a/xFIYeZ4Qd2TnGpWDIQNcP80dIaZgf704za8enro=",
        version = "v0.21.2",
    )
    go_repository(
        name = "com_github_go_openapi_runtime",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/runtime",
        sum = "h1:yX9HMGQbz32M87ECaAhGpJjBmErO3QLcgdZj9BzGx7c=",
        version = "v0.24.2",
    )
    go_repository(
        name = "com_github_go_openapi_spec",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/spec",
        sum = "h1:1Rlu/ZrOCCob0n+JKKJAWhNWMPW8bOZRg8FJaY+0SKI=",
        version = "v0.20.7",
    )
    go_repository(
        name = "com_github_go_openapi_strfmt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/strfmt",
        sum = "h1:xwhj5X6CjXEZZHMWy1zKJxvW9AfHC9pkyUjLvHtKG7o=",
        version = "v0.21.3",
    )
    go_repository(
        name = "com_github_go_openapi_swag",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/swag",
        sum = "h1:yMBqmnQ0gyZvEb/+KzuWZOXgllrXT4SADYbvDaXHv/g=",
        version = "v0.22.3",
    )
    go_repository(
        name = "com_github_go_openapi_validate",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/validate",
        sum = "h1:b0QecH6VslW/TxtpKgzpO1SNG7GU2FsaqKdP1E2T50Y=",
        version = "v0.22.0",
    )
    go_repository(
        name = "com_github_go_pdf_fpdf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-pdf/fpdf",
        sum = "h1:MlgtGIfsdMEEQJr2le6b/HNr1ZlQwxyWr77r2aj2U/8=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_go_playground_locales",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-playground/locales",
        sum = "h1:u50s323jtVGugKlcYeyzC0etD1HifMjqmJqb8WugfUU=",
        version = "v0.14.0",
    )
    go_repository(
        name = "com_github_go_playground_universal_translator",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-playground/universal-translator",
        sum = "h1:82dyy6p4OuJq4/CByFNOn/jYrnRPArHwAcmLoJZxyho=",
        version = "v0.18.0",
    )
    go_repository(
        name = "com_github_go_playground_validator_v10",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-playground/validator/v10",
        sum = "h1:prmOlTVv+YjZjmRmNSF3VmspqJIxJWXmqUsHwfTRRkQ=",
        version = "v10.11.1",
    )
    go_repository(
        name = "com_github_go_redis_redis",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-redis/redis",
        sum = "h1:K0pv1D7EQUjfyoMql+r/jZqCLizCGKFlFgcHWWmHQjg=",
        version = "v6.15.9+incompatible",
    )
    go_repository(
        name = "com_github_go_redis_redis_v7",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-redis/redis/v7",
        sum = "h1:7obg6wUoj05T0EpY0o8B59S9w5yeMWql7sw2kwNW1x4=",
        version = "v7.4.0",
    )
    go_repository(
        name = "com_github_go_redis_redis_v8",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-redis/redis/v8",
        sum = "h1:AcZZR7igkdvfVmQTPnu9WE37LRrO/YrBH5zWyjDC0oI=",
        version = "v8.11.5",
    )
    go_repository(
        name = "com_github_go_redsync_redsync_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-redsync/redsync/v4",
        sum = "h1:rq2RvdTI0obznMdxKUWGdmmulo7lS9yCzb8fgDKOlbM=",
        version = "v4.8.1",
    )
    go_repository(
        name = "com_github_go_resty_resty_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-resty/resty/v2",
        sum = "h1:JVrqSeQfdhYRFk24TvhTZWU0q8lfCojxZQFi3Ou7+uY=",
        version = "v2.1.1-0.20191201195748-d7b97669fe48",
    )
    go_repository(
        name = "com_github_go_sql_driver_mysql",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-sql-driver/mysql",
        sum = "h1:lUIinVbN1DY0xBg0eMOzmmtGoHwWBbvnWubQUrtU8EI=",
        version = "v1.7.1",
    )
    go_repository(
        name = "com_github_go_stack_stack",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-stack/stack",
        sum = "h1:ntEHSVwIt7PNXNpgPmVfMrNhLtgjlmnZha2kOpuRiDw=",
        version = "v1.8.1",
    )
    go_repository(
        name = "com_github_go_swagger_go_swagger",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-swagger/go-swagger",
        sum = "h1:HuzvdMRed/9Q8vmzVcfNBQByZVtT79DNZxZ18OprdoI=",
        version = "v0.30.3",
    )
    go_repository(
        name = "com_github_go_task_slim_sprig",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-task/slim-sprig",
        sum = "h1:tfuBGBXKqDEevZMzYi5KSi8KkcZtzBcTgAUUtapy0OI=",
        version = "v0.0.0-20230315185526-52ccab3ef572",
    )
    go_repository(
        name = "com_github_go_test_deep",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-test/deep",
        sum = "h1:u2CU3YKy9I2pmu9pX0eq50wCgjfGIt539SqR7FbHiho=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_go_testfixtures_testfixtures_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-testfixtures/testfixtures/v3",
        sum = "h1:uonwvepqRvSgddcrReZQhojTlWlmOlHkYAb9ZaOMWgU=",
        version = "v3.8.1",
    )
    go_repository(
        name = "com_github_go_toast_toast",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-toast/toast",
        sum = "h1:qZNfIGkIANxGv/OqtnntR4DfOY2+BgwR60cAcu/i3SE=",
        version = "v0.0.0-20190211030409-01e6764cf0a4",
    )
    go_repository(
        name = "com_github_go_zookeeper_zk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-zookeeper/zk",
        sum = "h1:7M2kwOsc//9VeeFiPtf+uSJlVpU66x9Ba5+8XK7/TDg=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_gobuffalo_attrs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/attrs",
        sum = "h1:hSkbZ9XSyjyBirMeqSqUrK+9HboWrweVlzRNqoBi2d4=",
        version = "v0.0.0-20190224210810-a9411de4debd",
    )
    go_repository(
        name = "com_github_gobuffalo_depgen",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/depgen",
        sum = "h1:31atYa/UW9V5q8vMJ+W6wd64OaaTHUrCUXER358zLM4=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_gobuffalo_envy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/envy",
        sum = "h1:GlXgaiBkmrYMHco6t4j7SacKO4XUjvh5pwXh0f4uxXU=",
        version = "v1.7.0",
    )
    go_repository(
        name = "com_github_gobuffalo_flect",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/flect",
        sum = "h1:3GQ53z7E3o00C/yy7Ko8VXqQXoJGLkrTQCLTF1EjoXU=",
        version = "v0.1.3",
    )
    go_repository(
        name = "com_github_gobuffalo_genny",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/genny",
        sum = "h1:iQ0D6SpNXIxu52WESsD+KoQ7af2e3nCfnSBoSF/hKe0=",
        version = "v0.1.1",
    )
    go_repository(
        name = "com_github_gobuffalo_gitgen",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/gitgen",
        sum = "h1:mSVZ4vj4khv+oThUfS+SQU3UuFIZ5Zo6UNcvK8E8Mz8=",
        version = "v0.0.0-20190315122116-cc086187d211",
    )
    go_repository(
        name = "com_github_gobuffalo_gogen",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/gogen",
        sum = "h1:dLg+zb+uOyd/mKeQUYIbwbNmfRsr9hd/WtYWepmayhI=",
        version = "v0.1.1",
    )
    go_repository(
        name = "com_github_gobuffalo_logger",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/logger",
        sum = "h1:8thhT+kUJMTMy3HlX4+y9Da+BNJck+p109tqqKp7WDs=",
        version = "v0.0.0-20190315122211-86e12af44bc2",
    )
    go_repository(
        name = "com_github_gobuffalo_mapi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/mapi",
        sum = "h1:fq9WcL1BYrm36SzK6+aAnZ8hcp+SrmnDyAxhNx8dvJk=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_gobuffalo_packd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/packd",
        sum = "h1:4sGKOD8yaYJ+dek1FDkwcxCHA40M4kfKgFHx8N2kwbU=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_gobuffalo_packr_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/packr/v2",
        sum = "h1:Ir9W9XIm9j7bhhkKE9cokvtTl1vBm62A/fene/ZCj6A=",
        version = "v2.2.0",
    )
    go_repository(
        name = "com_github_gobuffalo_syncx",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/syncx",
        sum = "h1:tpom+2CJmpzAWj5/VEHync2rJGi+epHNIeRSWjzGA+4=",
        version = "v0.0.0-20190224160051-33c29581e754",
    )
    go_repository(
        name = "com_github_gobwas_glob",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobwas/glob",
        sum = "h1:A4xDbljILXROh+kObIiy5kIaPYD8e96x1tgBhUI5J+Y=",
        version = "v0.2.3",
    )
    go_repository(
        name = "com_github_gobwas_httphead",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobwas/httphead",
        sum = "h1:exrUm0f4YX0L7EBwZHuCF4GDp8aJfVeBrlLQrs6NqWU=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_gobwas_pool",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobwas/pool",
        sum = "h1:xfeeEhW7pwmX8nuLVlqbzVc7udMDrwetjEv+TZIz1og=",
        version = "v0.2.1",
    )
    go_repository(
        name = "com_github_gobwas_ws",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobwas/ws",
        sum = "h1:7RFti/xnNkMJnrK7D1yQ/iCIB5OrrY/54/H930kIbHA=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_goccy_go_json",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/goccy/go-json",
        sum = "h1:CrxCmQqYDkv1z7lO7Wbh2HN93uovUHgrECaO5ZrCXAU=",
        version = "v0.10.2",
    )
    go_repository(
        name = "com_github_godbus_dbus_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/godbus/dbus/v5",
        sum = "h1:mkgN1ofwASrYnJ5W6U/BxG15eXXXjirgZc7CLqkcaro=",
        version = "v5.0.6",
    )
    go_repository(
        name = "com_github_gofrs_flock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gofrs/flock",
        sum = "h1:+gYjHKf32LDeiEEFhQaotPbLuUXjY5ZqxKgXy7n59aw=",
        version = "v0.8.1",
    )
    go_repository(
        name = "com_github_gofrs_uuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gofrs/uuid",
        sum = "h1:yyYWMnhkhrKwwr8gAOcOCYxOOscHgDS9yZgBrnJfGa0=",
        version = "v4.2.0+incompatible",
    )
    go_repository(
        name = "com_github_gogo_googleapis",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gogo/googleapis",
        sum = "h1:1Yx4Myt7BxzvUr5ldGSbwYiZG6t9wGBZ+8/fX3Wvtq0=",
        version = "v1.4.1",
    )
    go_repository(
        name = "com_github_gogo_protobuf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gogo/protobuf",
        sum = "h1:Ov1cvc58UF3b5XjBnZv7+opcTcQFZebYjWzi34vdm4Q=",
        version = "v1.3.2",
    )
    go_repository(
        name = "com_github_gogo_status",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gogo/status",
        sum = "h1:+eIkrewn5q6b30y+g/BJINVVdi2xH7je5MPJ3ZPK3JA=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_gogs_chardet",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gogs/chardet",
        sum = "h1:3BSP1Tbs2djlpprl7wCLuiqMaUh5SJkkzI2gDs+FgLs=",
        version = "v0.0.0-20211120154057-b7413eaefb8f",
    )
    go_repository(
        name = "com_github_gogs_cron",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gogs/cron",
        sum = "h1:yXtpJr/LV6PFu4nTLgfjQdcMdzjbqqXMEnHfq0Or6p8=",
        version = "v0.0.0-20171120032916-9f6c956d3e14",
    )
    go_repository(
        name = "com_github_gogs_go_gogs_client",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gogs/go-gogs-client",
        sum = "h1:UjoPNDAQ5JPCjlxoJd6K8ALZqSDDhk2ymieAZOVaDg0=",
        version = "v0.0.0-20210131175652-1d7215cd8d85",
    )
    go_repository(
        name = "com_github_golang_freetype",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/freetype",
        sum = "h1:DACJavvAHhabrF08vX0COfcOBJRhZ8lUbR+ZWIs0Y5g=",
        version = "v0.0.0-20170609003504-e2365dfdc4a0",
    )
    go_repository(
        name = "com_github_golang_gddo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/gddo",
        sum = "h1:16RtHeWGkJMc80Etb8RPCcKevXGldr57+LOyZt8zOlg=",
        version = "v0.0.0-20210115222349-20d68f94ee1f",
    )
    go_repository(
        name = "com_github_golang_geo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/geo",
        sum = "h1:gtexQ/VGyN+VVFRXSFiguSNcXmS6rkKT+X7FdIrTtfo=",
        version = "v0.0.0-20210211234256-740aa86cb551",
    )
    go_repository(
        name = "com_github_golang_glog",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/glog",
        sum = "h1:DVjP2PbBOzHyzA+dn3WhHIq4NdVu3Q+pvivFICf/7fo=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_golang_groupcache",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/groupcache",
        sum = "h1:oI5xCqsCo564l8iNU+DwB5epxmsaqB+rhGL0m5jtYqE=",
        version = "v0.0.0-20210331224755-41bb18bfe9da",
    )
    go_repository(
        name = "com_github_golang_jwt_jwt_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-jwt/jwt/v4",
        sum = "h1:7cYmW1XlMY7h7ii7UhUyChSgS5wUJEnm9uZVTGqOWzg=",
        version = "v4.5.0",
    )
    go_repository(
        name = "com_github_golang_jwt_jwt_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-jwt/jwt/v5",
        sum = "h1:1n1XNM9hk7O9mnQoNBGolZvzebBQ7p93ULHRc28XJUE=",
        version = "v5.0.0",
    )
    go_repository(
        name = "com_github_golang_lint",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/lint",
        replace = "golang.org/x/lint",
        sum = "h1:J5lckAjkw6qYlOZNj90mLYNTEKDvWeuc1yieZ8qUzUE=",
        version = "v0.0.0-20191125180803-fdd1cda4f05f",
    )
    go_repository(
        name = "com_github_golang_mock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/mock",
        sum = "h1:ErTB+efbowRARo13NNdxyJji2egdxLGQhRaY+DUumQc=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_golang_protobuf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/protobuf",
        sum = "h1:KhyjKVUg7Usr/dYsdSqoFveMYd5ko72D+zANwlG1mmg=",
        version = "v1.5.3",
    )
    go_repository(
        name = "com_github_golang_snappy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/snappy",
        sum = "h1:yAGX7huGHXlcLOEtBnF4w7FQwA26wojNCwOYAEhLjQM=",
        version = "v0.0.4",
    )
    go_repository(
        name = "com_github_golang_sql_civil",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-sql/civil",
        sum = "h1:au07oEsX2xN0ktxqI+Sida1w446QrXBRJ0nee3SNZlA=",
        version = "v0.0.0-20220223132316-b832511892a9",
    )
    go_repository(
        name = "com_github_golang_sql_sqlexp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-sql/sqlexp",
        sum = "h1:ZCD6MBpcuOVfGVqsEmY5/4FtYiKz6tSyUv9LPEDei6A=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_golangplus_bytes",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golangplus/bytes",
        sum = "h1:YQKBijBVMsBxIiXT4IEhlKR2zHohjEqPole4umyDX+c=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_golangplus_fmt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golangplus/fmt",
        sum = "h1:FnUKtw86lXIPfBMc3FimNF3+ABcV+aH5F17OOitTN+E=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_golangplus_testing",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golangplus/testing",
        sum = "h1:+ZeeiKZENNOMkTTELoSySazi+XaEhVO0mb+eanrSEUQ=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_gomodule_oauth1",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gomodule/oauth1",
        sum = "h1:/nNHAD99yipOEspQFbAnNmwGTZ1UNXiD/+JLxwx79fo=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_gomodule_redigo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gomodule/redigo",
        replace = "github.com/gomodule/redigo",
        sum = "h1:Sl3u+2BI/kk+VEatbj0scLdrFhjPmbxOc1myhDP41ws=",
        version = "v1.8.9",
    )
    go_repository(
        name = "com_github_google_btree",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/btree",
        sum = "h1:gK4Kx5IaGY9CD5sPJ36FHiBJ6ZXl0kilRiiCj+jdYp4=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_google_cel_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/cel-go",
        sum = "h1:s2151PDGy/eqpCI80/8dl4VL3xTkqI/YubXLXCFw0mw=",
        version = "v0.17.1",
    )
    go_repository(
        name = "com_github_google_certificate_transparency_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/certificate-transparency-go",
        sum = "h1:806qveZBQtRNHroYHyg6yrsjqBJh9kIB4nfmB8uJnak=",
        version = "v1.1.2-0.20210511102531-373a877eec92",
    )
    go_repository(
        name = "com_github_google_flatbuffers",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/flatbuffers",
        sum = "h1:ivUb1cGomAB101ZM1T0nOiWz9pSrTMoa9+EiY7igmkM=",
        version = "v2.0.8+incompatible",
    )
    go_repository(
        name = "com_github_google_gnostic",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/gnostic",
        sum = "h1:FhTMOKj2VhjpouxvWJAV1TL304uMlb9zcDqkl6cEI54=",
        version = "v0.5.7-v3refs",
    )
    go_repository(
        name = "com_github_google_go_cmp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-cmp",
        sum = "h1:ofyhxvXcZhMsU5ulbFiLKl/XBFqE1GSq7atu8tAmTRI=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_google_go_containerregistry",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-containerregistry",
        sum = "h1:rUEt426sR6nyrL3gt+18ibRcvYpKYdpsa5ZW7MA08dQ=",
        version = "v0.16.1",
    )
    go_repository(
        name = "com_github_google_go_github_v27",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-github/v27",
        sum = "h1:oiOZuBmGHvrGM1X9uNUAUlLgp5r1UUO/M/KnbHnLRlQ=",
        version = "v27.0.6",
    )
    go_repository(
        name = "com_github_google_go_github_v45",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-github/v45",
        sum = "h1:5oRLszbrkvxDDqBCNj2hjDZMKmvexaZ1xw/FCD+K3FI=",
        version = "v45.2.0",
    )
    go_repository(
        name = "com_github_google_go_github_v48",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-github/v48",
        sum = "h1:68puzySE6WqUY9KWmpOsDEQfDZsso98rT6pZcz9HqcE=",
        version = "v48.2.0",
    )
    go_repository(
        name = "com_github_google_go_github_v55",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-github/v55",
        sum = "h1:4pp/1tNMB9X/LuAhs5i0KQAE40NmiR/y6prLNb9x9cg=",
        version = "v55.0.0",
    )
    go_repository(
        name = "com_github_google_go_pkcs11",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-pkcs11",
        sum = "h1:OF1IPgv+F4NmqmJ98KTjdN97Vs1JxDPB3vbmYzV2dpk=",
        version = "v0.2.1-0.20230907215043-c6f79328ddf9",
    )
    go_repository(
        name = "com_github_google_go_querystring",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-querystring",
        sum = "h1:AnCroh3fv4ZBgVIf1Iwtovgjaw/GiKJo8M8yD/fhyJ8=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_google_gofuzz",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/gofuzz",
        sum = "h1:xRy4A+RhZaiKjJ1bPfwQ8sedCA+YS2YcCHW6ec7JMi0=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_google_martian",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/martian",
        sum = "h1:/CP5g8u/VJHijgedC/Legn3BAbAaWPgecwXBIDzw5no=",
        version = "v2.1.0+incompatible",
    )
    go_repository(
        name = "com_github_google_martian_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/martian/v3",
        sum = "h1:IqNFLAmvJOgVlpdEBiQbDc2EwKW77amAycfTuWKdfvw=",
        version = "v3.3.2",
    )
    go_repository(
        name = "com_github_google_pprof",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/pprof",
        sum = "h1:hR7/MlvK23p6+lIw9SN1TigNLn9ZnF3W4SYRKq2gAHs=",
        version = "v0.0.0-20230602150820-91b7bce49751",
    )
    go_repository(
        name = "com_github_google_renameio",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/renameio",
        sum = "h1:GOZbcHa3HfsPKPlmyPyN2KEohoMXOhdMbHrvbpl2QaA=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_google_s2a_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/s2a-go",
        sum = "h1:60BLSyTrOV4/haCDW4zb1guZItoSq8foHCXrAnjBo/o=",
        version = "v0.1.7",
    )
    go_repository(
        name = "com_github_google_shlex",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/shlex",
        sum = "h1:El6M4kTTCOh6aBiKaUGG7oYTSPP8MxqL4YI3kZKwcP4=",
        version = "v0.0.0-20191202100458-e7afc7fbc510",
    )
    go_repository(
        name = "com_github_google_slothfs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/slothfs",
        sum = "h1:iuModVoTuW2lBUobX9QBgqD+ipHbWKug6n8qkJfDtUE=",
        version = "v0.0.0-20190717100203-59c1163fd173",
    )
    go_repository(
        name = "com_github_google_uuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/uuid",
        sum = "h1:1p67kYwdtXjb0gL0BPiP1Av9wiZPo5A8z2cWkTZ+eyU=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_googleapis_enterprise_certificate_proxy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/googleapis/enterprise-certificate-proxy",
        sum = "h1:Vie5ybvEvT75RniqhfFxPRy3Bf7vr3h0cechB90XaQs=",
        version = "v0.3.2",
    )
    go_repository(
        name = "com_github_googleapis_gax_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/googleapis/gax-go",
        sum = "h1:j0GKcs05QVmm7yesiZq2+9cxHkNK9YM6zKx4D2qucQU=",
        version = "v2.0.0+incompatible",
    )
    go_repository(
        name = "com_github_googleapis_gax_go_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/googleapis/gax-go/v2",
        sum = "h1:A+gCJKdRfqXkr+BIRGtZLibNXf0m1f9E4HG56etFpas=",
        version = "v2.12.0",
    )
    go_repository(
        name = "com_github_googleapis_gnostic",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/googleapis/gnostic",
        replace = "github.com/googleapis/gnostic",
        sum = "h1:9fHAtK0uDfpveeqqo1hkEZJcFvYXAiCN3UutL8F9xHw=",
        version = "v0.5.5",
    )
    go_repository(
        name = "com_github_googlecloudplatform_opentelemetry_operations_go_detectors_gcp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp",
        sum = "h1:tk85AYGwOf6VNtoOQi8w/kVDi2vmPxp3/OU2FsUpdcA=",
        version = "v1.20.0",
    )
    go_repository(
        name = "com_github_googlecloudplatform_opentelemetry_operations_go_exporter_metric",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric",
        sum = "h1:VahL5SjDdCas8mMKARolw2vvBsuLc5oV7XNSbxeMQP8=",
        version = "v0.41.0",
    )
    go_repository(
        name = "com_github_googlecloudplatform_opentelemetry_operations_go_exporter_trace",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace",
        sum = "h1:DwGeS/9k9xdpnvVQuJF+L9bYNFvBCmCWlDA8d8opoZY=",
        version = "v1.17.0",
    )
    go_repository(
        name = "com_github_googlecloudplatform_opentelemetry_operations_go_internal_cloudmock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock",
        sum = "h1:ZJwvlTjB8GycSRpysdcRv3FztommLDUfgii0VUUp5ys=",
        version = "v0.41.0",
    )
    go_repository(
        name = "com_github_googlecloudplatform_opentelemetry_operations_go_internal_resourcemapping",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping",
        sum = "h1:MWQ81b2TkSLbDpLINiKdZdoht1VMEHCKr4BSZpb/KQ8=",
        version = "v0.41.0",
    )
    go_repository(
        name = "com_github_gophercloud_gophercloud",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gophercloud/gophercloud",
        sum = "h1:9nTGx0jizmHxDobe4mck89FyQHVyA3CaXLIUSGJjP9k=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_gopherjs_gopherjs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gopherjs/gopherjs",
        sum = "h1:fQnZVsXk8uxXIStYb0N4bGk7jeyTalG/wsZjQ25dO0g=",
        version = "v1.17.2",
    )
    go_repository(
        name = "com_github_gopherjs_gopherwasm",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gopherjs/gopherwasm",
        sum = "h1:fA2uLoctU5+T3OhOn2vYP0DVT6pxc7xhTlBB1paATqQ=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_gordonklaus_ineffassign",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gordonklaus/ineffassign",
        sum = "h1:vc7Dmrk4JwS0ZPS6WZvWlwDflgDTA26jItmbSj83nug=",
        version = "v0.0.0-20200309095847-7953dde2c7bf",
    )
    go_repository(
        name = "com_github_gorilla_context",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/context",
        sum = "h1:AWwleXJkX/nhcU9bZSnZoi3h/qGYqQAGhq6zZe/aQW8=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_gorilla_css",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/css",
        sum = "h1:BQqNyPTi50JCFMTw/b67hByjMVXZRwGha6wxVGkeihY=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_gorilla_feeds",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/feeds",
        sum = "h1:HwKXxqzcRNg9to+BbvJog4+f3s/xzvtZXICcQGutYfY=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_gorilla_handlers",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/handlers",
        sum = "h1:9lRY6j8DEeeBT10CvO9hGW0gmky0BprnvDI5vfhUHH4=",
        version = "v1.5.1",
    )
    go_repository(
        name = "com_github_gorilla_mux",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/mux",
        sum = "h1:i40aqfkR1h2SlN9hojwV5ZA91wcXFOvkdNIeFDP5koI=",
        version = "v1.8.0",
    )
    go_repository(
        name = "com_github_gorilla_schema",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/schema",
        sum = "h1:YufUaxZYCKGFuAq3c96BOhjgd5nmXiOY9NGzF247Tsc=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_gorilla_securecookie",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/securecookie",
        sum = "h1:miw7JPhV+b/lAHSXz4qd/nN9jRiAFV5FwjeKyCS8BvQ=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_gorilla_sessions",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/sessions",
        sum = "h1:DHd3rPN5lE3Ts3D8rKkQ8x/0kqfeNmBAaiSi+o7FsgI=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_gorilla_websocket",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/websocket",
        sum = "h1:PPwGk2jz7EePpoHN/+ClbZu8SPxiqlu12wZP/3sWmnc=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_gosimple_slug",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gosimple/slug",
        sum = "h1:xzuhj7G7cGtd34NXnW/yF0l+AGNfWqwgh/IXgFy7dnc=",
        version = "v1.12.0",
    )
    go_repository(
        name = "com_github_gosimple_unidecode",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gosimple/unidecode",
        sum = "h1:hZzFTMMqSswvf0LBJZCZgThIZrpDHFXux9KeGmn6T/o=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_goware_urlx",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/goware/urlx",
        sum = "h1:BbvKl8oiXtJAzOzMqAQ0GfIhf96fKeNEZfm9ocNSUBI=",
        version = "v0.3.1",
    )
    go_repository(
        name = "com_github_grafana_regexp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/grafana/regexp",
        sum = "h1:7aN5cccjIqCLTzedH7MZzRZt5/lsAHch6Z3L2ZGn5FA=",
        version = "v0.0.0-20221123153739-15dc172cd2db",
    )
    go_repository(
        name = "com_github_grafana_tools_sdk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/grafana-tools/sdk",
        sum = "h1:PXZQA2WCxe85Tnn+WEvr8fDpfwibmEPgfgFEaC87G24=",
        version = "v0.0.0-20220919052116-6562121319fc",
    )
    go_repository(
        name = "com_github_graph_gophers_graphql_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/graph-gophers/graphql-go",
        sum = "h1:fDqblo50TEpD0LY7RXk/LFVYEVqo3+tXMNMPSVXA1yc=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_graphql_go_graphql",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/graphql-go/graphql",
        sum = "h1:p7/Ou/WpmulocJeEx7wjQy611rtXGQaAcXGqanuMMgc=",
        version = "v0.8.1",
    )
    go_repository(
        name = "com_github_gregjones_httpcache",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gregjones/httpcache",
        sum = "h1:+ngKgrYPPJrOjhax5N+uePQ0Fh1Z7PheYoUI/0nzkPA=",
        version = "v0.0.0-20190611155906-901d90724c79",
    )
    go_repository(
        name = "com_github_grpc_ecosystem_go_grpc_middleware",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/grpc-ecosystem/go-grpc-middleware",
        sum = "h1:+9834+KizmvFV7pXQGSXQTsaWhq2GjuNUt0aUU0YBYw=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_grpc_ecosystem_go_grpc_middleware_providers_prometheus",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus",
        sum = "h1:mdLirNAJBxnGgyB6pjZLcs6ue/6eZGBui6gXspfq4ks=",
        version = "v1.0.0-rc.0",
    )
    go_repository(
        name = "com_github_grpc_ecosystem_go_grpc_middleware_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/grpc-ecosystem/go-grpc-middleware/v2",
        sum = "h1:2cz5kSrxzMYHiWOBbKj8itQm+nRykkB8aMv4ThcHYHA=",
        version = "v2.0.0",
    )
    go_repository(
        name = "com_github_grpc_ecosystem_go_grpc_prometheus",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/grpc-ecosystem/go-grpc-prometheus",
        sum = "h1:Ovs26xHkKqVztRpIrF/92BcuyuQ/YW4NSIpoGtfXNho=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_grpc_ecosystem_grpc_gateway",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/grpc-ecosystem/grpc-gateway",
        sum = "h1:gmcG1KaJ57LophUzW0Hy8NmPhnMZb4M0+kPpLofRdBo=",
        version = "v1.16.0",
    )
    go_repository(
        name = "com_github_grpc_ecosystem_grpc_gateway_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/grpc-ecosystem/grpc-gateway/v2",
        sum = "h1:RoziI+96HlQWrbaVhgOOdFYUHtX81pwA6tCgDS9FNRo=",
        version = "v2.16.1",
    )
    go_repository(
        name = "com_github_hanwen_go_fuse_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hanwen/go-fuse/v2",
        sum = "h1:ibbzF2InxMOS+lLCphY9PHNKPURDUBNKaG6ErSq8gJQ=",
        version = "v2.1.1-0.20220112183258-f57e95bda82d",
    )
    go_repository(
        name = "com_github_hashicorp_consul_api",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/consul/api",
        sum = "h1:WYONYL2rxTXtlekAqblR2SCdJsizMDIj/uXb5wNy9zU=",
        version = "v1.15.3",
    )
    go_repository(
        name = "com_github_hashicorp_consul_sdk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/consul/sdk",
        sum = "h1:OJtKBtEjboEZvG6AOUdh4Z1Zbyu0WcxQ0qatRrZHTVU=",
        version = "v0.8.0",
    )
    go_repository(
        name = "com_github_hashicorp_cronexpr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/cronexpr",
        sum = "h1:NJZDd87hGXjoZBdvyCF9mX4DCq5Wy7+A/w+A7q0wn6c=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_hashicorp_errwrap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/errwrap",
        sum = "h1:OxrOeh75EUXMY8TBjag2fzXGZ40LB6IKw45YeGUDY2I=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_hashicorp_go_cleanhttp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-cleanhttp",
        sum = "h1:035FKYIWjmULyFRBKPs8TBQoi0x6d9G4xc9neXJWAZQ=",
        version = "v0.5.2",
    )
    go_repository(
        name = "com_github_hashicorp_go_hclog",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-hclog",
        sum = "h1:La19f8d7WIlm4ogzNHB0JGqs5AUDAZ2UfCY4sJXcJdM=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_hashicorp_go_immutable_radix",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-immutable-radix",
        sum = "h1:DKHmCUm2hRBK510BaiZlwvpD40f8bJFeZnpfm2KLowc=",
        version = "v1.3.1",
    )
    go_repository(
        name = "com_github_hashicorp_go_msgpack",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-msgpack",
        sum = "h1:zKjpN5BK/P5lMYrLmBHdBULWbJ0XpYR+7NGzqkZzoD4=",
        version = "v0.5.3",
    )
    go_repository(
        name = "com_github_hashicorp_go_multierror",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-multierror",
        sum = "h1:H5DkEtf6CXdFp0N0Em5UCwQpXMWke8IA0+lD48awMYo=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_hashicorp_go_net",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go.net",
        sum = "h1:sNCoNyDEvN1xa+X0baata4RdcpKwcMS6DH+xwfqPgjw=",
        version = "v0.0.1",
    )
    go_repository(
        name = "com_github_hashicorp_go_plugin",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-plugin",
        sum = "h1:4OtAfUGbnKC6yS48p0CtMX2oFYtzFZVv6rok3cRWgnE=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_hashicorp_go_retryablehttp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-retryablehttp",
        sum = "h1:ZQgVdpTdAL7WpMIwLzCfbalOcSUdkDZnpUv3/+BxzFA=",
        version = "v0.7.4",
    )
    go_repository(
        name = "com_github_hashicorp_go_rootcerts",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-rootcerts",
        sum = "h1:jzhAVGtqPKbwpyCPELlgNWhE1znq+qwJtW5Oi2viEzc=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_hashicorp_go_slug",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-slug",
        sum = "h1:lYhmKXXonP4KGSz3JBmks6YpDRjP1cMA/yvcoPxoNw8=",
        version = "v0.12.1",
    )
    go_repository(
        name = "com_github_hashicorp_go_sockaddr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-sockaddr",
        sum = "h1:ztczhD1jLxIRjVejw8gFomI1BQZOe2WoVOu0SyteCQc=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_hashicorp_go_syslog",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-syslog",
        sum = "h1:KaodqZuhUoZereWVIYmpUgZysurB1kBLX2j0MwMrUAE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_hashicorp_go_tfe",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-tfe",
        sum = "h1:5G84t70g70xWK5U0WaYb8wLfbC5fR6dvawqCkmMN5X0=",
        version = "v1.32.1",
    )
    go_repository(
        name = "com_github_hashicorp_go_uuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-uuid",
        sum = "h1:2gKiV6YVmrJ1i2CKKa9obLvRieoRGviZFL26PcT/Co8=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_hashicorp_go_version",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-version",
        sum = "h1:feTTfFNnjP967rlCxM/I9g701jU+RN74YKx2mOkIeek=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_hashicorp_golang_lru",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/golang-lru",
        sum = "h1:YDjusn29QI/Das2iO9M0BHnIbxPeyuCHsjMW+lJfyTc=",
        version = "v0.5.4",
    )
    go_repository(
        name = "com_github_hashicorp_golang_lru_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/golang-lru/v2",
        sum = "h1:Dwmkdr5Nc/oBiXgJS3CDHNhJtIHkuZ3DZF5twqnfBdU=",
        version = "v2.0.2",
    )
    go_repository(
        name = "com_github_hashicorp_hcl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/hcl",
        sum = "h1:0Anlzjpi4vEasTeNFn2mLJgTSwt0+6sfsiTG8qcWGx4=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_hashicorp_jsonapi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/jsonapi",
        sum = "h1:9ARUJJ1VVynB176G1HCwleORqCaXm/Vx0uUi0dL26I0=",
        version = "v0.0.0-20210826224640-ee7dae0fb22d",
    )
    go_repository(
        name = "com_github_hashicorp_logutils",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/logutils",
        sum = "h1:dLEQVugN8vlakKOUE3ihGLTZJRB4j+M2cdTm/ORI65Y=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_hashicorp_mdns",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/mdns",
        sum = "h1:sY0CMhFmjIPDMlTB+HfymFHCaYLhgifZ0QhjaYKD/UQ=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_hashicorp_memberlist",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/memberlist",
        sum = "h1:8+567mCcFDnS5ADl7lrpxPMWiFCElyUEeW0gtj34fMA=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_hashicorp_nomad_api",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/nomad/api",
        sum = "h1:jKwXhVS4F7qk0g8laz+Anz0g/6yaSJ3HqmSAuSNLUcA=",
        version = "v0.0.0-20221102143410-8a95f1239005",
    )
    go_repository(
        name = "com_github_hashicorp_serf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/serf",
        sum = "h1:hkdgbqizGQHuU5IPqYM1JdSMV8nKfpuOnZYXssk9muY=",
        version = "v0.9.7",
    )
    go_repository(
        name = "com_github_hashicorp_terraform_cdk_go_cdktf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/terraform-cdk-go/cdktf",
        sum = "h1:VtehUlxej6l1TV9NRjO5jHlikMfV39Qj6bTsR4ps5do=",
        version = "v0.17.3",
    )
    go_repository(
        name = "com_github_hashicorp_terraform_registry_address",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/terraform-registry-address",
        sum = "h1:92LUg03NhfgZv44zpNTLBGIbiyTokQCDcdH5BhVHT3s=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_hashicorp_terraform_svchost",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/terraform-svchost",
        sum = "h1:Zj6fR5wnpOHnJUmLyWozjMeDaVuE+cstMPj41/eKmSQ=",
        version = "v0.0.1",
    )
    go_repository(
        name = "com_github_hashicorp_vault_api",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/vault/api",
        sum = "h1:j08Or/wryXT4AcHj1oCbMd7IijXcKzYUGw59LGu9onU=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_hashicorp_vault_sdk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/vault/sdk",
        sum = "h1:mOEPeOhT7jl0J4AMl1E705+BcmeRs1VmKNb9F0sMLy8=",
        version = "v0.1.13",
    )
    go_repository(
        name = "com_github_hashicorp_yamux",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/yamux",
        sum = "h1:kJCB4vdITiW1eC1vq2e6IsrXKrZit1bv/TDYFGMp4BQ=",
        version = "v0.0.0-20181012175058-2f1d1f20f75d",
    )
    go_repository(
        name = "com_github_hdrhistogram_hdrhistogram_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/HdrHistogram/hdrhistogram-go",
        sum = "h1:5IcZpTvzydCQeHzK4Ef/D5rrSqwxob0t8PQPMybUNFM=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_hetznercloud_hcloud_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hetznercloud/hcloud-go",
        sum = "h1:WCmFAhLRooih2QHAsbCbEdpIHnshQQmrPqsr3rHE1Ow=",
        version = "v1.35.3",
    )
    go_repository(
        name = "com_github_hexops_autogold",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hexops/autogold",
        sum = "h1:YgxF9OHWbEIUjhDbpnLhgVsjUDsiHDTyDfy2lrfdlzo=",
        version = "v1.3.1",
    )
    go_repository(
        name = "com_github_hexops_autogold_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hexops/autogold/v2",
        sum = "h1:5s9J6CROngFPkgowSkV20bIflBrImSdDqIpoXJeZSkU=",
        version = "v2.1.0",
    )
    go_repository(
        name = "com_github_hexops_gotextdiff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hexops/gotextdiff",
        sum = "h1:gitA9+qJrrTCsiCl7+kh75nPqQt1cx4ZkudSTLoUqJM=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_hexops_valast",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hexops/valast",
        sum = "h1:oBoGERMJh6UZdRc6cduE1CTPK+VAdXA59Y1HFgu3sm0=",
        version = "v1.4.3",
    )
    go_repository(
        name = "com_github_hhatto_gocloc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hhatto/gocloc",
        sum = "h1:deh3Xb1uqiySNgOccMNYb3HbKsUoQDzsZRpfQmbTIhs=",
        version = "v0.4.2",
    )
    go_repository(
        name = "com_github_hjson_hjson_go_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hjson/hjson-go/v4",
        sum = "h1:wlm6IYYqHjOdXH1gHev4VoXCaW20HdQAGCxdOEEg2cs=",
        version = "v4.0.0",
    )
    go_repository(
        name = "com_github_honeycombio_libhoney_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/honeycombio/libhoney-go",
        sum = "h1:TECEltZ48K6J4NG1JVYqmi0vCJNnHYooFor83fgKesA=",
        version = "v1.15.8",
    )
    go_repository(
        name = "com_github_hpcloud_tail",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hpcloud/tail",
        sum = "h1:nfCOvKYfkgYP8hkirhJocXT2+zOD8yUNjXaWfTlyFKI=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_huandu_xstrings",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/huandu/xstrings",
        sum = "h1:D17IlohoQq4UcpqD7fDk80P7l+lwAmlFaBHgOipl2FU=",
        version = "v1.4.0",
    )
    go_repository(
        name = "com_github_hydrogen18_memlistener",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hydrogen18/memlistener",
        sum = "h1:JR7eDj8HD6eXrc5fWLbSUnfcQFL06PYvCc0DKQnWfaU=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_iancoleman_strcase",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/iancoleman/strcase",
        sum = "h1:nTXanmYxhfFAMjZL34Ov6gkzEsSJZ5DbhxWjvSASxEI=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_ianlancetaylor_demangle",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ianlancetaylor/demangle",
        sum = "h1:BA4a7pe6ZTd9F8kXETBoijjFJ/ntaa//1wiH9BZu4zU=",
        version = "v0.0.0-20230524184225-eabc099b10ab",
    )
    go_repository(
        name = "com_github_imdario_mergo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/imdario/mergo",
        sum = "h1:wwQJbIsHYGMUyLSPrEq1CT16AhnhNJQ51+4fdHUnCl4=",
        version = "v0.3.16",
    )
    go_repository(
        name = "com_github_in_toto_in_toto_golang",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/in-toto/in-toto-golang",
        sum = "h1:hb8bgwr0M2hGdDsLjkJ3ZqJ8JFLL/tgYdAxF/XEFBbY=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_inconshreveable_log15",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/inconshreveable/log15",
        sum = "h1:n1DqxAo4oWPMvH1+v+DLYlMCecgumhhgnxAPdqDIFHI=",
        version = "v0.0.0-20201112154412-8562bdadbbac",
    )
    go_repository(
        name = "com_github_inconshreveable_mousetrap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/inconshreveable/mousetrap",
        sum = "h1:wN+x4NVGpMsO7ErUn/mUI3vEoE6Jt13X2s0bqwp9tc8=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_invopop_jsonschema",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/invopop/jsonschema",
        sum = "h1:6ovsNSuvn9wEQVOyc72aycBMVQFKz7cPdMJn10CvzRI=",
        version = "v0.12.0",
    )
    go_repository(
        name = "com_github_ionos_cloud_sdk_go_v6",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ionos-cloud/sdk-go/v6",
        sum = "h1:vb6yqdpiqaytvreM0bsn2pXw+1YDvEk2RKSmBAQvgDQ=",
        version = "v6.1.3",
    )
    go_repository(
        name = "com_github_iris_contrib_schema",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/iris-contrib/schema",
        sum = "h1:CPSBLyx2e91H2yJzPuhGuifVRnZBBJ3pCOMbOvPZaTw=",
        version = "v0.0.6",
    )
    go_repository(
        name = "com_github_itchyny_gojq",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/itchyny/gojq",
        sum = "h1:YhLueoHhHiN4mkfM+3AyJV6EPcCxKZsOnYf+aVSwaQw=",
        version = "v0.12.11",
    )
    go_repository(
        name = "com_github_itchyny_timefmt_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/itchyny/timefmt-go",
        sum = "h1:G0INE2la8S6ru/ZI5JecgyzbbJNs5lG1RcBqa7Jm6GE=",
        version = "v0.1.5",
    )
    go_repository(
        name = "com_github_jackc_chunkreader_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/chunkreader/v2",
        sum = "h1:i+RDz65UE+mmpjTfyz0MoVTnzeYxroil2G82ki7MGG8=",
        version = "v2.0.1",
    )
    go_repository(
        name = "com_github_jackc_pgconn",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgconn",
        sum = "h1:vrbA9Ud87g6JdFWkHTJXppVce58qPIdP7N8y0Ml/A7Q=",
        version = "v1.14.0",
    )
    go_repository(
        name = "com_github_jackc_pgerrcode",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgerrcode",
        sum = "h1:s+4MhCQ6YrzisK6hFJUX53drDT4UsSW3DEhKn0ifuHw=",
        version = "v0.0.0-20220416144525-469b46aa5efa",
    )
    go_repository(
        name = "com_github_jackc_pgio",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgio",
        sum = "h1:g12B9UwVnzGhueNavwioyEEpAmqMe1E/BN9ES+8ovkE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_jackc_pgpassfile",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgpassfile",
        sum = "h1:/6Hmqy13Ss2zCq62VdNG8tM1wchn8zjSGOBJ6icpsIM=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_jackc_pgproto3_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgproto3/v2",
        sum = "h1:7eY55bdBeCz1F2fTzSz69QC+pG46jYq9/jtSPiJ5nn0=",
        version = "v2.3.2",
    )
    go_repository(
        name = "com_github_jackc_pgservicefile",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgservicefile",
        sum = "h1:L0QtFUgDarD7Fpv9jeVMgy/+Ec0mtnmYuImjTz6dtDA=",
        version = "v0.0.0-20231201235250-de7065d80cb9",
    )
    go_repository(
        name = "com_github_jackc_pgtype",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgtype",
        sum = "h1:y+xUdabmyMkJLyApYuPj38mW+aAIqCe5uuBB51rH3Vw=",
        version = "v1.14.0",
    )
    go_repository(
        name = "com_github_jackc_pgx_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgx/v4",
        sum = "h1:YP7G1KABtKpB5IHrO9vYwSrCOhs7p3uqhvhhQBptya0=",
        version = "v4.18.1",
    )
    go_repository(
        name = "com_github_jackc_pgx_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgx/v5",
        sum = "h1:5I9etrGkLrN+2XPCsi6XLlV5DITbSL/xBZdmAxFcXPI=",
        version = "v5.5.1",
    )
    go_repository(
        name = "com_github_jackc_puddle_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/puddle/v2",
        sum = "h1:RhxXJtFG022u4ibrCSMSiu5aOq1i77R3OHKNJj77OAk=",
        version = "v2.2.1",
    )
    go_repository(
        name = "com_github_jaytaylor_html2text",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jaytaylor/html2text",
        sum = "h1:QFQpJdgbON7I0jr2hYW7Bs+XV0qjc3d5tZoDnRFnqTg=",
        version = "v0.0.0-20211105163654-bc68cce691ba",
    )
    go_repository(
        name = "com_github_jbenet_go_context",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jbenet/go-context",
        sum = "h1:BQSFePA1RWJOlocH6Fxy8MmwDt+yVQYULKfN0RoTN8A=",
        version = "v0.0.0-20150711004518-d14ea06fba99",
    )
    go_repository(
        name = "com_github_jdxcode_netrc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jdxcode/netrc",
        sum = "h1:d4+I1YEKVmWZrgkt6jpXBnLgV2ZjO0YxEtLDdfIZfH4=",
        version = "v0.0.0-20210204082910-926c7f70242a",
    )
    go_repository(
        name = "com_github_jessevdk_go_flags",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jessevdk/go-flags",
        sum = "h1:1jKYvbxEjfUl0fmqTCOfonvskHHXMjBySTLW4y9LFvc=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_jhump_gopoet",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jhump/gopoet",
        sum = "h1:gYjOPnzHd2nzB37xYQZxj4EIQNpBrBskRqQQ3q4ZgSg=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_jhump_goprotoc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jhump/goprotoc",
        sum = "h1:Y1UgUX+txUznfqcGdDef8ZOVlyQvnV0pKWZH08RmZuo=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_jhump_protocompile",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jhump/protocompile",
        sum = "h1:BNuUg9k2EiJmlMwjoef3e8vZLHplbVw6DrjGFjLL+Yo=",
        version = "v0.0.0-20220216033700-d705409f108f",
    )
    go_repository(
        name = "com_github_jhump_protoreflect",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jhump/protoreflect",
        sum = "h1:uFlcJKZPLQd7rmOY/RrvBuUaYmAFnlFHKLivhO6cOy8=",
        version = "v1.12.1-0.20220417024638-438db461d753",
    )
    go_repository(
        name = "com_github_jinzhu_inflection",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jinzhu/inflection",
        sum = "h1:K317FqzuhWc8YvSVlFMCCUb36O/S9MCKRDI7QkRKD/E=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_jinzhu_now",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jinzhu/now",
        sum = "h1:/o9tlHleP7gOFmsnYNz3RGnqzefHA47wQpKrrdTIwXQ=",
        version = "v1.1.5",
    )
    go_repository(
        name = "com_github_jmespath_go_jmespath",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jmespath/go-jmespath",
        sum = "h1:BEgLn5cpjn8UN1mAw4NjwDrS35OdebyEtFe+9YPoQUg=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_jmespath_go_jmespath_internal_testify",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jmespath/go-jmespath/internal/testify",
        sum = "h1:shLQSRRSCCPj3f2gpwzGwWFoC7ycTf1rcQZHOlsJ6N8=",
        version = "v1.5.1",
    )
    go_repository(
        name = "com_github_johncgriffin_overflow",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/JohnCGriffin/overflow",
        sum = "h1:RGWPOewvKIROun94nF7v2cua9qP+thov/7M50KEoeSU=",
        version = "v0.0.0-20211019200055-46fa312c352c",
    )
    go_repository(
        name = "com_github_joho_godotenv",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/joho/godotenv",
        sum = "h1:3l4+N6zfMWnkbPEXKng2o2/MR5mSwTrBih4ZEkkz1lg=",
        version = "v1.4.0",
    )
    go_repository(
        name = "com_github_joker_jade",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Joker/jade",
        sum = "h1:Qbeh12Vq6BxURXT1qZBRHsDxeURB8ztcL6f3EXSGeHk=",
        version = "v1.1.3",
    )
    go_repository(
        name = "com_github_jonboulle_clockwork",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jonboulle/clockwork",
        sum = "h1:9BSCMi8C+0qdApAp4auwX0RkLGUjs956h0EkuQymUhg=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_jordan_wright_email",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jordan-wright/email",
        sum = "h1:jdpOPRN1zP63Td1hDQbZW73xKmzDvZHzVdNYxhnTMDA=",
        version = "v4.0.1-0.20210109023952-943e75fe5223+incompatible",
    )
    go_repository(
        name = "com_github_josharian_intern",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/josharian/intern",
        sum = "h1:vlS4z54oSdjm0bgjRigI+G1HpF+tI+9rE5LLzOg8HmY=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_jpillora_backoff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jpillora/backoff",
        sum = "h1:uvFg412JmmHBHw7iwprIxkPMI+sGQ4kzOWsMeHnm2EA=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_json_iterator_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/json-iterator/go",
        sum = "h1:PV8peI4a0ysnczrg+LtxykD8LfKY9ML6u2jnxaEnrnM=",
        version = "v1.1.12",
    )
    go_repository(
        name = "com_github_jstemmer_go_junit_report",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jstemmer/go-junit-report",
        sum = "h1:6QPYqodiu3GuPL+7mfx+NwDdp2eTkp9IfEUpgAwUN0o=",
        version = "v0.9.1",
    )
    go_repository(
        name = "com_github_jtolds_gls",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jtolds/gls",
        sum = "h1:xdiiI2gbIgH/gLH7ADydsJ1uDOEzR8yvV7C0MuV77Wo=",
        version = "v4.20.0+incompatible",
    )
    go_repository(
        name = "com_github_julienschmidt_httprouter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/julienschmidt/httprouter",
        sum = "h1:U0609e9tgbseu3rBINet9P48AI/D3oJs4dN7jwJOQ1U=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_k0kubun_go_ansi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/k0kubun/go-ansi",
        sum = "h1:qGQQKEcAR99REcMpsXCp3lJ03zYT1PkRd3kQGPn9GVg=",
        version = "v0.0.0-20180517002512-3bf9e2903213",
    )
    go_repository(
        name = "com_github_k0kubun_pp_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/k0kubun/pp/v3",
        sum = "h1:ifxtqJkRZhw3h554/z/8zm6AAbyO4LLKDlA5eV+9O8Q=",
        version = "v3.1.0",
    )
    go_repository(
        name = "com_github_k3a_html2text",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/k3a/html2text",
        sum = "h1:ks4hKSTdiTRsLr0DM771mI5TvsoG6zH7m1Ulv7eJRHw=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_karlseguin_expect",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/karlseguin/expect",
        sum = "h1:OF4mqjblc450v8nKARBS5Q0AweBNR0A+O3VjjpxwBrg=",
        version = "v1.0.7",
    )
    go_repository(
        name = "com_github_karlseguin_typed",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/karlseguin/typed",
        sum = "h1:ND0eDpwiUFIrm/n1ehxUyh/XNGs9zkYrLxtGqENSalY=",
        version = "v1.1.8",
    )
    go_repository(
        name = "com_github_karrick_godirwalk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/karrick/godirwalk",
        sum = "h1:lOpSw2vJP0y5eLBW906QwKsUK/fe/QDyoqM5rnnuPDY=",
        version = "v1.10.3",
    )
    go_repository(
        name = "com_github_kataras_blocks",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/blocks",
        sum = "h1:cF3RDY/vxnSRezc7vLFlQFTYXG/yAr1o7WImJuZbzC4=",
        version = "v0.0.7",
    )
    go_repository(
        name = "com_github_kataras_golog",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/golog",
        sum = "h1:isP8th4PJH2SrbkciKnylaND9xoTtfxv++NB+DF0l9g=",
        version = "v0.1.8",
    )
    go_repository(
        name = "com_github_kataras_iris_v12",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/iris/v12",
        sum = "h1:WzDY5nGuW/LgVaFS5BtTkW3crdSKJ/FEgWnxPnIVVLI=",
        version = "v12.2.0",
    )
    go_repository(
        name = "com_github_kataras_pio",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/pio",
        sum = "h1:kqreJ5KOEXGMwHAWHDwIl+mjfNCPhAwZPa8gK7MKlyw=",
        version = "v0.0.11",
    )
    go_repository(
        name = "com_github_kataras_sitemap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/sitemap",
        sum = "h1:w71CRMMKYMJh6LR2wTgnk5hSgjVNB9KL60n5e2KHvLY=",
        version = "v0.0.6",
    )
    go_repository(
        name = "com_github_kataras_tunnel",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/tunnel",
        sum = "h1:sCAqWuJV7nPzGrlb0os3j49lk2JhILT0rID38NHNLpA=",
        version = "v0.0.4",
    )
    go_repository(
        name = "com_github_kballard_go_shellquote",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kballard/go-shellquote",
        sum = "h1:Z9n2FFNUXsshfwJMBgNA0RU6/i7WVaAegv3PtuIHPMs=",
        version = "v0.0.0-20180428030007-95032a82bc51",
    )
    go_repository(
        name = "com_github_keegancsmith_rpc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/keegancsmith/rpc",
        sum = "h1:wGWOpjcNrZaY8GDYZJfvyxmlLljm3YQWF+p918DXtDk=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_keegancsmith_sqlf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/keegancsmith/sqlf",
        sum = "h1:b3DZm7eILJYkj4igLMjIvUM8fI4Ey4LCV5Wk8JGL+uA=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_keegancsmith_tmpfriend",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/keegancsmith/tmpfriend",
        sum = "h1:xa9SZfAid/jlS3kjwAvVDQFpe6t8SiS0Vl/H51BZYww=",
        version = "v0.0.0-20180423180255-86e88902a513",
    )
    go_repository(
        name = "com_github_kevinburke_ssh_config",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kevinburke/ssh_config",
        sum = "h1:x584FjTGwHzMwvHx18PXxbBVzfnxogHaAReU4gf13a4=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_kevinmbeaulieu_eq_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kevinmbeaulieu/eq-go",
        sum = "h1:AQgYHURDOmnVJ62jnEk0W/7yFKEn+Lv8RHN6t7mB0Zo=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_keybase_go_crypto",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/keybase/go-crypto",
        sum = "h1:cTxwSmnaqLoo+4tLukHoB9iqHOu3LmLhRmgUxZo6Vp4=",
        version = "v0.0.0-20200123153347-de78d2cb44f4",
    )
    go_repository(
        name = "com_github_khan_genqlient",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Khan/genqlient",
        sum = "h1:TMZJ+tl/BpbmGyIBiXzKzUftDhw4ZWxQZ+1ydn0gyII=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_kisielk_errcheck",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kisielk/errcheck",
        sum = "h1:e8esj/e4R+SAOwFwN+n3zr0nYeCyeweozKfO23MvHzY=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_kisielk_gotool",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kisielk/gotool",
        sum = "h1:AV2c/EiW3KqPNT9ZKl07ehoAGi4C5/01Cfbblndcapg=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_klauspost_asmfmt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/klauspost/asmfmt",
        sum = "h1:4Ri7ox3EwapiOjCki+hw14RyKk201CN4rzyCJRFLpK4=",
        version = "v1.3.2",
    )
    go_repository(
        name = "com_github_klauspost_compress",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/klauspost/compress",
        sum = "h1:2mk3MPGNzKyxErAw8YaohYh69+pa4sIQSC0fPGCFR9I=",
        version = "v1.16.7",
    )
    go_repository(
        name = "com_github_klauspost_cpuid_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/klauspost/cpuid/v2",
        sum = "h1:acbojRNwl3o09bUq+yDCtZFc1aiwaAAxtcn8YkZXnvk=",
        version = "v2.2.4",
    )
    go_repository(
        name = "com_github_klauspost_pgzip",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/klauspost/pgzip",
        sum = "h1:qnWYvvKqedOF2ulHpMG72XQol4ILEJ8k2wwRl/Km8oE=",
        version = "v1.2.5",
    )
    go_repository(
        name = "com_github_kljensen_snowball",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kljensen/snowball",
        sum = "h1:6DZLCcZeL0cLfodx+Md4/OLC6b/bfurWUOUGs1ydfOU=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_knadh_koanf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/knadh/koanf",
        sum = "h1:q2TSd/3Pyc/5yP9ldIrSdIz26MCcyNQzW0pEAugLPNs=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_knadh_koanf_maps",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/knadh/koanf/maps",
        sum = "h1:G5TjmUh2D7G2YWf5SQQqSiHRJEjaicvU0KpypqB3NIs=",
        version = "v0.1.1",
    )
    go_repository(
        name = "com_github_knadh_koanf_providers_confmap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/knadh/koanf/providers/confmap",
        sum = "h1:gOkxhHkemwG4LezxxN8DMOFopOPghxRVp7JbIvdvqzU=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_knadh_koanf_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/knadh/koanf/v2",
        sum = "h1:1dYGITt1I23x8cfx8ZnldtezdyaZtfAuRtIFOiRzK7g=",
        version = "v2.0.1",
    )
    go_repository(
        name = "com_github_kolo_xmlrpc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kolo/xmlrpc",
        sum = "h1:udzkj9S/zlT5X367kqJis0QP7YMxobob6zhzq6Yre00=",
        version = "v0.0.0-20220921171641-a4b6fa1dd06b",
    )
    go_repository(
        name = "com_github_konsorten_go_windows_terminal_sequences",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/konsorten/go-windows-terminal-sequences",
        sum = "h1:DB17ag19krx9CFsz4o3enTrPXyIXCl+2iCXH/aMAp9s=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_kr_fs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kr/fs",
        sum = "h1:Jskdu9ieNAYnjxsi0LbQp1ulIKZV1LAFgK1tWhpZgl8=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_kr_logfmt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kr/logfmt",
        sum = "h1:T+h1c/A9Gawja4Y9mFVWj2vyii2bbUNDw3kt9VxK2EY=",
        version = "v0.0.0-20140226030751-b84e30acd515",
    )
    go_repository(
        name = "com_github_kr_pretty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kr/pretty",
        sum = "h1:flRD4NNwYAUpkphVc1HcthR4KEIFJ65n8Mw5qdRn3LE=",
        version = "v0.3.1",
    )
    go_repository(
        name = "com_github_kr_pty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kr/pty",
        sum = "h1:hyz3dwM5QLc1Rfoz4FuWJQG5BN7tc6K1MndAUnGpQr4=",
        version = "v1.1.5",
    )
    go_repository(
        name = "com_github_kr_text",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kr/text",
        sum = "h1:5Nx0Ya0ZqY2ygV366QzturHI13Jq95ApcVaJBhpS+AY=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_kylelemons_godebug",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kylelemons/godebug",
        sum = "h1:RPNrshWIDI6G2gRW9EHilWtl7Z6Sb1BR0xunSBf0SNc=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_labstack_echo_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/labstack/echo/v4",
        sum = "h1:5CiyngihEO4HXsz3vVsJn7f8xAlWwRr3aY6Ih280ZKA=",
        version = "v4.10.0",
    )
    go_repository(
        name = "com_github_labstack_gommon",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/labstack/gommon",
        sum = "h1:y7cvthEAEbU0yHOf4axH8ZG2NH8knB9iNSoTO8dyIk8=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_leodido_go_urn",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/leodido/go-urn",
        sum = "h1:BqpAaACuzVSgi/VLzGZIobT2z4v53pjosyNd9Yv6n/w=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_lib_pq",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lib/pq",
        sum = "h1:YXG7RB+JIjhP29X+OtkiDnYaXQwpS4JEWq7dtCCRUEw=",
        version = "v1.10.9",
    )
    go_repository(
        name = "com_github_libdns_libdns",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/libdns/libdns",
        sum = "h1:Wu59T7wSHRgtA0cfxC+n1c/e+O3upJGWytknkmFEDis=",
        version = "v0.2.1",
    )
    go_repository(
        name = "com_github_linode_linodego",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/linode/linodego",
        sum = "h1:+lxNZw4avRxhCqGjwfPgQ2PvMT+vOL0OMsTdzixR7hQ=",
        version = "v1.9.3",
    )
    go_repository(
        name = "com_github_logrusorgru_aurora_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/logrusorgru/aurora/v3",
        sum = "h1:R6zcoZZbvVcGMvDCKo45A9U/lzYyzl5NfYIvznmDfE4=",
        version = "v3.0.0",
    )
    go_repository(
        name = "com_github_lucasb_eyer_go_colorful",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lucasb-eyer/go-colorful",
        sum = "h1:1nnpGOrhyZZuNyfu1QjKiUICQ74+3FNCN69Aj6K7nkY=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_lufia_plan9stats",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lufia/plan9stats",
        sum = "h1:6E+4a0GO5zZEnZ81pIr0yLvtUWk2if982qA3F3QD6H4=",
        version = "v0.0.0-20211012122336-39d0f177ccd0",
    )
    go_repository(
        name = "com_github_lyft_protoc_gen_star_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lyft/protoc-gen-star/v2",
        sum = "h1:/3+/2sWyXeMLzKd1bX+ixWKgEMsULrIivpDsuaF441o=",
        version = "v2.0.3",
    )
    go_repository(
        name = "com_github_machinebox_graphql",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/machinebox/graphql",
        sum = "h1:dWKpJligYKhYKO5A2gvNhkJdQMNZeChZYyBbrZkBZfo=",
        version = "v0.2.2",
    )
    go_repository(
        name = "com_github_magiconair_properties",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/magiconair/properties",
        sum = "h1:5ibWZ6iY0NctNGWo87LalDlEZ6R41TqbbDamhfG/Qzo=",
        version = "v1.8.6",
    )
    go_repository(
        name = "com_github_mailgun_raymond_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mailgun/raymond/v2",
        sum = "h1:5dmlB680ZkFG2RN/0lvTAghrSxIESeu9/2aeDqACtjw=",
        version = "v2.0.48",
    )
    go_repository(
        name = "com_github_mailru_easyjson",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mailru/easyjson",
        sum = "h1:UGYAvKxe3sBsEDzO8ZeWOSlIQfWFlxbzLZe7hwFURr0=",
        version = "v0.7.7",
    )
    go_repository(
        name = "com_github_markbates_going",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/markbates/going",
        sum = "h1:DQw0ZP7NbNlFGcKbcE/IVSOAFzScxRtLpd0rLMzLhq0=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_markbates_goth",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/markbates/goth",
        sum = "h1:X5QUUHLP5puJ4dhoPKkV3PhDIvvQEzsfVxsUmDNSJ28=",
        version = "v1.73.0",
    )
    go_repository(
        name = "com_github_markbates_oncer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/markbates/oncer",
        sum = "h1:JgVTCPf0uBVcUSWpyXmGpgOc62nK5HWUBKAGc3Qqa5k=",
        version = "v0.0.0-20181203154359-bf2de49a0be2",
    )
    go_repository(
        name = "com_github_markbates_safe",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/markbates/safe",
        sum = "h1:yjZkbvRM6IzKj9tlu/zMJLS0n/V351OZWRnF3QfaUxI=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_masterminds_goutils",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Masterminds/goutils",
        sum = "h1:5nUrii3FMTL5diU80unEVvNevw1nH4+ZV4DSLVJLSYI=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_masterminds_semver",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Masterminds/semver",
        sum = "h1:H65muMkzWKEuNDnfl9d70GUjFniHKHRbFPGBuZ3QEww=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_masterminds_semver_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Masterminds/semver/v3",
        sum = "h1:RN9w6+7QoMeJVGyfmbcgs28Br8cvmnucEXnY0rYXWg0=",
        version = "v3.2.1",
    )
    go_repository(
        name = "com_github_masterminds_sprig",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Masterminds/sprig",
        sum = "h1:z4yfnGrZ7netVz+0EDJ0Wi+5VZCSYp4Z0m2dk6cEM60=",
        version = "v2.22.0+incompatible",
    )
    go_repository(
        name = "com_github_masterminds_sprig_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Masterminds/sprig/v3",
        sum = "h1:17jRggJu518dr3QaafizSXOjKYp94wKfABxUmyxvxX8=",
        version = "v3.2.2",
    )
    go_repository(
        name = "com_github_matryer_is",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/matryer/is",
        sum = "h1:92UTHpy8CDwaJ08GqLDzhhuixiBUUD1p3AU6PHddz4A=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_matryer_moq",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/matryer/moq",
        sum = "h1:Q06vEqnBYjjfx5KKgHfYRKE/lvlRu+Nj+xodG4YdHnU=",
        version = "v0.2.3",
    )
    go_repository(
        name = "com_github_mattermost_xml_roundtrip_validator",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattermost/xml-roundtrip-validator",
        sum = "h1:RXbVD2UAl7A7nOTR4u7E3ILa4IbtvKBHw64LDsmu9hU=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_mattn_go_colorable",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-colorable",
        sum = "h1:fFA4WZxdEF4tXPZVKMLwD8oUnCTTo08duU7wxecdEvA=",
        version = "v0.1.13",
    )
    go_repository(
        name = "com_github_mattn_go_isatty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-isatty",
        sum = "h1:xfD0iDuEKnDkl03q4limB+vH+GxLEtL/jb4xVJSWWEY=",
        version = "v0.0.20",
    )
    go_repository(
        name = "com_github_mattn_go_runewidth",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-runewidth",
        sum = "h1:+xnbZSEeDbOIg5/mE6JF0w6n9duR1l3/WmbinWVwUuU=",
        version = "v0.0.14",
    )
    go_repository(
        name = "com_github_mattn_go_sqlite3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-sqlite3",
        sum = "h1:yOQRA0RpS5PFz/oikGwBEqvAWhWg5ufRz4ETLjwpU1Y=",
        version = "v1.14.16",
    )
    go_repository(
        name = "com_github_matttproud_golang_protobuf_extensions",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/matttproud/golang_protobuf_extensions",
        sum = "h1:mmDVorXM7PCGKw94cs5zkfA9PSy5pEvNWRP0ET0TIVo=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_mholt_acmez",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mholt/acmez",
        sum = "h1:N3cE4Pek+dSolbsofIkAYz6H1d3pE+2G0os7QHslf80=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_mholt_archiver_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mholt/archiver/v3",
        sum = "h1:rDjOBX9JSF5BvoJGvjqK479aL70qh9DIpZCl+k7Clwo=",
        version = "v3.5.1",
    )
    go_repository(
        name = "com_github_microcosm_cc_bluemonday",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/microcosm-cc/bluemonday",
        sum = "h1:SMZe2IGa0NuHvnVNAZ+6B38gsTbi5e4sViiWJyDDqFY=",
        version = "v1.0.23",
    )
    go_repository(
        name = "com_github_microsoft_go_mssqldb",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/microsoft/go-mssqldb",
        sum = "h1:mM3gYdVwEPFrlg/Dvr2DNVEgYFG7L42l+dGc67NNNpc=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_microsoft_go_winio",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Microsoft/go-winio",
        sum = "h1:9/kr64B9VUZrLm5YYwbGtUJnMgqWVOdUAXu6Migciow=",
        version = "v0.6.1",
    )
    go_repository(
        name = "com_github_microsoft_hcsshim",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Microsoft/hcsshim",
        sum = "h1:lf7xxK2+Ikbj9sVf2QZsouGjRjEp2STj1yDHgoVtU5k=",
        version = "v0.9.8",
    )
    go_repository(
        name = "com_github_miekg_dns",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/miekg/dns",
        sum = "h1:DQUfb9uc6smULcREF09Uc+/Gd46YWqJd5DbpPE9xkcA=",
        version = "v1.1.50",
    )
    go_repository(
        name = "com_github_minio_asm2plan9s",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/minio/asm2plan9s",
        sum = "h1:AMFGa4R4MiIpspGNG7Z948v4n35fFGB3RR3G/ry4FWs=",
        version = "v0.0.0-20200509001527-cdd76441f9d8",
    )
    go_repository(
        name = "com_github_minio_c2goasm",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/minio/c2goasm",
        sum = "h1:+n/aFZefKZp7spd8DFdX7uMikMLXX4oubIzJF4kv/wI=",
        version = "v0.0.0-20190812172519-36a3d3bbc4f3",
    )
    go_repository(
        name = "com_github_minio_md5_simd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/minio/md5-simd",
        sum = "h1:Gdi1DZK69+ZVMoNHRXJyNcxrMA4dSxoYHZSQbirFg34=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_minio_minio_go_v7",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/minio/minio-go/v7",
        sum = "h1:upnbu1jCGOqEvrGSpRauSN9ZG7RCHK7VHxXS8Vmg2zk=",
        version = "v7.0.39",
    )
    go_repository(
        name = "com_github_minio_sha256_simd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/minio/sha256-simd",
        sum = "h1:v1ta+49hkWZyvaKwrQB8elexRqm6Y0aMLjCNsrYxo6g=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mitchellh_cli",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/cli",
        sum = "h1:tEElEatulEHDeedTxwckzyYMA5c86fbmNIUL1hBIiTg=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_mitchellh_colorstring",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/colorstring",
        sum = "h1:62I3jR2EmQ4l5rM/4FEfDWcRD+abF5XlKShorW5LRoQ=",
        version = "v0.0.0-20190213212951-d06e56a500db",
    )
    go_repository(
        name = "com_github_mitchellh_copystructure",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/copystructure",
        sum = "h1:vpKXTN4ewci03Vljg/q9QvCGUDttBOGBIa15WveJJGw=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_mitchellh_go_homedir",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/go-homedir",
        sum = "h1:lukF9ziXFxDFPkA1vsr5zpc1XuPDn/wFntq5mG+4E0Y=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_mitchellh_go_testing_interface",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/go-testing-interface",
        sum = "h1:fzU/JVNcaqHQEcVFAKeR41fkiLdIPrefOvVG1VZ96U0=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mitchellh_go_wordwrap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/go-wordwrap",
        sum = "h1:TLuKupo69TCn6TQSyGxwI1EblZZEsQ0vMlAFQflz0v0=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_mitchellh_gox",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/gox",
        sum = "h1:lfGJxY7ToLJQjHHwi0EX6uYBdK78egf954SQl13PQJc=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_mitchellh_hashstructure",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/hashstructure",
        sum = "h1:P6P1hdjqAAknpY/M1CGipelZgp+4y9ja9kmUZPXP+H0=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_mitchellh_hashstructure_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/hashstructure/v2",
        sum = "h1:vGKWl0YJqUNxE8d+h8f6NJLcCJrgbhC4NcD46KavDd4=",
        version = "v2.0.2",
    )
    go_repository(
        name = "com_github_mitchellh_iochan",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/iochan",
        sum = "h1:C+X3KsSTLFVBr/tK1eYN/vs4rJcvsiLU338UhYPJWeY=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mitchellh_mapstructure",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/mapstructure",
        sum = "h1:BpfhmLKZf+SjVanKKhCgf3bg+511DmU9eDQTen7LLbY=",
        version = "v1.5.1-0.20220423185008-bf980b35cac4",
    )
    go_repository(
        name = "com_github_mitchellh_osext",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/osext",
        sum = "h1:2+myh5ml7lgEU/51gbeLHfKGNfgEQQIWrlbdaOsidbQ=",
        version = "v0.0.0-20151018003038-5e2d6d41470f",
    )
    go_repository(
        name = "com_github_mitchellh_reflectwalk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/reflectwalk",
        sum = "h1:G2LzWKi524PWgd3mLHV8Y5k7s6XUvT0Gef6zxSIeXaQ=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_mmcloughlin_avo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mmcloughlin/avo",
        sum = "h1:nAco9/aI9Lg2kiuROBY6BhCI/z0t5jEvJfjWbL8qXLU=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_moby_buildkit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/buildkit",
        sum = "h1:VYNdoKk5TVxN7k4RvZgdeM4GOyRvIi4Z8MXOY7xvyUs=",
        version = "v0.11.6",
    )
    go_repository(
        name = "com_github_moby_locker",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/locker",
        sum = "h1:fOXqR41zeveg4fFODix+1Ch4mj/gT0NE1XJbp/epuBg=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_moby_patternmatcher",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/patternmatcher",
        sum = "h1:YCZgJOeULcxLw1Q+sVR636pmS7sPEn1Qo2iAN6M7DBo=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_moby_spdystream",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/spdystream",
        sum = "h1:cjW1zVyyoiM0T7b6UoySUFqzXMoqRckQtXwGPiBhOM8=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_moby_sys_mount",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/sys/mount",
        sum = "h1:fX1SVkXFJ47XWDoeFW4Sq7PdQJnV2QIDZAqjNqgEjUs=",
        version = "v0.3.3",
    )
    go_repository(
        name = "com_github_moby_sys_mountinfo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/sys/mountinfo",
        sum = "h1:BzJjoreD5BMFNmD9Rus6gdd1pLuecOFPt8wC+Vygl78=",
        version = "v0.6.2",
    )
    go_repository(
        name = "com_github_moby_sys_sequential",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/sys/sequential",
        sum = "h1:OPvI35Lzn9K04PBbCLW0g4LcFAJgHsvXsRyewg5lXtc=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_moby_sys_signal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/sys/signal",
        sum = "h1:25RW3d5TnQEoKvRbEKUGay6DCQ46IxAVTT9CUMgmsSI=",
        version = "v0.7.0",
    )
    go_repository(
        name = "com_github_moby_term",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/term",
        sum = "h1:HfkjXDfhgVaN5rmueG8cL8KKeFNecRCXFhaJ2qZ5SKA=",
        version = "v0.0.0-20221205130635-1aeaba878587",
    )
    go_repository(
        name = "com_github_modern_go_concurrent",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/modern-go/concurrent",
        sum = "h1:TRLaZ9cD/w8PVh93nsPXa1VrQ6jlwL5oN8l14QlcNfg=",
        version = "v0.0.0-20180306012644-bacd9c7ef1dd",
    )
    go_repository(
        name = "com_github_modern_go_reflect2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/modern-go/reflect2",
        sum = "h1:xBagoLtFs94CBntxluKeaWgTMpvLxC4ur3nMaC9Gz0M=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_modocache_gover",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/modocache/gover",
        sum = "h1:8Q0qkMVC/MmWkpIdlvZgcv2o2jrlF6zqVOh7W5YHdMA=",
        version = "v0.0.0-20171022184752-b58185e213c5",
    )
    go_repository(
        name = "com_github_monochromegane_go_gitignore",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/monochromegane/go-gitignore",
        sum = "h1:n6/2gBQ3RWajuToeY6ZtZTIKv2v7ThUy5KKusIT0yc0=",
        version = "v0.0.0-20200626010858-205db1a8cc00",
    )
    go_repository(
        name = "com_github_montanaflynn_stats",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/montanaflynn/stats",
        sum = "h1:r3y12KyNxj/Sb/iOE46ws+3mS1+MZca1wlHQFPsY/JU=",
        version = "v0.7.0",
    )
    go_repository(
        name = "com_github_morikuni_aec",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/morikuni/aec",
        sum = "h1:nP9CBfwrvYnBRgY6qfDQkygYDmYwOilePFkwzv4dU8A=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mostynb_go_grpc_compression",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mostynb/go-grpc-compression",
        sum = "h1:KJzRFSYPXlcoYjG5/xLZB8tpuOyWF2UnlW4tAuaWnfI=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_mpvl_unique",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mpvl/unique",
        sum = "h1:D5x39vF5KCwKQaw+OC9ZPiLVHXz3UFw2+psEX+gYcto=",
        version = "v0.0.0-20150818121801-cbe035fff7de",
    )
    go_repository(
        name = "com_github_mrjones_oauth",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mrjones/oauth",
        sum = "h1:j2kD3MT1z4PXCiUllUJF9mWUESr9TWKS7iEKsQ/IipM=",
        version = "v0.0.0-20190623134757-126b35219450",
    )
    go_repository(
        name = "com_github_mroth_weightedrand_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mroth/weightedrand/v2",
        sum = "h1:zrEVDIaau/E4QLOKu02kpg8T8myweFlMGikIgbIdrRA=",
        version = "v2.0.1",
    )
    go_repository(
        name = "com_github_mschoch_smat",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mschoch/smat",
        sum = "h1:8imxQsjDm8yFEAVBe7azKmKSgzSkZXDuKkSq9374khM=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_msteinert_pam",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/msteinert/pam",
        sum = "h1:VhLun/0n0kQYxiRBJJvVpC2jR6d21SWJFjpvUVj20Kc=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_muesli_reflow",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/muesli/reflow",
        sum = "h1:IFsN6K9NfGtjeggFP+68I4chLZV2yIKsXJFNZ+eWh6s=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_muesli_termenv",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/muesli/termenv",
        sum = "h1:KuQRUE3PgxRFWhq4gHvZtPSLCGDqM5q/cYr1pZ39ytc=",
        version = "v0.12.0",
    )
    go_repository(
        name = "com_github_munnerz_goautoneg",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/munnerz/goautoneg",
        sum = "h1:C3w9PqII01/Oq1c1nUAm88MOHcQC9l5mIlSMApZMrHA=",
        version = "v0.0.0-20191010083416-a7dc8b61c822",
    )
    go_repository(
        name = "com_github_mwitkow_go_conntrack",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mwitkow/go-conntrack",
        sum = "h1:KUppIJq7/+SVif2QVs3tOP0zanoHgBEVAwHxUSIzRqU=",
        version = "v0.0.0-20190716064945-2f068394615f",
    )
    go_repository(
        name = "com_github_mwitkow_go_proto_validators",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mwitkow/go-proto-validators",
        sum = "h1:qRlmpTzm2pstMKKzTdvwPCF5QfBNURSlAgN/R+qbKos=",
        version = "v0.3.2",
    )
    go_repository(
        name = "com_github_mxk_go_flowrate",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mxk/go-flowrate",
        sum = "h1:y5//uYreIhSUg3J1GEMiLbxo1LJaP8RfCpH6pymGZus=",
        version = "v0.0.0-20140419014527-cca7078d478f",
    )
    go_repository(
        name = "com_github_ncw_swift",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ncw/swift",
        sum = "h1:4DQRPj35Y41WogBxyhOXlrI37nzGlyEcsforeudyYPQ=",
        version = "v1.0.47",
    )
    go_repository(
        name = "com_github_neelance_astrewrite",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/neelance/astrewrite",
        sum = "h1:D6paGObi5Wud7xg83MaEFyjxQB1W5bz5d0IFppr+ymk=",
        version = "v0.0.0-20160511093645-99348263ae86",
    )
    go_repository(
        name = "com_github_neelance_sourcemap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/neelance/sourcemap",
        sum = "h1:bY6ktFuJkt+ZXkX0RChQch2FtHpWQLVS8Qo1YasiIVk=",
        version = "v0.0.0-20200213170602-2833bce08e4c",
    )
    go_repository(
        name = "com_github_nfnt_resize",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nfnt/resize",
        sum = "h1:zYyBkD/k9seD2A7fsi6Oo2LfFZAehjjQMERAvZLEDnQ=",
        version = "v0.0.0-20180221191011-83c6a9932646",
    )
    go_repository(
        name = "com_github_niemeyer_pretty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/niemeyer/pretty",
        sum = "h1:fD57ERR4JtEqsWbfPhv4DMiApHyliiK5xCTNVSPiaAs=",
        version = "v0.0.0-20200227124842-a10e7caefd8e",
    )
    go_repository(
        name = "com_github_nightlyone_lockfile",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nightlyone/lockfile",
        sum = "h1:RHep2cFKK4PonZJDdEl4GmkabuhbsRMgk/k3uAmxBiA=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_niklasfasching_go_org",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/niklasfasching/go-org",
        sum = "h1:5YAIqNTdl6lAOb7lD2AyQ1RuFGPVrAKvUexphk8PGbo=",
        version = "v1.6.5",
    )
    go_repository(
        name = "com_github_nishanths_predeclared",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nishanths/predeclared",
        sum = "h1:3f0nxAmdj/VoCGN/ijdMy7bj6SBagaqYg1B0hu8clMA=",
        version = "v0.0.0-20200524104333-86fad755b4d3",
    )
    go_repository(
        name = "com_github_npillmayer_nestext",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/npillmayer/nestext",
        sum = "h1:2dkbzJ5xMcyJW5b8wwrX+nnRNvf/Nn1KwGhIauGyE2E=",
        version = "v0.1.3",
    )
    go_repository(
        name = "com_github_nu7hatch_gouuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nu7hatch/gouuid",
        sum = "h1:VhgPp6v9qf9Agr/56bj7Y/xa04UccTW04VP0Qed4vnQ=",
        version = "v0.0.0-20131221200532-179d4d0c4d8d",
    )
    go_repository(
        name = "com_github_nwaples_rardecode",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nwaples/rardecode",
        sum = "h1:cWCaZwfM5H7nAD6PyEdcVnczzV8i/JtotnyW/dD9lEc=",
        version = "v1.1.3",
    )
    go_repository(
        name = "com_github_nxadm_tail",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nxadm/tail",
        sum = "h1:nPr65rt6Y5JFSKQO7qToXr7pePgD6Gwiw05lkbyAQTE=",
        version = "v1.4.8",
    )
    go_repository(
        name = "com_github_nytimes_gziphandler",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/NYTimes/gziphandler",
        sum = "h1:ZUDjpQae29j0ryrS0u/B8HZfJBtBQHjqw2rQ2cqUQ3I=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_oklog_run",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/oklog/run",
        sum = "h1:GEenZ1cK0+q0+wsJew9qUg/DyD8k3JzYsZAi5gYi2mA=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_oklog_ulid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/oklog/ulid",
        sum = "h1:EGfNDEx6MqHz8B3uNV6QAib1UR2Lm97sHi3ocA6ESJ4=",
        version = "v1.3.1",
    )
    go_repository(
        name = "com_github_oklog_ulid_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/oklog/ulid/v2",
        sum = "h1:r4fFzBm+bv0wNKNh5eXTwU7i85y5x+uwkxCUTNVQqLc=",
        version = "v2.0.2",
    )
    go_repository(
        name = "com_github_olekukonko_tablewriter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/olekukonko/tablewriter",
        sum = "h1:P2Ga83D34wi1o9J6Wh1mRuqd4mF/x/lgBS7N7AbDhec=",
        version = "v0.0.5",
    )
    go_repository(
        name = "com_github_oliamb_cutter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/oliamb/cutter",
        sum = "h1:Lfwkya0HHNU1YLnGv2hTkzHfasrSMkgv4Dn+5rmlk3k=",
        version = "v0.2.2",
    )
    go_repository(
        name = "com_github_olivere_elastic_v7",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/olivere/elastic/v7",
        sum = "h1:R7CXvbu8Eq+WlsLgxmKVKPox0oOwAE/2T9Si5BnvK6E=",
        version = "v7.0.32",
    )
    go_repository(
        name = "com_github_oneofone_xxhash",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/OneOfOne/xxhash",
        sum = "h1:KMrpdQIwFcEqXDklaen+P1axHaj9BSKzvpUUfnHldSE=",
        version = "v1.2.2",
    )
    go_repository(
        name = "com_github_onsi_ginkgo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/onsi/ginkgo",
        sum = "h1:8xi0RTUf59SOSfEtZMvwTvXYMzG4gV23XVHOZiXNtnE=",
        version = "v1.16.5",
    )
    go_repository(
        name = "com_github_onsi_ginkgo_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/onsi/ginkgo/v2",
        sum = "h1:06xGQy5www2oN160RtEZoTvnP2sPhEfePYmCDc2szss=",
        version = "v2.9.7",
    )
    go_repository(
        name = "com_github_onsi_gomega",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/onsi/gomega",
        sum = "h1:gegWiwZjBsf2DgiSbf5hpokZ98JVDMcWkUiigk6/KXc=",
        version = "v1.27.8",
    )
    go_repository(
        name = "com_github_opencontainers_go_digest",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opencontainers/go-digest",
        sum = "h1:apOUWs51W5PlhuyGyz9FCeeBIOUDA/6nW8Oi/yOhh5U=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_opencontainers_image_spec",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opencontainers/image-spec",
        sum = "h1:fzg1mXZFj8YdPeNkRXMg+zb88BFV0Ys52cJydRwBkb8=",
        version = "v1.1.0-rc3",
    )
    go_repository(
        name = "com_github_opencontainers_runc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opencontainers/runc",
        sum = "h1:L44KXEpKmfWDcS02aeGm8QNTFXTo2D+8MYGDIJ/GDEs=",
        version = "v1.1.5",
    )
    go_repository(
        name = "com_github_opencontainers_runtime_spec",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opencontainers/runtime-spec",
        sum = "h1:3snG66yBm59tKhhSPQrQ/0bCrv1LQbKt40LnUPiUxdc=",
        version = "v1.0.3-0.20210326190908-1c3f411f0417",
    )
    go_repository(
        name = "com_github_opencontainers_selinux",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opencontainers/selinux",
        sum = "h1:NFy2xCsjn7+WspbfZkUd5zyVeisV7VFbPSP96+8/ha4=",
        version = "v1.10.2",
    )

    go_repository(
        # This is no longer used but we keep it for backwards compatability
        # tests while on 5.0.x.
        name = "com_github_opentracing_contrib_go_stdlib",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opentracing-contrib/go-stdlib",
        sum = "h1:TBS7YuVotp8myLon4Pv7BtCBzOTo1DeZCld0Z63mW2w=",
        version = "v1.0.0",
    )  # keep
    go_repository(
        name = "com_github_opentracing_opentracing_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opentracing/opentracing-go",
        sum = "h1:uEJPy/1a5RIPAJ0Ov+OIO8OxWu77jEv+1B0VhjKrZUs=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_opsgenie_opsgenie_go_sdk_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opsgenie/opsgenie-go-sdk-v2",
        sum = "h1:0h+YoXSyipf6XQGyIaDg6z5jwRik1JSm+sQetnD7vGY=",
        version = "v1.2.22",
    )
    go_repository(
        name = "com_github_oschwald_maxminddb_golang",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/oschwald/maxminddb-golang",
        sum = "h1:9FnTOD0YOhP7DGxGsq4glzpGy5+w7pq50AS6wALUMYs=",
        version = "v1.12.0",
    )
    go_repository(
        name = "com_github_ovh_go_ovh",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ovh/go-ovh",
        sum = "h1:bHXZmw8nTgZin4Nv7JuaLs0KG5x54EQR7migYTd1zrk=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_package_url_packageurl_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/package-url/packageurl-go",
        sum = "h1:DiLBVp4DAcZlBVBEtJpNWZpZVq0AEeCY7Hqk8URVs4o=",
        version = "v0.1.1-0.20220428063043-89078438f170",
    )
    go_repository(
        name = "com_github_pandatix_go_cvss",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pandatix/go-cvss",
        sum = "h1:9441i+Sn/P/TP9kNBl3kI7mwYtNYFr1eN8JdsiybiMM=",
        version = "v0.5.2",
    )
    go_repository(
        name = "com_github_pascaldekloe_goe",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pascaldekloe/goe",
        sum = "h1:cBOtyMzM9HTpWjXfbbunk26uA6nG3a8n06Wieeh0MwY=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_pborman_uuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pborman/uuid",
        sum = "h1:J7Q5mO4ysT1dv8hyrUGHb9+ooztCXu1D8MY8DZYsu3g=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_pelletier_go_toml",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pelletier/go-toml",
        sum = "h1:4yBQzkHv+7BHq2PQUZF3Mx0IYxG7LsP222s7Agd3ve8=",
        version = "v1.9.5",
    )
    go_repository(
        name = "com_github_pelletier_go_toml_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pelletier/go-toml/v2",
        sum = "h1:ipoSadvV8oGUjnUbMub59IDPPwfxF694nG/jwbMiyQg=",
        version = "v2.0.5",
    )
    go_repository(
        name = "com_github_peterbourgon_diskv",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/peterbourgon/diskv",
        sum = "h1:UBdAOUP5p4RWqPBg048CAvpKN+vxiaj6gdUUzhl4XmI=",
        version = "v2.0.1+incompatible",
    )
    go_repository(
        name = "com_github_peterbourgon_ff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/peterbourgon/ff",
        sum = "h1:xt1lxTG+Nr2+tFtysY7abFgPoH3Lug8CwYJMOmJRXhk=",
        version = "v1.7.1",
    )
    go_repository(
        name = "com_github_peterbourgon_ff_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/peterbourgon/ff/v3",
        sum = "h1:2J07/5/36kd9HYVt42Zve0xCeQ+LLRIvoKrt6sAZXJ4=",
        version = "v3.3.2",
    )
    go_repository(
        name = "com_github_peterhellberg_link",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/peterhellberg/link",
        sum = "h1:s2+RH8EGuI/mI4QwrWGSYQCRz7uNgip9BaM04HKu5kc=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_pierrec_lz4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pierrec/lz4",
        sum = "h1:2xWsjqPFWcplujydGg4WmhC/6fZqK42wMM8aXeqhl0I=",
        version = "v2.0.5+incompatible",
    )
    go_repository(
        name = "com_github_pierrec_lz4_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pierrec/lz4/v4",
        sum = "h1:kV4Ip+/hUBC+8T6+2EgburRtkE9ef4nbY3f4dFhGjMc=",
        version = "v4.1.17",
    )
    go_repository(
        name = "com_github_pingcap_errors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pingcap/errors",
        sum = "h1:lFuQV/oaUMGcD2tqt+01ROSmJs75VG1ToEOkZIZ4nE4=",
        version = "v0.11.4",
    )
    go_repository(
        name = "com_github_pjbgf_sha1cd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pjbgf/sha1cd",
        sum = "h1:4D5XXmUUBUl/xQ6IjCkEAbqXskkq/4O7LmGn0AqMDs4=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_pkg_browser",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkg/browser",
        sum = "h1:KoWmjvw+nsYOo29YJK9vDA65RGE3NrOnUtO7a+RF9HU=",
        version = "v0.0.0-20210911075715-681adbf594b8",
    )
    go_repository(
        name = "com_github_pkg_diff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkg/diff",
        sum = "h1:aoZm08cpOy4WuID//EZDgcC4zIxODThtZNPirFr42+A=",
        version = "v0.0.0-20210226163009-20ebb0f2a09e",
    )
    go_repository(
        name = "com_github_pkg_errors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkg/errors",
        sum = "h1:FEBLx1zS214owpjy7qsBeixbURkuhQAwrK5UwLGTwt4=",
        version = "v0.9.1",
    )
    go_repository(
        name = "com_github_pkg_profile",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkg/profile",
        sum = "h1:hUDfIISABYI59DyeB3OTay/HxSRwTQ8rB/H83k6r5dM=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_pkg_sftp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkg/sftp",
        sum = "h1:VasscCm72135zRysgrJDKsntdmPN+OuU3+nnHYA9wyc=",
        version = "v1.10.1",
    )
    go_repository(
        name = "com_github_pkoukk_tiktoken_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkoukk/tiktoken-go",
        sum = "h1:JF0TlJzhTbrI30wCvFuiw6FzP2+/bR+FIxUdgEAcUsw=",
        version = "v0.1.6",
    )
    go_repository(
        name = "com_github_pmezard_go_difflib",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pmezard/go-difflib",
        sum = "h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_posener_complete",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/posener/complete",
        sum = "h1:NP0eAhjcjImqslEwo/1hq7gpajME0fTLTezBKDqfXqo=",
        version = "v1.2.3",
    )
    go_repository(
        name = "com_github_power_devops_perfstat",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/power-devops/perfstat",
        sum = "h1:0LFwY6Q3gMACTjAbMZBjXAqTOzOwFaj2Ld6cjeQ7Rig=",
        version = "v0.0.0-20221212215047-62379fc7944b",
    )
    go_repository(
        name = "com_github_pquerna_cachecontrol",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pquerna/cachecontrol",
        sum = "h1:yJMy84ti9h/+OEWa752kBTKv4XC30OtVVHYv/8cTqKc=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_pquerna_otp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pquerna/otp",
        sum = "h1:oJV/SkzR33anKXwQU3Of42rL4wbrffP4uvUf1SvS5Xs=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_prashantv_gostub",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prashantv/gostub",
        sum = "h1:BTyx3RfQjRHnUWaGF9oQos79AlQ5k8WNktv7VGvVH4g=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_prometheus_alertmanager",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/alertmanager",
        replace = "github.com/sourcegraph/alertmanager",
        sum = "h1:Mlytsllyx6d1eaKXt8urXX0YjP5Tq4UGV+BfL6Yc1aQ=",
        version = "v0.21.1-0.20211110092431-863f5b1ee51b",
    )
    go_repository(
        name = "com_github_prometheus_client_golang",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/client_golang",
        sum = "h1:yk/hx9hDbrGHovbci4BY+pRMfSuuat626eFsHb7tmT8=",
        version = "v1.16.0",
    )
    go_repository(
        name = "com_github_prometheus_client_model",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/client_model",
        sum = "h1:VQw1hfvPvk3Uv6Qf29VrPF32JB6rtbgI6cYPYQjL0Qw=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_prometheus_common",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/common",
        replace = "github.com/prometheus/common",
        sum = "h1:hWIdL3N2HoUx3B8j3YN9mWor0qhY/NlEKZEaXxuIRh4=",
        version = "v0.32.1",
    )
    go_repository(
        name = "com_github_prometheus_common_assets",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/common/assets",
        sum = "h1:0P5OrzoHrYBOSM1OigWL3mY8ZvV2N4zIE/5AahrSrfM=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_prometheus_common_sigv4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/common/sigv4",
        sum = "h1:qoVebwtwwEhS85Czm2dSROY5fTo2PAPEVdDeppTwGX4=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_prometheus_exporter_toolkit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/exporter-toolkit",
        sum = "h1:sbJAfBXQFkG6sUkbwBun8MNdzW9+wd5YfPYofbmj0YM=",
        version = "v0.8.2",
    )
    go_repository(
        name = "com_github_prometheus_procfs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/procfs",
        sum = "h1:xRC8Iq1yyca5ypa9n1EZnWZkt7dwcoRPQwX/5gwaUuI=",
        version = "v0.11.1",
    )
    go_repository(
        name = "com_github_prometheus_prometheus",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/prometheus",
        sum = "h1:wmk5yNrQlkQ2OvZucMhUB4k78AVfG34szb1UtopS8Vc=",
        version = "v0.40.5",
    )
    go_repository(
        name = "com_github_prometheus_statsd_exporter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/statsd_exporter",
        sum = "h1:7Pji/i2GuhK6Lu7DHrtTkFmNBCudCPT1pX2CziuyQR0=",
        version = "v0.22.7",
    )
    go_repository(
        name = "com_github_prometheus_tsdb",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/tsdb",
        sum = "h1:YZcsG11NqnK4czYLrWd9mpEuAJIHVQLwdrleYfszMAA=",
        version = "v0.7.1",
    )
    go_repository(
        name = "com_github_protocolbuffers_txtpbfmt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/protocolbuffers/txtpbfmt",
        sum = "h1:gSVONBi2HWMFXCa9jFdYvYk7IwW/mTLxWOF7rXS4LO0=",
        version = "v0.0.0-20201118171849-f6a6b3f636fc",
    )
    go_repository(
        name = "com_github_protonmail_go_crypto",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ProtonMail/go-crypto",
        sum = "h1:vV3RryLxt42+ZIVOFbYJCH1jsZNTNmj2NYru5zfx+4E=",
        version = "v0.0.0-20230626094100-7e9e0395ebec",
    )
    go_repository(
        name = "com_github_pseudomuto_protoc_gen_doc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pseudomuto/protoc-gen-doc",
        sum = "h1:Ah259kcrio7Ix1Rhb6u8FCaOkzf9qRBqXnvAufg061w=",
        version = "v1.5.1",
    )
    go_repository(
        name = "com_github_pseudomuto_protokit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pseudomuto/protokit",
        sum = "h1:kCYpE3thoR6Esm0CUvd5xbrDTOZPvQPTDeyXpZfrJdk=",
        version = "v0.2.1",
    )
    go_repository(
        name = "com_github_puerkitobio_goquery",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/PuerkitoBio/goquery",
        sum = "h1:PJTF7AmFCFKk1N6V6jmKfrNH9tV5pNE6lZMkG0gta/U=",
        version = "v1.8.0",
    )
    go_repository(
        name = "com_github_puerkitobio_purell",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/PuerkitoBio/purell",
        sum = "h1:WEQqlqaGbrPkxLJWfBwQmfEAE1Z7ONdDLqrN38tNFfI=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_puerkitobio_rehttp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/PuerkitoBio/rehttp",
        sum = "h1:JFZ7OeK+hbJpTxhNB0NDZT47AuXqCU0Smxfjtph7/Rs=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_puerkitobio_urlesc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/PuerkitoBio/urlesc",
        sum = "h1:d+Bc7a5rLufV/sSk/8dngufqelfh6jnri85riMAaF/M=",
        version = "v0.0.0-20170810143723-de5bf2ad4578",
    )
    go_repository(
        name = "com_github_qdrant_go_client",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/qdrant/go-client",
        sum = "h1:kh5B4yWjrd5BcynJoA4014mZlI/j6++inCMMQoKUOtI=",
        version = "v1.4.1",
    )
    go_repository(
        name = "com_github_quasoft_websspi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/quasoft/websspi",
        sum = "h1:/mA4w0LxWlE3novvsoEL6BBA1WnjJATbjkh1kFrTidw=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_qustavo_sqlhooks_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/qustavo/sqlhooks/v2",
        sum = "h1:54yBemHnGHp/7xgT+pxwmIlMSDNYKx5JW5dfRAiCZi0=",
        version = "v2.1.0",
    )

    go_repository(
        # This is no longer used but we keep it for backwards compatability
        # tests while on 5.0.x.
        name = "com_github_rafaeljusto_redigomock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rafaeljusto/redigomock",
        sum = "h1:d7uo5MVINMxnRr20MxbgDkmZ8QRfevjOVgEa4n0OZyY=",
        version = "v2.4.0+incompatible",
    )  # keep
    go_repository(
        name = "com_github_rafaeljusto_redigomock_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rafaeljusto/redigomock/v3",
        sum = "h1:B4Y0XJQiPjpwYmkH55aratKX1VfR+JRqzmDKyZbC99o=",
        version = "v3.1.2",
    )
    go_repository(
        name = "com_github_rainycape_unidecode",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rainycape/unidecode",
        sum = "h1:ta7tUOvsPHVHGom5hKW5VXNc2xZIkfCKP8iaqOyYtUQ=",
        version = "v0.0.0-20150907023854-cb7f23ec59be",
    )
    go_repository(
        name = "com_github_redis_go_redis_v9",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/redis/go-redis/v9",
        sum = "h1:CuQcn5HIEeK7BgElubPP8CGtE0KakrnbBSTLjathl5o=",
        version = "v9.0.5",
    )
    go_repository(
        name = "com_github_remyoudompheng_bigfft",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/remyoudompheng/bigfft",
        sum = "h1:W09IVJc94icq4NjY3clb7Lk8O1qJ8BdBEF8z0ibU0rE=",
        version = "v0.0.0-20230129092748-24d4a6f8daec",
    )
    go_repository(
        name = "com_github_rhnvrm_simples3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rhnvrm/simples3",
        sum = "h1:H0DJwybR6ryQE+Odi9eqkHuzjYAeJgtGcGtuBwOhsH8=",
        version = "v0.6.1",
    )
    go_repository(
        name = "com_github_rickb777_date",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rickb777/date",
        sum = "h1:FS47C+yEMgIPE2ZUA67ekpU1CZXMSliHkeB/V+ZFUg4=",
        version = "v1.14.3",
    )
    go_repository(
        name = "com_github_rickb777_plural",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rickb777/plural",
        sum = "h1:4CU5NiUqXSM++2+7JCrX+oguXd2D7RY5O1YisMw1yCI=",
        version = "v1.2.2",
    )
    go_repository(
        name = "com_github_rivo_uniseg",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rivo/uniseg",
        sum = "h1:utMvzDsuh3suAEnhH0RdHmoPbU648o6CvXxTx4SBMOw=",
        version = "v0.4.3",
    )
    go_repository(
        name = "com_github_rjeczalik_notify",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rjeczalik/notify",
        sum = "h1:6rJAzHTGKXGj76sbRgDiDcYj/HniypXmSJo1SWakZeY=",
        version = "v0.9.3",
    )
    go_repository(
        name = "com_github_roaringbitmap_roaring",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/RoaringBitmap/roaring",
        sum = "h1:aQmu9zQxDU0uhwR8SXOH/OrqEf+X8A0LQmwW3JX8Lcg=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_rogpeppe_fastuuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rogpeppe/fastuuid",
        sum = "h1:Ppwyp6VYCF1nvBTXL3trRso7mXMlRrw9ooo375wvi2s=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_rogpeppe_go_internal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rogpeppe/go-internal",
        sum = "h1:cWPaGQEPrBb5/AsnsZesgZZ9yb1OQ+GOISoDNXVBh4M=",
        version = "v1.11.0",
    )
    go_repository(
        name = "com_github_rs_cors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rs/cors",
        sum = "h1:l9HGsTsHJcvW14Nk7J9KFz8bzeAWXn3CG6bgt7LsrAE=",
        version = "v1.9.0",
    )
    go_repository(
        name = "com_github_rs_xid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rs/xid",
        sum = "h1:mKX4bl4iPYJtEIxp6CYiUuLQ/8DYMoz0PUdtGgMFRVc=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_russellhaering_gosaml2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/russellhaering/gosaml2",
        sum = "h1:H/whrl8NuSoxyW46Ww5lKPskm+5K+qYLw9afqJ/Zef0=",
        version = "v0.9.1",
    )
    go_repository(
        name = "com_github_russellhaering_goxmldsig",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/russellhaering/goxmldsig",
        sum = "h1:DllIWUgMy0cRUMfGiASiYEa35nsieyD3cigIwLonTPM=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_russross_blackfriday",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/russross/blackfriday",
        replace = "github.com/russross/blackfriday",
        sum = "h1:HyvC0ARfnZBqnXwABFeSZHpKvJHJJfPz81GNueLj0oo=",
        version = "v1.5.2",
    )
    go_repository(
        name = "com_github_russross_blackfriday_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/russross/blackfriday/v2",
        sum = "h1:JIOH55/0cWyOuilr9/qlrm0BSXldqnqwMsf35Ld67mk=",
        version = "v2.1.0",
    )
    go_repository(
        name = "com_github_ryanuber_columnize",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ryanuber/columnize",
        sum = "h1:j1Wcmh8OrK4Q7GXY+V7SVSY8nUWQxHW5TkBe7YUl+2s=",
        version = "v2.1.0+incompatible",
    )
    go_repository(
        name = "com_github_ryanuber_go_glob",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ryanuber/go-glob",
        sum = "h1:iQh3xXAumdQ+4Ufa5b25cRpC5TYKlno6hsv6Cb3pkBk=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_sabhiram_go_gitignore",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sabhiram/go-gitignore",
        sum = "h1:OkMGxebDjyw0ULyrTYWeN0UNCCkmCWfjPnIA2W6oviI=",
        version = "v0.0.0-20210923224102-525f6e181f06",
    )
    go_repository(
        name = "com_github_santhosh_tekuri_jsonschema_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/santhosh-tekuri/jsonschema/v5",
        sum = "h1:HNLA3HtUIROrQwG1cuu5EYuqk3UEoJ61Dr/9xkd6sok=",
        version = "v5.0.1",
    )
    go_repository(
        name = "com_github_scaleway_scaleway_sdk_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/scaleway/scaleway-sdk-go",
        sum = "h1:0roa6gXKgyta64uqh52AQG3wzZXH21unn+ltzQSXML0=",
        version = "v1.0.0-beta.9",
    )
    go_repository(
        name = "com_github_schollz_closestmatch",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/schollz/closestmatch",
        sum = "h1:Uel2GXEpJqOWBrlyI+oY9LTiyyjYS17cCYRqP13/SHk=",
        version = "v2.1.0+incompatible",
    )
    go_repository(
        name = "com_github_schollz_progressbar_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/schollz/progressbar/v3",
        sum = "h1:o8rySDYiQ59Mwzy2FELeHY5ZARXZTVJC7iHD6PEFUiE=",
        version = "v3.13.1",
    )
    go_repository(
        name = "com_github_scim2_filter_parser_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/scim2/filter-parser/v2",
        sum = "h1:QGadEcsmypxg8gYChRSM2j1edLyE/2j72j+hdmI4BJM=",
        version = "v2.2.0",
    )
    go_repository(
        name = "com_github_sean_seed",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sean-/seed",
        sum = "h1:nn5Wsu0esKSJiIVhscUtVbo7ada43DJhG55ua/hjS5I=",
        version = "v0.0.0-20170313163322-e2103e2c3529",
    )
    go_repository(
        name = "com_github_secure_systems_lab_go_securesystemslib",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/secure-systems-lab/go-securesystemslib",
        sum = "h1:b23VGrQhTA8cN2CbBw7/FulN9fTtqYUdS5+Oxzt+DUE=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_segmentio_fasthash",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/segmentio/fasthash",
        sum = "h1:EI9+KE1EwvMLBWwjpRDc+fEM+prwxDYbslddQGtrmhM=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_segmentio_ksuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/segmentio/ksuid",
        sum = "h1:sBo2BdShXjmcugAMwjugoGUdUV0pcxY5mW4xKRn3v4c=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_sergi_go_diff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sergi/go-diff",
        sum = "h1:xkr+Oxo4BOQKmkn/B9eMK0g5Kg/983T9DqqPHwYqD+8=",
        version = "v1.3.1",
    )
    go_repository(
        name = "com_github_serialx_hashring",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/serialx/hashring",
        sum = "h1:ka9QPuQg2u4LGipiZGsgkg3rJCo4iIUCy75FddM0GRQ=",
        version = "v0.0.0-20190422032157-8b2912629002",
    )
    go_repository(
        name = "com_github_shibumi_go_pathspec",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shibumi/go-pathspec",
        sum = "h1:QUyMZhFo0Md5B8zV8x2tesohbb5kfbpTi9rBnKh5dkI=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_shirou_gopsutil_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shirou/gopsutil/v3",
        sum = "h1:5SgDCeQ0KW0S4N0znjeM/eFHXXOKyv2dVNgRq/c9P6Y=",
        version = "v3.23.5",
    )
    go_repository(
        name = "com_github_shoenig_go_m1cpu",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shoenig/go-m1cpu",
        sum = "h1:nxdKQNcEB6vzgA2E2bvzKIYRuNj7XNJ4S/aRSwKzFtM=",
        version = "v0.1.6",
    )
    go_repository(
        name = "com_github_shoenig_test",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shoenig/test",
        sum = "h1:kVTaSd7WLz5WZ2IaoM0RSzRsUD+m8wRR+5qvntpn4LU=",
        version = "v0.6.4",
    )
    go_repository(
        name = "com_github_shopify_goreferrer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Shopify/goreferrer",
        sum = "h1:KkH3I3sJuOLP3TjA/dfr4NAY8bghDwnXiU7cTKxQqo0=",
        version = "v0.0.0-20220729165902-8cddb4f5de06",
    )
    go_repository(
        name = "com_github_shopify_logrus_bugsnag",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Shopify/logrus-bugsnag",
        sum = "h1:UrqY+r/OJnIp5u0s1SbQ8dVfLCZJsnvazdBP5hS4iRs=",
        version = "v0.0.0-20171204204709-577dee27f20d",
    )
    go_repository(
        name = "com_github_shopspring_decimal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shopspring/decimal",
        sum = "h1:abSATXmQEYyShuxI4/vyW3tV1MrKAJzCZ/0zLUXYbsQ=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_shurcool_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shurcooL/go",
        sum = "h1:aSISeOcal5irEhJd1M+IrApc0PdcN7e7Aj4yuEnOrfQ=",
        version = "v0.0.0-20200502201357-93f07166e636",
    )
    go_repository(
        name = "com_github_shurcool_go_goon",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shurcooL/go-goon",
        sum = "h1:llrF3Fs4018ePo4+G/HV/uQUqEI1HMDjCeOf2V6puPc=",
        version = "v0.0.0-20170922171312-37c2f522c041",
    )
    go_repository(
        name = "com_github_shurcool_httpfs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shurcooL/httpfs",
        sum = "h1:bUGsEnyNbVPw06Bs80sCeARAlK8lhwqGyi6UT8ymuGk=",
        version = "v0.0.0-20190707220628-8d4bc4ba7749",
    )
    go_repository(
        name = "com_github_shurcool_httpgzip",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shurcooL/httpgzip",
        replace = "github.com/sourcegraph/httpgzip",
        sum = "h1:VaS8k40GiNVUxVx0ZUilU38NU6tWUHNQOX342DWtZUM=",
        version = "v0.0.0-20211015085752-0bad89b3b4df",
    )
    go_repository(
        name = "com_github_shurcool_sanitized_anchor_name",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shurcooL/sanitized_anchor_name",
        sum = "h1:PdmoCO6wvbs+7yrJyMORt4/BmY5IYyJwS/kOiWx8mHo=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_shurcool_vfsgen",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shurcooL/vfsgen",
        sum = "h1:pXY9qYc/MP5zdvqWEUH6SjNiu7VhSjuVFTFiTcphaLU=",
        version = "v0.0.0-20200824052919-0d455de96546",
    )
    go_repository(
        name = "com_github_sirupsen_logrus",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sirupsen/logrus",
        sum = "h1:dueUQJ1C2q9oE3F7wvmSGAaVtTmUizReu6fjN8uqzbQ=",
        version = "v1.9.3",
    )
    go_repository(
        name = "com_github_skeema_knownhosts",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/skeema/knownhosts",
        sum = "h1:MTk78x9FPgDFVFkDLTrsnnfCJl7g1C/nnKvePgrIngE=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_slack_go_slack",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/slack-go/slack",
        sum = "h1:BGbxa0kMsGEvLOEoZmYs8T1wWfoZXwmQFBb6FgYCXUA=",
        version = "v0.10.1",
    )
    go_repository(
        name = "com_github_smacker_go_tree_sitter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/smacker/go-tree-sitter",
        sum = "h1:WrsSqod9T70HFyq8hjL6wambOKb4ISUXzFUuNTJHDwo=",
        version = "v0.0.0-20220209044044-0d3022e933c3",
    )
    go_repository(
        name = "com_github_smartystreets_assertions",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/smartystreets/assertions",
        sum = "h1:Dx1kYM01xsSqKPno3aqLnrwac2LetPvN23diwyr69Qs=",
        version = "v1.13.0",
    )
    go_repository(
        name = "com_github_smartystreets_goconvey",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/smartystreets/goconvey",
        sum = "h1:fv0U8FUIMPNf1L9lnHLvLhgicrIVChEkdzIKYqbNC9s=",
        version = "v1.6.4",
    )
    go_repository(
        name = "com_github_soheilhy_cmux",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/soheilhy/cmux",
        sum = "h1:jjzc5WVemNEDTLwv9tlmemhC73tI08BNOIGwBOo10Js=",
        version = "v0.1.5",
    )
    go_repository(
        name = "com_github_sourcegraph_conc",
        build_directives = [
            "gazelle:resolve go github.com/sourcegraph/sourcegraph/lib/errors @//lib/errors",
        ],
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/conc",
        sum = "h1:0FsjN+u9fNXTS2C1JnJ6G/gkgcUg8dscyyfu3PwHWUQ=",
        version = "v0.3.1-0.20240108182409-4afefce20f9b",
    )
    go_repository(
        name = "com_github_sourcegraph_go_ctags",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/go-ctags",
        sum = "h1:7MrEECTEf+UmMdllIIbAHng17Uwqm8WbHEUAyv9LMBk=",
        version = "v0.0.0-20231024141911-299d0263dc95",
    )
    go_repository(
        name = "com_github_sourcegraph_go_diff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/go-diff",
        sum = "h1:11miag7hlORpW7ici5mL7T9PyiEsmVmf+8PFOvJ/ZrA=",
        version = "v0.6.2-0.20221123165719-f8cd299c40f3",
    )
    go_repository(
        name = "com_github_sourcegraph_go_jsonschema",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/go-jsonschema",
        sum = "h1:Bq9XPdAOBkA9NWeNEh2VeIlGlizg9rL5VkYFZje2S+4=",
        version = "v0.0.0-20221230021921-34aaf28fc4ac",
    )
    go_repository(
        name = "com_github_sourcegraph_go_langserver",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/go-langserver",
        sum = "h1:EX1bSaIbVia2U/Pt/2Z62y8wJS4Z4iNiK5VCeUKuBx8=",
        version = "v2.0.1-0.20181108233942-4a51fa2e1238+incompatible",
    )
    go_repository(
        name = "com_github_sourcegraph_go_lsp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/go-lsp",
        sum = "h1:afLbh+ltiygTOB37ymZVwKlJwWZn+86syPTbrrOAydY=",
        version = "v0.0.0-20200429204803-219e11d77f5d",
    )
    go_repository(
        name = "com_github_sourcegraph_go_rendezvous",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/go-rendezvous",
        sum = "h1:uBLhh66Nf4BcRnvCkMVEuYZ/bQ9ok0rOlEJhfVUpJj4=",
        version = "v0.0.0-20210910070954-ef39ade5591d",
    )
    go_repository(
        name = "com_github_sourcegraph_jsonx",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/jsonx",
        sum = "h1:oAdWFqhStsWiiMP/vkkHiMXqFXzl1XfUNOdxKJbd6bI=",
        version = "v0.0.0-20200629203448-1a936bd500cf",
    )
    go_repository(
        name = "com_github_sourcegraph_log",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/log",
        sum = "h1:tHKdC+bXxxGJ0cy/R06kg6Z0zqwVGOWMx8uWsIwsaoY=",
        version = "v0.0.0-20231018134238-fbadff7458bb",
    )
    go_repository(
        name = "com_github_sourcegraph_managed_services_platform_cdktf_gen_cloudflare",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/managed-services-platform-cdktf/gen/cloudflare",
        sum = "h1:0bXluGjV4O3XBeFA3Kck9kHS3ilvgJo8mW9ADx8oeHE=",
        version = "v0.0.0-20230822024612-edb48c530722",
    )
    go_repository(
        name = "com_github_sourcegraph_managed_services_platform_cdktf_gen_google",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/managed-services-platform-cdktf/gen/google",
        sum = "h1:5sY4zK5cz127gqcUQyLHQUc6oAijf9cWLNyBATntBSg=",
        version = "v0.0.0-20231222002716-51aa712b9bcc",
    )
    go_repository(
        name = "com_github_sourcegraph_managed_services_platform_cdktf_gen_google_beta",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/managed-services-platform-cdktf/gen/google_beta",
        sum = "h1:BMYXT/otUbE7daoJLSvTHo6/WwEclPzpoH8m/8+M0Ws=",
        version = "v0.0.0-20231222002716-51aa712b9bcc",
    )
    go_repository(
        name = "com_github_sourcegraph_managed_services_platform_cdktf_gen_opsgenie",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/managed-services-platform-cdktf/gen/opsgenie",
        sum = "h1:h0n42wBNI4h28Bb42A7TJbSLwmcpIvYI9NZrprPUt/A=",
        version = "v0.0.0-20231230001101-a13188f9c749",
    )
    go_repository(
        name = "com_github_sourcegraph_managed_services_platform_cdktf_gen_postgresql",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/managed-services-platform-cdktf/gen/postgresql",
        sum = "h1:YinyQmBBDqLW67QIqhvDpz8XcFSNPPWJugRuWrHDhbI=",
        version = "v0.0.0-20231220215815-b87ebb3e8c47",
    )
    go_repository(
        name = "com_github_sourcegraph_managed_services_platform_cdktf_gen_random",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/managed-services-platform-cdktf/gen/random",
        sum = "h1:N0OxHqeujHxvVU666KQY9whauLyw4s3BJGBLxx6gKR0=",
        version = "v0.0.0-20230822024612-edb48c530722",
    )
    go_repository(
        name = "com_github_sourcegraph_managed_services_platform_cdktf_gen_slack",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/managed-services-platform-cdktf/gen/slack",
        sum = "h1:Iip3TVeiTF8HyeRsK0MKMZ1vQ87fqFqwz0cVq1dib6E=",
        version = "v0.0.0-20240103014439-025608ddf849",
    )
    go_repository(
        name = "com_github_sourcegraph_managed_services_platform_cdktf_gen_tfe",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/managed-services-platform-cdktf/gen/tfe",
        sum = "h1:sCs3+EDrM4pp7PI0/efZPZZK+u955+gwr0byWGg+AVg=",
        version = "v0.0.0-20231218231056-4749baca142f",
    )
    go_repository(
        name = "com_github_sourcegraph_mountinfo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/mountinfo",
        sum = "h1:2lUb58rz1bq77wL3hb6OBT58uBVtlNs1o23Kahfj/kU=",
        version = "v0.0.0-20231018142932-e00da332dac5",
    )
    go_repository(
        name = "com_github_sourcegraph_run",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/run",
        sum = "h1:3A8w5e8HIYPfafHekvmdmmh42RHKGVhmiTZAPJclg7I=",
        version = "v0.12.0",
    )
    go_repository(
        name = "com_github_sourcegraph_scip",
        # This fixes the build for sourcegraph/scip which depends on sourcegraph/sourcegraph/lib but
        # gazelle doesn't know how to resolve those packages from within sourcegraph/scip.
        build_directives = [
            "gazelle:resolve go github.com/sourcegraph/sourcegraph/lib/errors @//lib/errors",
            "gazelle:resolve go github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol @//lib/codeintel/lsif/protocol",
            "gazelle:resolve go github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/reader @//lib/codeintel/lsif/protocol/reader",
            "gazelle:resolve go github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/writer @//lib/codeintel/lsif/protocol/writer",
        ],
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/scip",
        sum = "h1:o+eq0cjVV3B5ngIBF04Lv3GwttKOuYFF5NTcfXWXzfA=",
        version = "v0.3.1-0.20230627154934-45df7f6d33fc",
    )
    go_repository(
        name = "com_github_sourcegraph_zoekt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sourcegraph/zoekt",
        sum = "h1:TB4KxGtcJm98TfyDHhSkbffngM+EVlTu0epG3SxUU0U=",
        version = "v0.0.0-20240110094557-7487a0d53131",
    )
    go_repository(
        name = "com_github_spaolacci_murmur3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spaolacci/murmur3",
        sum = "h1:qLC7fQah7D6K1B0ujays3HV9gkFtllcxhzImRR7ArPQ=",
        version = "v0.0.0-20180118202830-f09979ecbc72",
    )
    go_repository(
        name = "com_github_spdx_tools_golang",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spdx/tools-golang",
        sum = "h1:9B623Cfs+mclYK6dsae7gLSwuIBHvlgmEup87qpqsAQ=",
        version = "v0.3.1-0.20230104082527-d6f58551be3f",
    )
    go_repository(
        name = "com_github_spf13_afero",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/afero",
        sum = "h1:xehSyVa0YnHWsJ49JFljMpg1HX19V6NDZ1fkm1Xznbo=",
        version = "v1.8.2",
    )
    go_repository(
        name = "com_github_spf13_cast",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/cast",
        sum = "h1:rj3WzYc11XZaIZMPKmwP96zkFEnnAmV8s6XbB2aY32w=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_spf13_cobra",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/cobra",
        sum = "h1:hyqWnYt1ZQShIddO5kBpj3vu05/++x6tJ6dg8EC572I=",
        version = "v1.7.0",
    )
    go_repository(
        name = "com_github_spf13_jwalterweatherman",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/jwalterweatherman",
        sum = "h1:ue6voC5bR5F8YxI5S67j9i582FU4Qvo2bmqnqMYADFk=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_spf13_pflag",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/pflag",
        sum = "h1:iy+VFUOCP1a+8yFto/drg2CJ5u0yRoB7fZw3DKv/JXA=",
        version = "v1.0.5",
    )
    go_repository(
        name = "com_github_spf13_viper",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/viper",
        sum = "h1:CZ7eSOd3kZoaYDLbXnmzgQI5RlciuXBMA+18HwHRfZQ=",
        version = "v1.12.0",
    )
    go_repository(
        name = "com_github_ssor_bom",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ssor/bom",
        sum = "h1:pvbZ0lM0XWPBqUKqFU8cmavspvIl9nulOYwdy6IFRRo=",
        version = "v0.0.0-20170718123548-6386211fdfcf",
    )
    go_repository(
        name = "com_github_stoewer_go_strcase",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/stoewer/go-strcase",
        sum = "h1:g0eASXYtp+yvN9fK8sH94oCIk0fau9uV1/ZdJ0AVEzs=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_stretchr_objx",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/stretchr/objx",
        sum = "h1:1zr/of2m5FGMsad5YfcqgdqdWrIhu+EBEJRhR1U7z/c=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_stretchr_testify",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/stretchr/testify",
        sum = "h1:CcVxjf3Q8PM0mHUKJCdn+eZZtm5yQwehR5yeSVQQcUk=",
        version = "v1.8.4",
    )
    go_repository(
        name = "com_github_stvp_go_udp_testing",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/stvp/go-udp-testing",
        sum = "h1:LUsDduamlucuNnWcaTbXQ6aLILFcLXADpOzeEH3U+OI=",
        version = "v0.0.0-20201019212854-469649b16807",
    )
    go_repository(
        name = "com_github_stvp_tempredis",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/stvp/tempredis",
        sum = "h1:QVqDTf3h2WHt08YuiTGPZLls0Wq99X9bWd0Q5ZSBesM=",
        version = "v0.0.0-20181119212430-b82af8480203",
    )
    go_repository(
        name = "com_github_subosito_gotenv",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/subosito/gotenv",
        sum = "h1:mjC+YW8QpAdXibNi+vNWgzmgBH4+5l5dCXv8cNysBLI=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_syndtr_goleveldb",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/syndtr/goleveldb",
        sum = "h1:fBdIW9lB4Iz0n9khmH8w27SJ3QEJ7+IgjPEwGSZiFdE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_tadvi_systray",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tadvi/systray",
        sum = "h1:6yITBqGTE2lEeTPG04SN9W+iWHCRyHqlVYILiSXziwk=",
        version = "v0.0.0-20190226123456-11a2b8fa57af",
    )
    go_repository(
        name = "com_github_tdewolff_minify_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tdewolff/minify/v2",
        sum = "h1:kejsHQMM17n6/gwdw53qsi6lg0TGddZADVyQOz1KMdE=",
        version = "v2.12.4",
    )
    go_repository(
        name = "com_github_tdewolff_parse_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tdewolff/parse/v2",
        sum = "h1:KCkDvNUMof10e3QExio9OPZJT8SbdKojLBumw8YZycQ=",
        version = "v2.6.4",
    )
    go_repository(
        name = "com_github_temoto_robotstxt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/temoto/robotstxt",
        sum = "h1:W2pOjSJ6SWvldyEuiFXNxz3xZ8aiWX5LbfDiOFd7Fxg=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_throttled_throttled_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/throttled/throttled/v2",
        sum = "h1:IezKE1uHlYC/0Al05oZV6Ar+uN/znw3cy9J8banxhEY=",
        version = "v2.12.0",
    )
    go_repository(
        name = "com_github_tidwall_gjson",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tidwall/gjson",
        sum = "h1:6aeJ0bzojgWLa82gDQHcx3S0Lr/O51I9bJ5nv6JFx5w=",
        version = "v1.14.0",
    )
    go_repository(
        name = "com_github_tidwall_match",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tidwall/match",
        sum = "h1:+Ho715JplO36QYgwN9PGYNhgZvoUSc9X2c80KVTi+GA=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_tidwall_pretty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tidwall/pretty",
        sum = "h1:RWIZEg2iJ8/g6fDDYzMpobmaoGh5OLl4AXtGUGPcqCs=",
        version = "v1.2.0",
    )

    go_repository(
        # This is no longer used but we keep it for backwards compatability
        # tests while on 5.2.x.
        name = "com_github_tj_assert",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tj/assert",
        sum = "h1:NSWpaDaurcAJY7PkL8Xt0PhZE7qpvbZl5ljd8r6U0bI=",
        version = "v0.0.0-20190920132354-ee03d75cd160",
    )  # keep
    go_repository(
        name = "com_github_tj_go_naturaldate",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tj/go-naturaldate",
        sum = "h1:OgJIPkR/Jk4bFMBLbxZ8w+QUxwjqSvzd9x+yXocY4RI=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_tklauser_go_sysconf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tklauser/go-sysconf",
        sum = "h1:89WgdJhk5SNwJfu+GKyYveZ4IaJ7xAkecBo+KdJV0CM=",
        version = "v0.3.11",
    )
    go_repository(
        name = "com_github_tklauser_numcpus",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tklauser/numcpus",
        sum = "h1:kebhY2Qt+3U6RNK7UqpYNA+tJ23IBEGKkB7JQBfDYms=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_tmc_grpc_websocket_proxy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tmc/grpc-websocket-proxy",
        sum = "h1:uruHq4dN7GR16kFc5fp3d1RIYzJW5onx8Ybykw2YQFA=",
        version = "v0.0.0-20201229170055-e5319fda7802",
    )
    go_repository(
        name = "com_github_tomnomnom_linkheader",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tomnomnom/linkheader",
        sum = "h1:nrZ3ySNYwJbSpD6ce9duiP+QkD3JuLCcWkdaehUS/3Y=",
        version = "v0.0.0-20180905144013-02ca5825eb80",
    )
    go_repository(
        name = "com_github_tonistiigi_fsutil",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tonistiigi/fsutil",
        sum = "h1:XOFp/3aBXlqmOFAg3r6e0qQjPnK5I970LilqX+Is1W8=",
        version = "v0.0.0-20230105215944-fb433841cbfa",
    )
    go_repository(
        name = "com_github_tonistiigi_go_actions_cache",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tonistiigi/go-actions-cache",
        sum = "h1:8eY6m1mjgyB8XySUR7WvebTM8D/Vs86jLJzD/Tw7zkc=",
        version = "v0.0.0-20220404170428-0bdeb6e1eac7",
    )
    go_repository(
        name = "com_github_tonistiigi_go_archvariant",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tonistiigi/go-archvariant",
        sum = "h1:5LC1eDWiBNflnTF1prCiX09yfNHIxDC/aukdhCdTyb0=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_tonistiigi_units",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tonistiigi/units",
        sum = "h1:SXhTLE6pb6eld/v/cCndK0AMpt1wiVFb/YYmqB3/QG0=",
        version = "v0.0.0-20180711220420-6950e57a87ea",
    )
    go_repository(
        name = "com_github_tonistiigi_vt100",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tonistiigi/vt100",
        sum = "h1:DLpt6B5oaaS8jyXHa9VA4rrZloBVPVXeCtrOsrFauxc=",
        version = "v0.0.0-20210615222946-8066bb97264f",
    )
    go_repository(
        name = "com_github_toqueteos_webbrowser",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/toqueteos/webbrowser",
        sum = "h1:tVP/gpK69Fx+qMJKsLE7TD8LuGWPnEV71wBN9rrstGQ=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_tstranex_u2f",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tstranex/u2f",
        sum = "h1:HhJkSzDDlVSVIVt7pDJwCHQj67k7A5EeBgPmeD+pVsQ=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_uber_gonduit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/uber/gonduit",
        sum = "h1:rP4TE8ZWChXDIkExuPMvB1TLWZWBrBQfY5qhIvJwKhk=",
        version = "v0.13.0",
    )
    go_repository(
        name = "com_github_uber_jaeger_client_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/uber/jaeger-client-go",
        sum = "h1:D6wyKGCecFaSRUpo8lCVbaOOb6ThwMmTEbhRwtKR97o=",
        version = "v2.30.0+incompatible",
    )
    go_repository(
        name = "com_github_uber_jaeger_lib",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/uber/jaeger-lib",
        sum = "h1:td4jdvLcExb4cBISKIpHuGoVXh+dVKhn2Um6rjCsSsg=",
        version = "v2.4.1+incompatible",
    )
    go_repository(
        name = "com_github_ugorji_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ugorji/go",
        sum = "h1:/68gy2h+1mWMrwZFeD1kQialdSzAb432dtpeJ42ovdo=",
        version = "v1.1.7",
    )
    go_repository(
        name = "com_github_ugorji_go_codec",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ugorji/go/codec",
        sum = "h1:YPXUKf7fYbp/y8xloBqZOw2qaVggbfwMlI8WM3wZUJ0=",
        version = "v1.2.7",
    )
    go_repository(
        name = "com_github_ulikunitz_xz",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ulikunitz/xz",
        sum = "h1:t92gobL9l3HE202wg3rlk19F6X+JOxl9BBrCCMYEYd8=",
        version = "v0.5.10",
    )
    go_repository(
        name = "com_github_unknwon_com",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/unknwon/com",
        sum = "h1:3d1LTxD+Lnf3soQiD4Cp/0BRB+Rsa/+RTvz8GMMzIXs=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_unrolled_render",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/unrolled/render",
        sum = "h1:uNTHMvVoI9pyyXfgoDHHycIqFONNY2p4eQR9ty+NsxM=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_urfave_cli",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/urfave/cli",
        sum = "h1:igJgVw1JdKH+trcLWLeLwZjU9fEfPesQ+9/e4MQ44S8=",
        version = "v1.22.12",
    )
    go_repository(
        name = "com_github_urfave_cli_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/urfave/cli/v2",
        sum = "h1:VAzn5oq403l5pHjc4OhD54+XGO9cdKVL/7lDjF+iKUs=",
        version = "v2.25.7",
    )
    go_repository(
        name = "com_github_urfave_negroni",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/urfave/negroni",
        sum = "h1:kIimOitoypq34K7TG7DUaJ9kq/N4Ofuwi1sjz0KipXc=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_valyala_bytebufferpool",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/valyala/bytebufferpool",
        sum = "h1:GqA5TC/0021Y/b9FG4Oi9Mr3q7XYx6KllzawFIhcdPw=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_valyala_fasthttp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/valyala/fasthttp",
        sum = "h1:CRq/00MfruPGFLTQKY8b+8SfdK60TxNztjRMnH0t1Yc=",
        version = "v1.40.0",
    )
    go_repository(
        name = "com_github_valyala_fastjson",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/valyala/fastjson",
        sum = "h1:tAKFnnwmeMGPbwJ7IwxcTPCNr3uIzoIj3/Fh90ra4xc=",
        version = "v1.6.3",
    )
    go_repository(
        name = "com_github_valyala_fasttemplate",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/valyala/fasttemplate",
        sum = "h1:lxLXG0uE3Qnshl9QyaK6XJxMXlQZELvChBOCmQD0Loo=",
        version = "v1.2.2",
    )
    go_repository(
        name = "com_github_vbatts_tar_split",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vbatts/tar-split",
        sum = "h1:hLFqsOLQ1SsppQNTMpkpPXClLDfC2A3Zgy9OUU+RVck=",
        version = "v0.11.3",
    )
    go_repository(
        name = "com_github_vektah_gqlparser",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vektah/gqlparser",
        sum = "h1:ZsyLGn7/7jDNI+y4SEhI4yAxRChlv15pUHMjijT+e68=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_vektah_gqlparser_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vektah/gqlparser/v2",
        sum = "h1:C02NsyEsL4TXJB7ndonqTfuQOL4XPIu0aAWugdmTgmc=",
        version = "v2.4.5",
    )
    go_repository(
        name = "com_github_vmihailenco_msgpack_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vmihailenco/msgpack/v5",
        sum = "h1:5gO0H1iULLWGhs2H5tbAHIZTV8/cYafcFOr9znI5mJU=",
        version = "v5.3.5",
    )
    go_repository(
        name = "com_github_vmihailenco_tagparser_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vmihailenco/tagparser/v2",
        sum = "h1:y09buUbR+b5aycVFQs/g70pqKVZNBmxwAhO7/IwNM9g=",
        version = "v2.0.0",
    )
    go_repository(
        name = "com_github_vultr_govultr_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vultr/govultr/v2",
        sum = "h1:gej/rwr91Puc/tgh+j33p/BLR16UrIPnSr+AIwYWZQs=",
        version = "v2.17.2",
    )
    go_repository(
        name = "com_github_vvakame_gcplogurl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vvakame/gcplogurl",
        sum = "h1:dH55ru2OQOIAKjZi5wwXjNnSfN0oXLFYkMQy908s+tU=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_wk8_go_ordered_map_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/wk8/go-ordered-map/v2",
        sum = "h1:5h/BUHu93oj4gIdvHHHGsScSTMijfx5PeYkE/fJgbpc=",
        version = "v2.1.8",
    )
    go_repository(
        name = "com_github_wsxiaoys_terminal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/wsxiaoys/terminal",
        sum = "h1:3UeQBvD0TFrlVjOeLOBz+CPAI8dnbqNSVwUwRrkp7vQ=",
        version = "v0.0.0-20160513160801-0940f3fc43a0",
    )
    go_repository(
        name = "com_github_x448_float16",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/x448/float16",
        sum = "h1:qLwI1I70+NjRFUR3zs1JPUCgaCXSh3SW62uAKT1mSBM=",
        version = "v0.8.4",
    )
    go_repository(
        name = "com_github_xanzy_go_gitlab",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xanzy/go-gitlab",
        sum = "h1:jR8V9cK9jXRQDb46KOB20NCF3ksY09luaG0IfXE6p7w=",
        version = "v0.86.0",
    )
    go_repository(
        name = "com_github_xanzy_ssh_agent",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xanzy/ssh-agent",
        sum = "h1:+/15pJfg/RsTxqYcX6fHqOXZwwMP+2VyYWJeWM2qQFM=",
        version = "v0.3.3",
    )
    go_repository(
        name = "com_github_xdg_go_pbkdf2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xdg-go/pbkdf2",
        sum = "h1:Su7DPu48wXMwC3bs7MCNG+z4FhcyEuz5dlvchbq0B0c=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_xdg_go_scram",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xdg-go/scram",
        sum = "h1:VOMT+81stJgXW3CpHyqHN3AXDYIMsx56mEFrB37Mb/E=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_xdg_go_stringprep",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xdg-go/stringprep",
        sum = "h1:kdwGpVNwPFtjs98xCGkHjQtGKh86rDcRZN17QEMCOIs=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_xdg_scram",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xdg/scram",
        sum = "h1:u40Z8hqBAAQyv+vATcGgV0YCnDjqSL7/q/JyPhhJSPk=",
        version = "v0.0.0-20180814205039-7eeb5667e42c",
    )
    go_repository(
        name = "com_github_xdg_stringprep",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xdg/stringprep",
        sum = "h1:n+nNi93yXLkJvKwXNP9d55HC7lGK4H/SRcwB5IaUZLo=",
        version = "v0.0.0-20180714160509-73f8eece6fdc",
    )
    go_repository(
        name = "com_github_xeipuuv_gojsonpointer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xeipuuv/gojsonpointer",
        sum = "h1:zGWFAtiMcyryUHoUjUJX0/lt1H2+i2Ka2n+D3DImSNo=",
        version = "v0.0.0-20190905194746-02993c407bfb",
    )
    go_repository(
        name = "com_github_xeipuuv_gojsonreference",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xeipuuv/gojsonreference",
        sum = "h1:EzJWgHovont7NscjpAxXsDA8S8BMYve8Y5+7cuRE7R0=",
        version = "v0.0.0-20180127040603-bd5ef7bd5415",
    )
    go_repository(
        name = "com_github_xeipuuv_gojsonschema",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xeipuuv/gojsonschema",
        sum = "h1:LhYJRs+L4fBtjZUfuSZIKGeVu0QRy8e5Xi7D17UxZ74=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_xeonx_timeago",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xeonx/timeago",
        sum = "h1:9rRzv48GlJC0vm+iBpLcWAr8YbETyN9Vij+7h2ammz4=",
        version = "v1.0.0-rc4",
    )
    go_repository(
        name = "com_github_xi2_xz",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xi2/xz",
        sum = "h1:nIPpBwaJSVYIxUFsDv3M8ofmx9yWTog9BfvIu0q41lo=",
        version = "v0.0.0-20171230120015-48954b6210f8",
    )
    go_repository(
        name = "com_github_xiang90_probing",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xiang90/probing",
        sum = "h1:eY9dn8+vbi4tKz5Qo6v2eYzo7kUS51QINcR5jNpbZS8=",
        version = "v0.0.0-20190116061207-43a291ad63a2",
    )
    go_repository(
        name = "com_github_xlab_treeprint",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xlab/treeprint",
        sum = "h1:G/1DjNkPpfZCFt9CSh6b5/nY4VimlbHF3Rh4obvtzDk=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_xordataexchange_crypt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xordataexchange/crypt",
        sum = "h1:ESFSdwYZvkeru3RtdrYueztKhOBCSAAzS4Gf+k0tEow=",
        version = "v0.0.3-0.20170626215501-b2862e3d0a77",
    )
    go_repository(
        name = "com_github_xrash_smetrics",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xrash/smetrics",
        sum = "h1:bAn7/zixMGCfxrRTfdpNzjtPYqr8smhKouy9mxVdGPU=",
        version = "v0.0.0-20201216005158-039620a65673",
    )
    go_repository(
        name = "com_github_xsam_otelsql",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/XSAM/otelsql",
        sum = "h1:NsJQS9YhI1+RDsFqE9mW5XIQmPmdF/qa8qQOLZN8XEA=",
        version = "v0.23.0",
    )
    go_repository(
        name = "com_github_yohcop_openid_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yohcop/openid-go",
        sum = "h1:EciJ7ZLETHR3wOtxBvKXx9RV6eyHZpCaSZ1inbBaUXE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_yosssi_ace",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yosssi/ace",
        sum = "h1:tUkIP/BLdKqrlrPwcmH0shwEEhTRHoGnc1wFIWmaBUA=",
        version = "v0.0.5",
    )
    go_repository(
        name = "com_github_youmark_pkcs8",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/youmark/pkcs8",
        sum = "h1:splanxYIlg+5LfHAM6xpdFEAYOk8iySO56hMFq6uLyA=",
        version = "v0.0.0-20181117223130-1be2e3e5546d",
    )
    go_repository(
        name = "com_github_yuin_goldmark",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yuin/goldmark",
        sum = "h1:ALmeCk/px5FSm1MAcFBAsVKZjDuMVj8Tm7FFIlMJnqU=",
        version = "v1.5.2",
    )
    go_repository(
        name = "com_github_yuin_goldmark_emoji",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yuin/goldmark-emoji",
        sum = "h1:ctuWEyzGBwiucEqxzwe0SOYDXPAucOrE9NQC18Wa1os=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_yuin_goldmark_highlighting_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yuin/goldmark-highlighting/v2",
        sum = "h1:Py16JEzkSdKAtEFJjiaYLYBOWGXc1r/xHj/Q/5lA37k=",
        version = "v2.0.0-20220924101305-151362477c87",
    )
    go_repository(
        name = "com_github_yuin_goldmark_meta",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yuin/goldmark-meta",
        sum = "h1:pWw+JLHGZe8Rk0EGsMVssiNb/AaPMHfSRszZeUeiOUc=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_yuin_gopher_lua",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yuin/gopher-lua",
        sum = "h1:k/gmLsJDWwWqbLCur2yWnJzwQEKRcAHXo6seXGuSwWw=",
        version = "v0.0.0-20210529063254-f4c35e4016d9",
    )
    go_repository(
        name = "com_github_yusufpapurcu_wmi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yusufpapurcu/wmi",
        sum = "h1:E1ctvB7uKFMOJw3fdOW32DwGE9I7t++CRUEMKvFoFiw=",
        version = "v1.2.3",
    )
    go_repository(
        name = "com_github_yvasiyarov_go_metrics",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yvasiyarov/go-metrics",
        sum = "h1:+lm10QQTNSBd8DVTNGHx7o/IKu9HYDvLMffDhbyLccI=",
        version = "v0.0.0-20140926110328-57bccd1ccd43",
    )
    go_repository(
        name = "com_github_yvasiyarov_gorelic",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yvasiyarov/gorelic",
        sum = "h1:hlE8//ciYMztlGpl/VA+Zm1AcTPHYkHJPbHqE6WJUXE=",
        version = "v0.0.0-20141212073537-a9bba5b9ab50",
    )
    go_repository(
        name = "com_github_yvasiyarov_newrelic_platform_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yvasiyarov/newrelic_platform_go",
        sum = "h1:ERexzlUfuTvpE74urLSbIQW0Z/6hF9t8U4NsJLaioAY=",
        version = "v0.0.0-20140908184405-b21fdbd4370f",
    )
    go_repository(
        name = "com_github_zeebo_assert",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/zeebo/assert",
        sum = "h1:g7C04CbJuIDKNPFHmsk4hwZDO5O+kntRxzaUoNXj+IQ=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_zeebo_xxh3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/zeebo/xxh3",
        sum = "h1:xZmwmqxHZA8AI603jOQ0tMqmBr9lPeFwGg6d+xy9DC0=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_zenazn_goji",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/zenazn/goji",
        sum = "h1:4lbD8Mx2h7IvloP7r2C0D6ltZP6Ufip8Hn0wmSK5LR8=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_google_cloud_go",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go",
        sum = "h1:LXy9GEO+timppncPIAZoOj3l58LIU9k+kn48AN7IO3Y=",
        version = "v0.110.10",
    )
    go_repository(
        name = "com_google_cloud_go_accessapproval",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/accessapproval",
        sum = "h1:ZvLvJ952zK8pFHINjpMBY5k7LTAp/6pBf50RDMRgBUI=",
        version = "v1.7.4",
    )
    go_repository(
        name = "com_google_cloud_go_accesscontextmanager",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/accesscontextmanager",
        sum = "h1:Yo4g2XrBETBCqyWIibN3NHNPQKUfQqti0lI+70rubeE=",
        version = "v1.8.4",
    )
    go_repository(
        name = "com_google_cloud_go_aiplatform",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/aiplatform",
        sum = "h1:wH7OYl9Vq/5tupok0BPTFY9xaTLb0GxkReHtB5PF7cI=",
        version = "v1.54.0",
    )
    go_repository(
        name = "com_google_cloud_go_analytics",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/analytics",
        sum = "h1:fnV7B8lqyEYxCU0LKk+vUL7mTlqRAq4uFlIthIdr/iA=",
        version = "v0.21.6",
    )
    go_repository(
        name = "com_google_cloud_go_apigateway",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/apigateway",
        sum = "h1:VVIxCtVerchHienSlaGzV6XJGtEM9828Erzyr3miUGs=",
        version = "v1.6.4",
    )
    go_repository(
        name = "com_google_cloud_go_apigeeconnect",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/apigeeconnect",
        sum = "h1:jSoGITWKgAj/ssVogNE9SdsTqcXnryPzsulENSRlusI=",
        version = "v1.6.4",
    )
    go_repository(
        name = "com_google_cloud_go_apigeeregistry",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/apigeeregistry",
        sum = "h1:DSaD1iiqvELag+lV4VnnqUUFd8GXELu01tKVdWZrviE=",
        version = "v0.8.2",
    )
    go_repository(
        name = "com_google_cloud_go_appengine",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/appengine",
        sum = "h1:Qub3fqR7iA1daJWdzjp/Q0Jz0fUG0JbMc7Ui4E9IX/E=",
        version = "v1.8.4",
    )
    go_repository(
        name = "com_google_cloud_go_area120",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/area120",
        sum = "h1:YnSO8m02pOIo6AEOgiOoUDVbw4pf+bg2KLHi4rky320=",
        version = "v0.8.4",
    )
    go_repository(
        name = "com_google_cloud_go_artifactregistry",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/artifactregistry",
        sum = "h1:/hQaadYytMdA5zBh+RciIrXZQBWK4vN7EUsrQHG+/t8=",
        version = "v1.14.6",
    )
    go_repository(
        name = "com_google_cloud_go_asset",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/asset",
        sum = "h1:uI8Bdm81s0esVWbWrTHcjFDFKNOa9aB7rI1vud1hO84=",
        version = "v1.15.3",
    )
    go_repository(
        name = "com_google_cloud_go_assuredworkloads",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/assuredworkloads",
        sum = "h1:FsLSkmYYeNuzDm8L4YPfLWV+lQaUrJmH5OuD37t1k20=",
        version = "v1.11.4",
    )
    go_repository(
        name = "com_google_cloud_go_automl",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/automl",
        sum = "h1:i9tOKXX+1gE7+rHpWKjiuPfGBVIYoWvLNIGpWgPtF58=",
        version = "v1.13.4",
    )
    go_repository(
        name = "com_google_cloud_go_baremetalsolution",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/baremetalsolution",
        sum = "h1:oQiFYYCe0vwp7J8ZmF6siVKEumWtiPFJMJcGuyDVRUk=",
        version = "v1.2.3",
    )
    go_repository(
        name = "com_google_cloud_go_batch",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/batch",
        sum = "h1:mPiIH20a5NU02rucbAmLeO4sLPO9hrTK0BLjdHyW8xw=",
        version = "v1.6.3",
    )
    go_repository(
        name = "com_google_cloud_go_beyondcorp",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/beyondcorp",
        sum = "h1:VXf9SnrnSmj2BF2cHkoTHvOUp8gjsz1KJFOMW7czdsY=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_google_cloud_go_bigquery",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/bigquery",
        sum = "h1:FiULdbbzUxWD0Y4ZGPSVCDLvqRSyCIO6zKV7E2nf5uA=",
        version = "v1.57.1",
    )
    go_repository(
        name = "com_google_cloud_go_billing",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/billing",
        sum = "h1:77/4kCqzH6Ou5CCDzNmqmboE+WvbwFBJmw1QZQz19AI=",
        version = "v1.17.4",
    )
    go_repository(
        name = "com_google_cloud_go_binaryauthorization",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/binaryauthorization",
        sum = "h1:3R6WYn1JKIaVicBmo18jXubu7xh4mMkmbIgsTXk0cBA=",
        version = "v1.7.3",
    )
    go_repository(
        name = "com_google_cloud_go_certificatemanager",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/certificatemanager",
        sum = "h1:5YMQ3Q+dqGpwUZ9X5sipsOQ1fLPsxod9HNq0+nrqc6I=",
        version = "v1.7.4",
    )
    go_repository(
        name = "com_google_cloud_go_channel",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/channel",
        sum = "h1:Rd4+fBrjiN6tZ4TR8R/38elkyEkz6oogGDr7jDyjmMY=",
        version = "v1.17.3",
    )
    go_repository(
        name = "com_google_cloud_go_cloudbuild",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/cloudbuild",
        sum = "h1:9IHfEMWdCklJ1cwouoiQrnxmP0q3pH7JUt8Hqx4Qbck=",
        version = "v1.15.0",
    )
    go_repository(
        name = "com_google_cloud_go_clouddms",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/clouddms",
        sum = "h1:xe/wJKz55VO1+L891a1EG9lVUgfHr9Ju/I3xh1nwF84=",
        version = "v1.7.3",
    )
    go_repository(
        name = "com_google_cloud_go_cloudsqlconn",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/cloudsqlconn",
        sum = "h1:rMtPv66pkuk2K1ciCicjZY8Ma1DSyOYSoqwPUw/Timo=",
        version = "v1.5.1",
    )
    go_repository(
        name = "com_google_cloud_go_cloudtasks",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/cloudtasks",
        sum = "h1:5xXuFfAjg0Z5Wb81j2GAbB3e0bwroCeSF+5jBn/L650=",
        version = "v1.12.4",
    )
    go_repository(
        name = "com_google_cloud_go_compute",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/compute",
        sum = "h1:6sVlXXBmbd7jNX0Ipq0trII3e4n1/MsADLK6a+aiVlk=",
        version = "v1.23.3",
    )
    go_repository(
        name = "com_google_cloud_go_compute_metadata",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/compute/metadata",
        sum = "h1:aRVqY1p2IJaBGStWMsQMpkAa83cPkCDLl80eOj0Rbz4=",
        version = "v0.2.4-0.20230617002413-005d2dfb6b68",
    )
    go_repository(
        name = "com_google_cloud_go_contactcenterinsights",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/contactcenterinsights",
        sum = "h1:wP41IUA4ucMVooj/TP53jd7vbNjWrDkAPOeulVJGT5U=",
        version = "v1.12.0",
    )
    go_repository(
        name = "com_google_cloud_go_container",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/container",
        sum = "h1:/o82CFWXIYnT9p/07SnRgybqL3Pmmu86jYIlzlJVUBY=",
        version = "v1.28.0",
    )
    go_repository(
        name = "com_google_cloud_go_containeranalysis",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/containeranalysis",
        sum = "h1:5rhYLX+3a01drpREqBZVXR9YmWH45RnML++8NsCtuD8=",
        version = "v0.11.3",
    )
    go_repository(
        name = "com_google_cloud_go_datacatalog",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/datacatalog",
        sum = "h1:rbYNmHwvAOOwnW2FPXYkaK3Mf1MmGqRzK0mMiIEyLdo=",
        version = "v1.19.0",
    )
    go_repository(
        name = "com_google_cloud_go_dataflow",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dataflow",
        sum = "h1:7VmCNWcPJBS/srN2QnStTB6nu4Eb5TMcpkmtaPVhRt4=",
        version = "v0.9.4",
    )
    go_repository(
        name = "com_google_cloud_go_dataform",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dataform",
        sum = "h1:jV+EsDamGX6cE127+QAcCR/lergVeeZdEQ6DdrxW3sQ=",
        version = "v0.9.1",
    )
    go_repository(
        name = "com_google_cloud_go_datafusion",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/datafusion",
        sum = "h1:Q90alBEYlMi66zL5gMSGQHfbZLB55mOAg03DhwTTfsk=",
        version = "v1.7.4",
    )
    go_repository(
        name = "com_google_cloud_go_datalabeling",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/datalabeling",
        sum = "h1:zrq4uMmunf2KFDl/7dS6iCDBBAxBnKVDyw6+ajz3yu0=",
        version = "v0.8.4",
    )
    go_repository(
        name = "com_google_cloud_go_dataplex",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dataplex",
        sum = "h1:AfFFR15Ifh4U+Me1IBztrSd5CrasTODzy3x8KtDyHdc=",
        version = "v1.11.2",
    )
    go_repository(
        name = "com_google_cloud_go_dataproc_v2",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dataproc/v2",
        sum = "h1:tTVP9tTxmc8fixxOd/8s6Q6Pz/+yzn7r7XdZHretQH0=",
        version = "v2.3.0",
    )
    go_repository(
        name = "com_google_cloud_go_dataqna",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dataqna",
        sum = "h1:NJnu1kAPamZDs/if3bJ3+Wb6tjADHKL83NUWsaIp2zg=",
        version = "v0.8.4",
    )
    go_repository(
        name = "com_google_cloud_go_datastore",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/datastore",
        sum = "h1:0P9WcsQeTWjuD1H14JIY7XQscIPQ4Laje8ti96IC5vg=",
        version = "v1.15.0",
    )
    go_repository(
        name = "com_google_cloud_go_datastream",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/datastream",
        sum = "h1:Z2sKPIB7bT2kMW5Uhxy44ZgdJzxzE5uKjavoW+EuHEE=",
        version = "v1.10.3",
    )
    go_repository(
        name = "com_google_cloud_go_deploy",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/deploy",
        sum = "h1:ZdmYzRMTGkVyP1nXEUat9FpbJGJemDcNcx82RSSOElc=",
        version = "v1.15.0",
    )
    go_repository(
        name = "com_google_cloud_go_dialogflow",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dialogflow",
        sum = "h1:cK/f88KX+YVR4tLH4clMQlvrLWD2qmKJQziusjGPjmc=",
        version = "v1.44.3",
    )
    go_repository(
        name = "com_google_cloud_go_dlp",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dlp",
        sum = "h1:OFlXedmPP/5//X1hBEeq3D9kUVm9fb6ywYANlpv/EsQ=",
        version = "v1.11.1",
    )
    go_repository(
        name = "com_google_cloud_go_documentai",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/documentai",
        sum = "h1:KAlzT+q8qvRxAmhsJUvLtfFHH0PNvz3M79H6CgVBKL8=",
        version = "v1.23.5",
    )
    go_repository(
        name = "com_google_cloud_go_domains",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/domains",
        sum = "h1:ua4GvsDztZ5F3xqjeLKVRDeOvJshf5QFgWGg1CKti3A=",
        version = "v0.9.4",
    )
    go_repository(
        name = "com_google_cloud_go_edgecontainer",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/edgecontainer",
        sum = "h1:Szy3Q/N6bqgQGyxqjI+6xJZbmvPvnFHp3UZr95DKcQ0=",
        version = "v1.1.4",
    )
    go_repository(
        name = "com_google_cloud_go_errorreporting",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/errorreporting",
        sum = "h1:kj1XEWMu8P0qlLhm3FwcaFsUvXChV/OraZwA70trRR0=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_google_cloud_go_essentialcontacts",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/essentialcontacts",
        sum = "h1:S2if6wkjR4JCEAfDtIiYtD+sTz/oXjh2NUG4cgT1y/Q=",
        version = "v1.6.5",
    )
    go_repository(
        name = "com_google_cloud_go_eventarc",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/eventarc",
        sum = "h1:+pFmO4eu4dOVipSaFBLkmqrRYG94Xl/TQZFOeohkuqU=",
        version = "v1.13.3",
    )
    go_repository(
        name = "com_google_cloud_go_filestore",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/filestore",
        sum = "h1:/+wUEGwk3x3Kxomi2cP5dsR8+SIXxo7M0THDjreFSYo=",
        version = "v1.8.0",
    )
    go_repository(
        name = "com_google_cloud_go_firestore",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/firestore",
        sum = "h1:8aLcKnMPoldYU3YHgu4t2exrKhLQkqaXAGqT0ljrFVw=",
        version = "v1.14.0",
    )
    go_repository(
        name = "com_google_cloud_go_functions",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/functions",
        sum = "h1:ZjdiV3MyumRM6++1Ixu6N0VV9LAGlCX4AhW6Yjr1t+U=",
        version = "v1.15.4",
    )
    go_repository(
        name = "com_google_cloud_go_gkebackup",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/gkebackup",
        sum = "h1:KhnOrr9A1tXYIYeXKqCKbCI8TL2ZNGiD3dm+d7BDUBg=",
        version = "v1.3.4",
    )
    go_repository(
        name = "com_google_cloud_go_gkeconnect",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/gkeconnect",
        sum = "h1:1JLpZl31YhQDQeJ98tK6QiwTpgHFYRJwpntggpQQWis=",
        version = "v0.8.4",
    )
    go_repository(
        name = "com_google_cloud_go_gkehub",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/gkehub",
        sum = "h1:J5tYUtb3r0cl2mM7+YHvV32eL+uZQ7lONyUZnPikCEo=",
        version = "v0.14.4",
    )
    go_repository(
        name = "com_google_cloud_go_gkemulticloud",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/gkemulticloud",
        sum = "h1:NmJsNX9uQ2CT78957xnjXZb26TDIMvv+d5W2vVUt0Pg=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_google_cloud_go_gsuiteaddons",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/gsuiteaddons",
        sum = "h1:uuw2Xd37yHftViSI8J2hUcCS8S7SH3ZWH09sUDLW30Q=",
        version = "v1.6.4",
    )
    go_repository(
        name = "com_google_cloud_go_iam",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/iam",
        sum = "h1:1jTsCu4bcsNsE4iiqNT5SHwrDRCfRmIaaaVFhRveTJI=",
        version = "v1.1.5",
    )
    go_repository(
        name = "com_google_cloud_go_iap",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/iap",
        sum = "h1:M4vDbQ4TLXdaljXVZSwW7XtxpwXUUarY2lIs66m0aCM=",
        version = "v1.9.3",
    )
    go_repository(
        name = "com_google_cloud_go_ids",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/ids",
        sum = "h1:VuFqv2ctf/A7AyKlNxVvlHTzjrEvumWaZflUzBPz/M4=",
        version = "v1.4.4",
    )
    go_repository(
        name = "com_google_cloud_go_iot",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/iot",
        sum = "h1:m1WljtkZnvLTIRYW1YTOv5A6H1yKgLHR6nU7O8yf27w=",
        version = "v1.7.4",
    )
    go_repository(
        name = "com_google_cloud_go_kms",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/kms",
        sum = "h1:pj1sRfut2eRbD9pFRjNnPNg/CzJPuQAzUujMIM1vVeM=",
        version = "v1.15.5",
    )
    go_repository(
        name = "com_google_cloud_go_language",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/language",
        sum = "h1:zg9uq2yS9PGIOdc0Kz/l+zMtOlxKWonZjjo5w5YPG2A=",
        version = "v1.12.2",
    )
    go_repository(
        name = "com_google_cloud_go_lifesciences",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/lifesciences",
        sum = "h1:rZEI/UxcxVKEzyoRS/kdJ1VoolNItRWjNN0Uk9tfexg=",
        version = "v0.9.4",
    )
    go_repository(
        name = "com_google_cloud_go_logging",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/logging",
        sum = "h1:26skQWPeYhvIasWKm48+Eq7oUqdcdbwsCVwz5Ys0FvU=",
        version = "v1.8.1",
    )
    go_repository(
        name = "com_google_cloud_go_longrunning",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/longrunning",
        sum = "h1:w8xEcbZodnA2BbW6sVirkkoC+1gP8wS57EUUgGS0GVg=",
        version = "v0.5.4",
    )
    go_repository(
        name = "com_google_cloud_go_managedidentities",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/managedidentities",
        sum = "h1:SF/u1IJduMqQQdJA4MDyivlIQ4SrV5qAawkr/ZEREkY=",
        version = "v1.6.4",
    )
    go_repository(
        name = "com_google_cloud_go_maps",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/maps",
        sum = "h1:2+eMp/1MvMPp5qrSOd3vtnLKa/pylt+krVRqET3jWsM=",
        version = "v1.6.1",
    )
    go_repository(
        name = "com_google_cloud_go_mediatranslation",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/mediatranslation",
        sum = "h1:VRCQfZB4s6jN0CSy7+cO3m4ewNwgVnaePanVCQh/9Z4=",
        version = "v0.8.4",
    )
    go_repository(
        name = "com_google_cloud_go_memcache",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/memcache",
        sum = "h1:cdex/ayDd294XBj2cGeMe6Y+H1JvhN8y78B9UW7pxuQ=",
        version = "v1.10.4",
    )
    go_repository(
        name = "com_google_cloud_go_metastore",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/metastore",
        sum = "h1:94l/Yxg9oBZjin2bzI79oK05feYefieDq0o5fjLSkC8=",
        version = "v1.13.3",
    )
    go_repository(
        name = "com_google_cloud_go_monitoring",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/monitoring",
        sum = "h1:mf2SN9qSoBtIgiMA4R/y4VADPWZA7VCNJA079qLaZQ8=",
        version = "v1.16.3",
    )
    go_repository(
        name = "com_google_cloud_go_networkconnectivity",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/networkconnectivity",
        sum = "h1:e9lUkCe2BexsqsUc2bjV8+gFBpQa54J+/F3qKVtW+wA=",
        version = "v1.14.3",
    )
    go_repository(
        name = "com_google_cloud_go_networkmanagement",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/networkmanagement",
        sum = "h1:HsQk4FNKJUX04k3OI6gUsoveiHMGvDRqlaFM2xGyvqU=",
        version = "v1.9.3",
    )
    go_repository(
        name = "com_google_cloud_go_networksecurity",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/networksecurity",
        sum = "h1:947tNIPnj1bMGTIEBo3fc4QrrFKS5hh0bFVsHmFm4Vo=",
        version = "v0.9.4",
    )
    go_repository(
        name = "com_google_cloud_go_notebooks",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/notebooks",
        sum = "h1:eTOTfNL1yM6L/PCtquJwjWg7ZZGR0URFaFgbs8kllbM=",
        version = "v1.11.2",
    )
    go_repository(
        name = "com_google_cloud_go_optimization",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/optimization",
        sum = "h1:iFsoexcp13cGT3k/Hv8PA5aK+FP7FnbhwDO9llnruas=",
        version = "v1.6.2",
    )
    go_repository(
        name = "com_google_cloud_go_orchestration",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/orchestration",
        sum = "h1:kgwZ2f6qMMYIVBtUGGoU8yjYWwMTHDanLwM/CQCFaoQ=",
        version = "v1.8.4",
    )
    go_repository(
        name = "com_google_cloud_go_orgpolicy",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/orgpolicy",
        sum = "h1:RWuXQDr9GDYhjmrredQJC7aY7cbyqP9ZuLbq5GJGves=",
        version = "v1.11.4",
    )
    go_repository(
        name = "com_google_cloud_go_osconfig",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/osconfig",
        sum = "h1:OrRCIYEAbrbXdhm13/JINn9pQchvTTIzgmOCA7uJw8I=",
        version = "v1.12.4",
    )
    go_repository(
        name = "com_google_cloud_go_oslogin",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/oslogin",
        sum = "h1:NP/KgsD9+0r9hmHC5wKye0vJXVwdciv219DtYKYjgqE=",
        version = "v1.12.2",
    )
    go_repository(
        name = "com_google_cloud_go_phishingprotection",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/phishingprotection",
        sum = "h1:sPLUQkHq6b4AL0czSJZ0jd6vL55GSTHz2B3Md+TCZI0=",
        version = "v0.8.4",
    )
    go_repository(
        name = "com_google_cloud_go_policytroubleshooter",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/policytroubleshooter",
        sum = "h1:sq+ScLP83d7GJy9+wpwYJVnY+q6xNTXwOdRIuYjvHT4=",
        version = "v1.10.2",
    )
    go_repository(
        name = "com_google_cloud_go_privatecatalog",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/privatecatalog",
        sum = "h1:Vo10IpWKbNvc/z/QZPVXgCiwfjpWoZ/wbgful4Uh/4E=",
        version = "v0.9.4",
    )
    go_repository(
        name = "com_google_cloud_go_profiler",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/profiler",
        sum = "h1:ZeRDZbsOBDyRG0OiK0Op1/XWZ3xeLwJc9zjkzczUxyY=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_google_cloud_go_pubsub",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/pubsub",
        sum = "h1:6SPCPvWav64tj0sVX/+npCBKhUi/UjJehy9op/V3p2g=",
        version = "v1.33.0",
    )
    go_repository(
        name = "com_google_cloud_go_pubsublite",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/pubsublite",
        sum = "h1:pX+idpWMIH30/K7c0epN6V703xpIcMXWRjKJsz0tYGY=",
        version = "v1.8.1",
    )
    go_repository(
        name = "com_google_cloud_go_recaptchaenterprise_v2",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/recaptchaenterprise/v2",
        sum = "h1:KOlLHLv3h3HwcZAkx91ubM3Oztz3JtT3ZacAJhWDorQ=",
        version = "v2.8.4",
    )
    go_repository(
        name = "com_google_cloud_go_recommendationengine",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/recommendationengine",
        sum = "h1:JRiwe4hvu3auuh2hujiTc2qNgPPfVp+Q8KOpsXlEzKQ=",
        version = "v0.8.4",
    )
    go_repository(
        name = "com_google_cloud_go_recommender",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/recommender",
        sum = "h1:VndmgyS/J3+izR8V8BHa7HV/uun8//ivQ3k5eVKKyyM=",
        version = "v1.11.3",
    )
    go_repository(
        name = "com_google_cloud_go_redis",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/redis",
        sum = "h1:J9cEHxG9YLmA9o4jTSvWt/RuVEn6MTrPlYSCRHujxDQ=",
        version = "v1.14.1",
    )
    go_repository(
        name = "com_google_cloud_go_resourcemanager",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/resourcemanager",
        sum = "h1:JwZ7Ggle54XQ/FVYSBrMLOQIKoIT/uer8mmNvNLK51k=",
        version = "v1.9.4",
    )
    go_repository(
        name = "com_google_cloud_go_resourcesettings",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/resourcesettings",
        sum = "h1:yTIL2CsZswmMfFyx2Ic77oLVzfBFoWBYgpkgiSPnC4Y=",
        version = "v1.6.4",
    )
    go_repository(
        name = "com_google_cloud_go_retail",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/retail",
        sum = "h1:geqdX1FNqqL2p0ADXjPpw8lq986iv5GrVcieTYafuJQ=",
        version = "v1.14.4",
    )
    go_repository(
        name = "com_google_cloud_go_run",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/run",
        sum = "h1:qdfZteAm+vgzN1iXzILo3nJFQbzziudkJrvd9wCf3FQ=",
        version = "v1.3.3",
    )
    go_repository(
        name = "com_google_cloud_go_scheduler",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/scheduler",
        sum = "h1:eMEettHlFhG5pXsoHouIM5nRT+k+zU4+GUvRtnxhuVI=",
        version = "v1.10.5",
    )
    go_repository(
        name = "com_google_cloud_go_secretmanager",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/secretmanager",
        sum = "h1:krnX9qpG2kR2fJ+u+uNyNo+ACVhplIAS4Pu7u+4gd+k=",
        version = "v1.11.4",
    )
    go_repository(
        name = "com_google_cloud_go_security",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/security",
        sum = "h1:sdnh4Islb1ljaNhpIXlIPgb3eYj70QWgPVDKOUYvzJc=",
        version = "v1.15.4",
    )
    go_repository(
        name = "com_google_cloud_go_securitycenter",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/securitycenter",
        sum = "h1:qCEyXoJoxNKKA1bDywBjjqCB7ODXazzHnVWnG5Uqd1M=",
        version = "v1.24.2",
    )
    go_repository(
        name = "com_google_cloud_go_servicedirectory",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/servicedirectory",
        sum = "h1:5niCMfkw+jifmFtbBrtRedbXkJm3fubSR/KHbxSJZVM=",
        version = "v1.11.3",
    )
    go_repository(
        name = "com_google_cloud_go_shell",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/shell",
        sum = "h1:nurhlJcSVFZneoRZgkBEHumTYf/kFJptCK2eBUq/88M=",
        version = "v1.7.4",
    )
    go_repository(
        name = "com_google_cloud_go_spanner",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/spanner",
        sum = "h1:/NzWQJ1MEhdRcffiutRKbW/AIGVKhcTeivWTDjEyCCo=",
        version = "v1.53.0",
    )
    go_repository(
        name = "com_google_cloud_go_speech",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/speech",
        sum = "h1:qkxNao58oF8ghAHE1Eghen7XepawYEN5zuZXYWaUTA4=",
        version = "v1.21.0",
    )
    go_repository(
        name = "com_google_cloud_go_storage",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/storage",
        sum = "h1:uOdMxAs8HExqBlnLtnQyP0YkvbiDpdGShGKtx6U/oNM=",
        version = "v1.30.1",
    )
    go_repository(
        name = "com_google_cloud_go_storagetransfer",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/storagetransfer",
        sum = "h1:YM1dnj5gLjfL6aDldO2s4GeU8JoAvH1xyIwXre63KmI=",
        version = "v1.10.3",
    )
    go_repository(
        name = "com_google_cloud_go_talent",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/talent",
        sum = "h1:LnRJhhYkODDBoTwf6BeYkiJHFw9k+1mAFNyArwZUZAs=",
        version = "v1.6.5",
    )
    go_repository(
        name = "com_google_cloud_go_texttospeech",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/texttospeech",
        sum = "h1:ahrzTgr7uAbvebuhkBAAVU6kRwVD0HWsmDsvMhtad5Q=",
        version = "v1.7.4",
    )
    go_repository(
        name = "com_google_cloud_go_tpu",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/tpu",
        sum = "h1:XIEH5c0WeYGaVy9H+UueiTaf3NI6XNdB4/v6TFQJxtE=",
        version = "v1.6.4",
    )
    go_repository(
        name = "com_google_cloud_go_trace",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/trace",
        sum = "h1:2qOAuAzNezwW3QN+t41BtkDJOG42HywL73q8x/f6fnM=",
        version = "v1.10.4",
    )
    go_repository(
        name = "com_google_cloud_go_translate",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/translate",
        sum = "h1:t5WXTqlrk8VVJu/i3WrYQACjzYJiff5szARHiyqqPzI=",
        version = "v1.9.3",
    )
    go_repository(
        name = "com_google_cloud_go_video",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/video",
        sum = "h1:Xrpbm2S9UFQ1pZEeJt9Vqm5t2T/z9y/M3rNXhFoo8Is=",
        version = "v1.20.3",
    )
    go_repository(
        name = "com_google_cloud_go_videointelligence",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/videointelligence",
        sum = "h1:YS4j7lY0zxYyneTFXjBJUj2r4CFe/UoIi/PJG0Zt/Rg=",
        version = "v1.11.4",
    )
    go_repository(
        name = "com_google_cloud_go_vision_v2",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/vision/v2",
        sum = "h1:T/ujUghvEaTb+YnFY/jiYwVAkMbIC8EieK0CJo6B4vg=",
        version = "v2.7.5",
    )
    go_repository(
        name = "com_google_cloud_go_vmmigration",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/vmmigration",
        sum = "h1:qPNdab4aGgtaRX+51jCOtJxlJp6P26qua4o1xxUDjpc=",
        version = "v1.7.4",
    )
    go_repository(
        name = "com_google_cloud_go_vmwareengine",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/vmwareengine",
        sum = "h1:WY526PqM6QNmFHSqe2sRfK6gRpzWjmL98UFkql2+JDM=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_google_cloud_go_vpcaccess",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/vpcaccess",
        sum = "h1:zbs3V+9ux45KYq8lxxn/wgXole6SlBHHKKyZhNJoS+8=",
        version = "v1.7.4",
    )
    go_repository(
        name = "com_google_cloud_go_webrisk",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/webrisk",
        sum = "h1:iceR3k0BCRZgf2D/NiKviVMFfuNC9LmeNLtxUFRB/wI=",
        version = "v1.9.4",
    )
    go_repository(
        name = "com_google_cloud_go_websecurityscanner",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/websecurityscanner",
        sum = "h1:5Gp7h5j7jywxLUp6NTpjNPkgZb3ngl0tUSw6ICWvtJQ=",
        version = "v1.6.4",
    )
    go_repository(
        name = "com_google_cloud_go_workflows",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/workflows",
        sum = "h1:qocsqETmLAl34mSa01hKZjcqAvt699gaoFbooGGMvaM=",
        version = "v1.12.3",
    )
    go_repository(
        name = "com_jolheiser_go_hcaptcha",
        build_file_proto_mode = "disable_global",
        importpath = "go.jolheiser.com/hcaptcha",
        sum = "h1:RrDERcr/Tz/kWyJenjVtI+V09RtLinXxlAemiwN5F+I=",
        version = "v0.0.4",
    )
    go_repository(
        name = "com_jolheiser_go_pwn",
        build_file_proto_mode = "disable_global",
        importpath = "go.jolheiser.com/pwn",
        sum = "h1:MQowb3QvCL5r5NmHmCPxw93SdjfgJ0q6rAwYn4i1Hjg=",
        version = "v0.0.3",
    )
    go_repository(
        name = "com_layeh_gopher_luar",
        build_file_proto_mode = "disable_global",
        importpath = "layeh.com/gopher-luar",
        sum = "h1:55b0mpBhN9XSshEd2Nz6WsbYXctyBT35azk4POQNSXo=",
        version = "v1.0.10",
    )
    go_repository(
        name = "com_lukechampine_uint128",
        build_file_proto_mode = "disable_global",
        importpath = "lukechampine.com/uint128",
        sum = "h1:mBi/5l91vocEN8otkC5bDLhi2KdCticRiwbdB0O+rjI=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_shuralyov_dmitri_gpu_mtl",
        build_file_proto_mode = "disable_global",
        importpath = "dmitri.shuralyov.com/gpu/mtl",
        sum = "h1:VpgP7xuJadIUuKccphEpTJnWhS2jkQyMt6Y7pJCD7fY=",
        version = "v0.0.0-20190408044501-666a987793e9",
    )
    go_repository(
        name = "dev_bobheadxi_go_gobenchdata",
        build_file_proto_mode = "disable_global",
        importpath = "go.bobheadxi.dev/gobenchdata",
        sum = "h1:3Pts2nPUZdgFSU63nWzvfs2xRbK8WVSNeJ2H9e/Ypew=",
        version = "v1.3.1",
    )
    go_repository(
        name = "dev_bobheadxi_go_streamline",
        build_file_proto_mode = "disable_global",
        importpath = "go.bobheadxi.dev/streamline",
        sum = "h1:Mv2NE8svJMB5K7nIT9WGwF014yuY/lPXtT8mvNr1OrU=",
        version = "v1.2.2",
    )
    go_repository(
        name = "ht_sr_git_mariusor_go_xsd_duration",
        build_file_proto_mode = "disable_global",
        importpath = "git.sr.ht/~mariusor/go-xsd-duration",
        sum = "h1:cliQ4HHsCo6xi2oWZYKWW4bly/Ory9FuTpFPRxj/mAg=",
        version = "v0.0.0-20220703122237-02e73435a078",
    )
    go_repository(
        name = "ht_sr_git_sbinet_gg",
        build_file_proto_mode = "disable_global",
        importpath = "git.sr.ht/~sbinet/gg",
        sum = "h1:LNhjNn8DerC8f9DHLz6lS0YYul/b602DUxDgGkd/Aik=",
        version = "v0.3.1",
    )
    go_repository(
        name = "in_gopkg_alecthomas_kingpin_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/alecthomas/kingpin.v2",
        sum = "h1:jMFz6MfLP0/4fUyZle81rXUoxOBFi19VUFKVDOQfozc=",
        version = "v2.2.6",
    )
    go_repository(
        name = "in_gopkg_alexcesaro_quotedprintable_v3",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/alexcesaro/quotedprintable.v3",
        sum = "h1:2gGKlE2+asNV9m7xrywl36YYNnBG5ZQ0r/BOOxqPpmk=",
        version = "v3.0.0-20150716171945-2caba252f4dc",
    )
    go_repository(
        name = "in_gopkg_alexcesaro_statsd_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/alexcesaro/statsd.v2",
        sum = "h1:FXkZSCZIH17vLCO5sO2UucTHsH9pc+17F6pl3JVCwMc=",
        version = "v2.0.0",
    )
    go_repository(
        name = "in_gopkg_asn1_ber_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/asn1-ber.v1",
        sum = "h1:TxyelI5cVkbREznMhfzycHdkp5cLA7DpE+GKjSslYhM=",
        version = "v1.0.0-20181015200546-f715ec2f112d",
    )
    go_repository(
        name = "in_gopkg_check_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/check.v1",
        sum = "h1:Hei/4ADfdWqJk1ZMxUNpqntNwaWcugrBjAiHlqqRiVk=",
        version = "v1.0.0-20201130134442-10cb98267c6c",
    )
    go_repository(
        name = "in_gopkg_cheggaaa_pb_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/cheggaaa/pb.v1",
        sum = "h1:n1tBJnnK2r7g9OW2btFH91V92STTUevLXYFb8gy9EMk=",
        version = "v1.0.28",
    )
    go_repository(
        name = "in_gopkg_errgo_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/errgo.v2",
        sum = "h1:0vLT13EuvQ0hNvakwLuFZ/jYrLp5F3kcWHXdRggjCE8=",
        version = "v2.1.0",
    )
    go_repository(
        name = "in_gopkg_fsnotify_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/fsnotify.v1",
        sum = "h1:xOHLXZwVvI9hhs+cLKq5+I5onOuwQLhQwiu63xxlHs4=",
        version = "v1.4.7",
    )
    go_repository(
        name = "in_gopkg_go_playground_assert_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/go-playground/assert.v1",
        sum = "h1:xoYuJVE7KT85PYWrN730RguIQO0ePzVRfFMXadIrXTM=",
        version = "v1.2.1",
    )
    go_repository(
        name = "in_gopkg_go_playground_validator_v9",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/go-playground/validator.v9",
        sum = "h1:SvGtYmN60a5CVKTOzMSyfzWDeZRxRuGvRQyEAKbw1xc=",
        version = "v9.29.1",
    )
    go_repository(
        name = "in_gopkg_gomail_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/gomail.v2",
        sum = "h1:n7WqCuqOuCbNr617RXOY0AWRXxgwEyPp2z+p0+hgMuE=",
        version = "v2.0.0-20160411212932-81ebce5c23df",
    )
    go_repository(
        name = "in_gopkg_inf_v0",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/inf.v0",
        sum = "h1:73M5CoZyi3ZLMOyDlQh031Cx6N9NDJ2Vvfl76EDAgDc=",
        version = "v0.9.1",
    )
    go_repository(
        name = "in_gopkg_ini_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/ini.v1",
        sum = "h1:Dgnx+6+nfE+IfzjUEISNeydPJh9AXNNsWbGP9KzCsOA=",
        version = "v1.67.0",
    )
    go_repository(
        name = "in_gopkg_natefinch_lumberjack_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/natefinch/lumberjack.v2",
        sum = "h1:bBRl1b0OH9s/DuPhuXpNl+VtCaJXFZ5/uEFST95x9zc=",
        version = "v2.2.1",
    )
    go_repository(
        name = "in_gopkg_resty_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/resty.v1",
        sum = "h1:CuXP0Pjfw9rOuY6EP+UvtNvt5DSqHpIxILZKT/quCZI=",
        version = "v1.12.0",
    )
    go_repository(
        name = "in_gopkg_square_go_jose_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/square/go-jose.v2",
        sum = "h1:NGk74WTnPKBNUhNzQX7PYcTLUjoq7mzKk2OKbvwk2iI=",
        version = "v2.6.0",
    )
    go_repository(
        name = "in_gopkg_tomb_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/tomb.v1",
        sum = "h1:uRGJdciOHaEIrze2W8Q3AKkepLTh2hOroT7a+7czfdQ=",
        version = "v1.0.0-20141024135613-dd632973f1e7",
    )
    go_repository(
        name = "in_gopkg_warnings_v0",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/warnings.v0",
        sum = "h1:wFXVbFY8DY5/xOe1ECiWdKCzZlxgshcYVNkBHstARME=",
        version = "v0.1.2",
    )
    go_repository(
        name = "in_gopkg_yaml_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/yaml.v2",
        sum = "h1:D8xgwECY7CYvx+Y2n4sBz93Jn9JRvxdiyyo8CTfuKaY=",
        version = "v2.4.0",
    )
    go_repository(
        name = "in_gopkg_yaml_v3",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/yaml.v3",
        sum = "h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=",
        version = "v3.0.1",
    )
    go_repository(
        name = "io_etcd_go_bbolt",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/bbolt",
        sum = "h1:/ecaJf0sk1l4l6V4awd65v2C3ILy7MSj+s/x1ADCIMU=",
        version = "v1.3.6",
    )
    go_repository(
        name = "io_etcd_go_etcd_api_v3",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/api/v3",
        sum = "h1:OHVyt3TopwtUQ2GKdd5wu3PmmipR4FTwCqoEjSyRdIc=",
        version = "v3.5.4",
    )
    go_repository(
        name = "io_etcd_go_etcd_client_pkg_v3",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/client/pkg/v3",
        sum = "h1:lrneYvz923dvC14R54XcA7FXoZ3mlGZAgmwhfm7HqOg=",
        version = "v3.5.4",
    )
    go_repository(
        name = "io_etcd_go_etcd_client_v2",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/client/v2",
        sum = "h1:Dcx3/MYyfKcPNLpR4VVQUP5KgYrBeJtktBwEKkw08Ao=",
        version = "v2.305.4",
    )
    go_repository(
        name = "io_etcd_go_etcd_client_v3",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/client/v3",
        sum = "h1:p83BUL3tAYS0OT/r0qglgc3M1JjhM0diV8DSWAhVXv4=",
        version = "v3.5.4",
    )
    go_repository(
        name = "io_etcd_go_etcd_etcdctl_v3",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/etcdctl/v3",
        sum = "h1:odMFuQQCg0UmPd7Cyw6TViRYv9ybGuXuki4CusDSzqA=",
        version = "v3.5.0-alpha.0",
    )
    go_repository(
        name = "io_etcd_go_etcd_pkg_v3",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/pkg/v3",
        sum = "h1:3yLUEC0nFCxw/RArImOyRUI4OAFbg4PFpBbAhSNzKNY=",
        version = "v3.5.0-alpha.0",
    )
    go_repository(
        name = "io_etcd_go_etcd_raft_v3",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/raft/v3",
        sum = "h1:DvYJotxV9q1Lkn7pknzAbFO/CLtCVidCr2K9qRLJ8pA=",
        version = "v3.5.0-alpha.0",
    )
    go_repository(
        name = "io_etcd_go_etcd_server_v3",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/server/v3",
        sum = "h1:fYv7CmmdyuIu27UmKQjS9K/1GtcCa+XnPKqiKBbQkrk=",
        version = "v3.5.0-alpha.0",
    )
    go_repository(
        name = "io_etcd_go_etcd_tests_v3",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/tests/v3",
        sum = "h1:UcRoCA1FgXoc4CEM8J31fqEvI69uFIObY5ZDEFH7Znc=",
        version = "v3.5.0-alpha.0",
    )
    go_repository(
        name = "io_etcd_go_etcd_v3",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/v3",
        sum = "h1:ZuqKJkD2HrzFUj8IB+GLkTMKZ3+7mWx172vx6F1TukM=",
        version = "v3.5.0-alpha.0",
    )
    go_repository(
        name = "io_gitea_code_gitea",
        build_file_proto_mode = "disable_global",
        importpath = "code.gitea.io/gitea",
        sum = "h1:qQXVeKHoFXywWVGGDmTOKxvzOK282EAPTw3qo2bgXAk=",
        version = "v1.18.0",
    )
    go_repository(
        name = "io_gitea_code_gitea_vet",
        build_file_proto_mode = "disable_global",
        importpath = "code.gitea.io/gitea-vet",
        sum = "h1:uv9a8eGSdQ8Dr4HyUcuHFfDsk/QuwO+wf+Y99RYdxY0=",
        version = "v0.2.2-0.20220122151748-48ebc902541b",
    )
    go_repository(
        name = "io_gitea_code_sdk_gitea",
        build_file_proto_mode = "disable_global",
        importpath = "code.gitea.io/sdk/gitea",
        sum = "h1:WJreC7YYuxbn0UDaPuWIe/mtiNKTvLN8MLkaw71yx/M=",
        version = "v0.15.1",
    )
    go_repository(
        name = "io_gorm_driver_postgres",
        build_file_proto_mode = "disable_global",
        importpath = "gorm.io/driver/postgres",
        sum = "h1:Iyrp9Meh3GmbSuyIAGyjkN+n9K+GHX9b9MqsTL4EJCo=",
        version = "v1.5.4",
    )
    go_repository(
        name = "io_gorm_gorm",
        build_file_proto_mode = "disable_global",
        importpath = "gorm.io/gorm",
        sum = "h1:zR9lOiiYf09VNh5Q1gphfyia1JpiClIWG9hQaxB/mls=",
        version = "v1.25.5",
    )
    go_repository(
        name = "io_k8s_api",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/api",
        sum = "h1:3YO8J4RtmG7elEgaWMb4HgmpS2CfY1QlaOz9nwB+ZSs=",
        version = "v0.25.4",
    )
    go_repository(
        name = "io_k8s_apimachinery",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/apimachinery",
        sum = "h1:CtXsuaitMESSu339tfhVXhQrPET+EiWnIY1rcurKnAc=",
        version = "v0.25.4",
    )
    go_repository(
        name = "io_k8s_client_go",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/client-go",
        sum = "h1:3RNRDffAkNU56M/a7gUfXaEzdhZlYhoW8dgViGy5fn8=",
        version = "v0.25.4",
    )
    go_repository(
        name = "io_k8s_gengo",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/gengo",
        sum = "h1:GohjlNKauSai7gN4wsJkeZ3WAJx4Sh+oT/b5IYn5suA=",
        version = "v0.0.0-20210813121822-485abfe95c7c",
    )
    go_repository(
        name = "io_k8s_klog",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/klog",
        sum = "h1:Pt+yjF5aB1xDSVbau4VsWe+dQNzA0qv1LlXdC2dF6Q8=",
        version = "v1.0.0",
    )
    go_repository(
        name = "io_k8s_klog_v2",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/klog/v2",
        sum = "h1:lyJt0TWMPaGoODa8B8bUuxgHS3W/m/bNr2cca3brA/g=",
        version = "v2.80.0",
    )
    go_repository(
        name = "io_k8s_kube_openapi",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/kube-openapi",
        sum = "h1:MQ8BAZPZlWk3S9K4a9NCkIFQtZShWqoha7snGixVgEA=",
        version = "v0.0.0-20220803162953-67bda5d908f1",
    )
    go_repository(
        name = "io_k8s_sigs_json",
        build_file_proto_mode = "disable_global",
        importpath = "sigs.k8s.io/json",
        sum = "h1:iXTIw73aPyC+oRdyqqvVJuloN1p0AC/kzH07hu3NE+k=",
        version = "v0.0.0-20220713155537-f223a00ba0e2",
    )
    go_repository(
        name = "io_k8s_sigs_kustomize_kyaml",
        build_file_proto_mode = "disable_global",
        importpath = "sigs.k8s.io/kustomize/kyaml",
        sum = "h1:tNNQIC+8cc+aXFTVg+RtQAOsjwUdYBZRAgYOVI3RBc4=",
        version = "v0.13.3",
    )
    go_repository(
        name = "io_k8s_sigs_structured_merge_diff_v4",
        build_file_proto_mode = "disable_global",
        importpath = "sigs.k8s.io/structured-merge-diff/v4",
        sum = "h1:PRbqxJClWWYMNV1dhaG4NsibJbArud9kFxnAMREiWFE=",
        version = "v4.2.3",
    )
    go_repository(
        name = "io_k8s_sigs_yaml",
        build_file_proto_mode = "disable_global",
        importpath = "sigs.k8s.io/yaml",
        sum = "h1:a2VclLzOGrwOHDiV8EfBGhvjHvP46CtW5j6POvhYGGo=",
        version = "v1.3.0",
    )
    go_repository(
        name = "io_k8s_utils",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/utils",
        sum = "h1:jAne/RjBTyawwAy0utX5eqigAwz/lQhTmy+Hr/Cpue4=",
        version = "v0.0.0-20220728103510-ee6ede2d64ed",
    )
    go_repository(
        name = "io_kbt_strk_projects_go_libravatar",
        build_file_proto_mode = "disable_global",
        importpath = "strk.kbt.io/projects/go/libravatar",
        sum = "h1:mUcz5b3FJbP5Cvdq7Khzn6J9OCUQJaBwgBkCR+MOwSs=",
        version = "v0.0.0-20191008002943-06d1c002b251",
    )
    go_repository(
        name = "io_opencensus_go",
        build_file_proto_mode = "disable_global",
        importpath = "go.opencensus.io",
        sum = "h1:y73uSU6J157QMP2kn2r30vwW1A2W2WFwSCGnAVxeaD0=",
        version = "v0.24.0",
    )
    go_repository(
        name = "io_opencensus_go_contrib_exporter_prometheus",
        build_file_proto_mode = "disable_global",
        importpath = "contrib.go.opencensus.io/exporter/prometheus",
        sum = "h1:sqfsYl5GIY/L570iT+l93ehxaWJs2/OwXtiWwew3oAg=",
        version = "v0.4.2",
    )
    go_repository(
        name = "io_opentelemetry_go_collector",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector",
        sum = "h1:pF+sB8xNXlg/W0a0QTLz4mUWyool1a9toVj8LmLoFqg=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_component",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/component",
        sum = "h1:AKsl6bss/SRrW248GFpmGiiI/4kdemW92Ai/X82CCqY=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_config_configauth",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/config/configauth",
        sum = "h1:NIiJuIGOdblN0EIJv64R2mvGhthcYfWuvyCnjk8HRN4=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_config_configcompression",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/config/configcompression",
        sum = "h1:Q725pvVH7tR6BP3WK7Ro3pbqMeQdZEV3KeFVHchBxCc=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_config_configgrpc",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/config/configgrpc",
        sum = "h1:Q2xEE2SGbg79j3TdHT+781eUu/2uUIyrHVJAG9bLpVk=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_config_confighttp",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/config/confighttp",
        sum = "h1:vIdiepUT7P/WtJRdfh8mjzvSqJRVF8/vl9GWtUNQlHQ=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_config_confignet",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/config/confignet",
        sum = "h1:Eu8m3eX8GaGhOUc//YXvV4i3cEivxUSxkLnV1U9ydhg=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_config_configopaque",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/config/configopaque",
        sum = "h1:MkCAGh0WydRWydETB9FLnuCj9hDPDiz2g4Wxnl53I0w=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_config_configtelemetry",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/config/configtelemetry",
        sum = "h1:j3dhWbAcrfL1n0RmShRJf99X/xIMoPfEShN/5Z8bY0k=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_config_configtls",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/config/configtls",
        sum = "h1:2vt+yOZUvGq5ADqFAxL5ONm1ACuGXDSs87AWT54Ez4M=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_config_internal",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/config/internal",
        sum = "h1:wRV2PBnJygdmKpIdt/xfG7zdQvXvHz9L+z8MhGsOji4=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_confmap",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/confmap",
        sum = "h1:AqweoBGdF3jGM2/KgP5GS6bmN+1aVrEiCy4nPf7IBE4=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_connector",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/connector",
        sum = "h1:5jYYjQwxxgJKFtVvvbFLd0+2QHsvS0z+lVDxzmRv8uk=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_consumer",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/consumer",
        sum = "h1:8R2iCrSzD7T0RtC2Wh4GXxDiqla2vNhDokGW6Bcrfas=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_exporter",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/exporter",
        sum = "h1:GLhB8WGrBx+zZSB1HIOx2ivFUMahGtAVO2CC5xbCUHQ=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_exporter_otlpexporter",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/exporter/otlpexporter",
        sum = "h1:Ri5pj0slm+FUbbG81UIhQaQ992z2+PcT2++4JI32XGI=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_exporter_otlphttpexporter",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/exporter/otlphttpexporter",
        sum = "h1:KSE7wjy1J0I0izLTodTW4axRmJplpQgCRqYFbAzufZo=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_extension",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/extension",
        sum = "h1:Ak7AzZzxTFJxGyVbEklsGzqHyOHW5USiifJilCcRyTU=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_extension_auth",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/extension/auth",
        sum = "h1:UzVQSG9naJh1hX7hh+HVcvB3n+rpCJXX2BBdUoL/Ybo=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_extension_zpagesextension",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/extension/zpagesextension",
        sum = "h1:ov3h5re95uJcF6N+vR/rLpjsEkGs6easxXSphH9UrPg=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_featuregate",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/featuregate",
        sum = "h1:tiTUG9X/gEDN1oDYQOBVUFYQfhUG2CvgW9VhBc2uk1U=",
        version = "v1.0.0-rcv0013",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_pdata",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/pdata",
        sum = "h1:4sONXE9hAX+4Di8m0bQ/KaoH3Mi+OPt04cXkZ7A8W3k=",
        version = "v1.0.0-rcv0013",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_processor",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/processor",
        sum = "h1:ypyNV5R0bnN3XGMAsH/q5eNARF5vXtFgSOK9rBWzsLc=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_receiver",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/receiver",
        sum = "h1:0c+YtIV7fmd9ev+zmwS9qjx5ASi8cw+gSypu4I7Gugc=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_receiver_otlpreceiver",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/receiver/otlpreceiver",
        sum = "h1:ewVbfATnAeQkwFK3r0dpFKCXcTb8HJKX4AixUioRt+c=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_collector_semconv",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/collector/semconv",
        sum = "h1:lCYNNo3powDvFIaTPP2jDKIrBiV1T92NK4QgL/aHYXw=",
        version = "v0.81.0",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_detectors_gcp",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/contrib/detectors/gcp",
        sum = "h1:SsuF2+gqrnmTKSz+KLXcx3A4A7PZXqbuRZbm4I6HcX0=",
        version = "v1.17.0",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_instrumentation_google_golang_org_grpc_otelgrpc",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc",
        sum = "h1:7XZai4VhA473clBrOqqHdjHBImGfyEtv0qW4nnn/kAo=",
        version = "v0.43.0",
    )

    # See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/872
    go_repository(
        name = "io_opentelemetry_go_contrib_instrumentation_net_http_httptrace_otelhttptrace",
        build_directives = [
            "gazelle:resolve go go.opentelemetry.io/otel/exporters/otlp/internal @io_opentelemetry_go_otel//exporters/otlp/internal",
            "gazelle:resolve go go.opentelemetry.io/otel/exporters/otlp/internal/envconfig @io_opentelemetry_go_otel//exporters/otlp/internal/envconfig",
        ],
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace",
        sum = "h1:Wjp9vsVSIEyvdiaECfqxY9xBqQ7JaSCGtvHgR4doXZk=",
        version = "v0.29.0",
    )

    # See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/872
    go_repository(
        name = "io_opentelemetry_go_contrib_instrumentation_net_http_otelhttp",
        build_directives = [
            "gazelle:resolve go go.opentelemetry.io/otel/exporters/otlp/internal @io_opentelemetry_go_otel//exporters/otlp/internal",
        ],
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
        sum = "h1:pginetY7+onl4qN1vl0xW/V/v6OBZ0vVdH+esuJgvmM=",
        version = "v0.42.0",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_propagators_b3",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/contrib/propagators/b3",
        sum = "h1:ImOVvHnku8jijXqkwCSyYKRDt2YrnGXD4BbhcpfbfJo=",
        version = "v1.17.0",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_propagators_jaeger",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/contrib/propagators/jaeger",
        sum = "h1:Zbpbmwav32Ea5jSotpmkWEl3a6Xvd4tw/3xxGO1i05Y=",
        version = "v1.17.0",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_propagators_ot",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/contrib/propagators/ot",
        sum = "h1:ufo2Vsz8l76eI47jFjuVyjyB3Ae2DmfiCV/o6Vc8ii0=",
        version = "v1.17.0",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_zpages",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/contrib/zpages",
        sum = "h1:hFscXKQ9PTjyIVmAr6zIV8cMoiEeR9lPIwPVqHi8+5Q=",
        version = "v0.42.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel",
        sum = "h1:MW+phZ6WZ5/uk2nd93ANk/6yJ+dVrvNWUjGhnnFU5jM=",
        version = "v1.17.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_bridge_opencensus",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/bridge/opencensus",
        sum = "h1:YHivttTaDhbZIHuPlg1sWsy2P5gj57vzqPfkHItgbwQ=",
        version = "v0.39.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_bridge_opentracing",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/bridge/opentracing",
        sum = "h1:Bgwi7P5NCV3bv2T13bwG0WfsxaT4SjQ1rDdmFc5P7do=",
        version = "v1.16.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_exporters_jaeger",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/exporters/jaeger",
        sum = "h1:YhxxmXZ011C0aDZKoNw+juVWAmEfv/0W2XBOv9aHTaA=",
        version = "v1.16.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_exporters_otlp_internal_retry",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/exporters/otlp/internal/retry",
        sum = "h1:t4ZwRPU+emrcvM2e9DHd0Fsf0JTPVcbfa/BhTDF03d0=",
        version = "v1.16.0",
    )

    # See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/872
    go_repository(
        name = "io_opentelemetry_go_otel_exporters_otlp_otlptrace",
        build_directives = [
            "gazelle:resolve go go.opentelemetry.io/otel/exporters/otlp/internal @io_opentelemetry_go_otel//exporters/otlp/internal",
            "gazelle:resolve go go.opentelemetry.io/otel/exporters/otlp/internal/envconfig @io_opentelemetry_go_otel//exporters/otlp/internal/envconfig",
        ],
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace",
        sum = "h1:cbsD4cUcviQGXdw8+bo5x2wazq10SKz8hEbtCRPcU78=",
        version = "v1.16.0",
    )

    # See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/872
    go_repository(
        name = "io_opentelemetry_go_otel_exporters_otlp_otlptrace_otlptracegrpc",
        build_directives = [
            "gazelle:resolve go go.opentelemetry.io/otel/exporters/otlp/internal @io_opentelemetry_go_otel//exporters/otlp/internal",
            "gazelle:resolve go go.opentelemetry.io/otel/exporters/otlp/internal/envconfig @io_opentelemetry_go_otel//exporters/otlp/internal/envconfig",
        ],
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc",
        sum = "h1:TVQp/bboR4mhZSav+MdgXB8FaRho1RC8UwVn3T0vjVc=",
        version = "v1.16.0",
    )

    # See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/872
    go_repository(
        name = "io_opentelemetry_go_otel_exporters_otlp_otlptrace_otlptracehttp",
        build_directives = [
            "gazelle:resolve go go.opentelemetry.io/otel/exporters/otlp/internal @io_opentelemetry_go_otel//exporters/otlp/internal",
            "gazelle:resolve go go.opentelemetry.io/otel/exporters/otlp/internal/envconfig @io_opentelemetry_go_otel//exporters/otlp/internal/envconfig",
        ],
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
        sum = "h1:iqjq9LAB8aK++sKVcELezzn655JnBNdsDhghU4G/So8=",
        version = "v1.16.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_exporters_prometheus",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/exporters/prometheus",
        sum = "h1:whAaiHxOatgtKd+w0dOi//1KUxj3KoPINZdtDaDj3IA=",
        version = "v0.39.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_internal_metric",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/internal/metric",
        sum = "h1:9dAVGAfFiiEq5NVB9FUJ5et+btbDQAUIJehJ+ikyryk=",
        version = "v0.27.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_metric",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/metric",
        sum = "h1:iG6LGVz5Gh+IuO0jmgvpTB6YVrCGngi8QGm+pMd8Pdc=",
        version = "v1.17.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_sdk",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/sdk",
        sum = "h1:Z1Ok1YsijYL0CSJpHt4cS3wDDh7p572grzNrBMiMWgE=",
        version = "v1.16.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_sdk_metric",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/sdk/metric",
        sum = "h1:Kun8i1eYf48kHH83RucG93ffz0zGV1sh46FAScOTuDI=",
        version = "v0.39.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_trace",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/trace",
        sum = "h1:/SWhSRHmDPOImIAetP1QAeMnZYiQXrTy4fMMYOdSKWQ=",
        version = "v1.17.0",
    )
    go_repository(
        name = "io_opentelemetry_go_proto_otlp",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/proto/otlp",
        sum = "h1:T0TX0tmXU8a3CbNXzEKGeU5mIVOdf0oykP+u2lIVU/I=",
        version = "v1.0.0",
    )
    go_repository(
        name = "io_rsc_binaryregexp",
        build_file_proto_mode = "disable_global",
        importpath = "rsc.io/binaryregexp",
        sum = "h1:HfqmD5MEmC0zvwBuF187nq9mdnXjXsSivRiXN7SmRkE=",
        version = "v0.2.0",
    )
    go_repository(
        name = "io_rsc_pdf",
        build_file_proto_mode = "disable_global",
        importpath = "rsc.io/pdf",
        sum = "h1:k1MczvYDUvJBe93bYd7wrZLLUEcLZAuF824/I4e5Xr4=",
        version = "v0.1.1",
    )
    go_repository(
        name = "io_rsc_quote_v3",
        build_file_proto_mode = "disable_global",
        importpath = "rsc.io/quote/v3",
        sum = "h1:9JKUTTIUgS6kzR9mK1YuGKv6Nl+DijDNIc0ghT58FaY=",
        version = "v3.1.0",
    )
    go_repository(
        name = "io_rsc_sampler",
        build_file_proto_mode = "disable_global",
        importpath = "rsc.io/sampler",
        sum = "h1:7uVkIFmeBqHfdjD+gZwtXXI+RODJ2Wc4O7MPEh/QiW4=",
        version = "v1.3.0",
    )
    go_repository(
        name = "io_xorm_builder",
        build_file_proto_mode = "disable_global",
        importpath = "xorm.io/builder",
        sum = "h1:naLkJitGyYW7ZZdncsh/JW+HF4HshmvTHTyUyPwJS00=",
        version = "v0.3.11",
    )
    go_repository(
        name = "io_xorm_xorm",
        build_file_proto_mode = "disable_global",
        importpath = "xorm.io/xorm",
        sum = "h1:3NvNsM4lnttTsHpk8ODHqrwN1MCEjsO3bD/rpd8A47k=",
        version = "v1.3.2-0.20220714055524-c3bce556200f",
    )
    go_repository(
        name = "net_starlark_go",
        build_file_proto_mode = "disable_global",
        importpath = "go.starlark.net",
        sum = "h1:+FNtrFTmVw0YZGpBGX56XDee331t6JAXeK2bcyhLOOc=",
        version = "v0.0.0-20200306205701-8dd3e2ee1dd5",
    )
    go_repository(
        name = "org_bitbucket_creachadair_shell",
        build_file_proto_mode = "disable_global",
        importpath = "bitbucket.org/creachadair/shell",
        sum = "h1:Z96pB6DkSb7F3Y3BBnJeOZH2gazyMTWlvecSD4vDqfk=",
        version = "v0.0.7",
    )
    go_repository(
        name = "org_codeberg_gusted_mcaptcha",
        build_file_proto_mode = "disable_global",
        importpath = "codeberg.org/gusted/mcaptcha",
        sum = "h1:TXbikPqa7YRtfU9vS6QJBg77pUvbEb6StRdZO8t1bEY=",
        version = "v0.0.0-20220723083913-4f3072e1d570",
    )
    go_repository(
        name = "org_cuelang_go",
        build_file_proto_mode = "disable_global",
        importpath = "cuelang.org/go",
        sum = "h1:W3oBBjDTm7+IZfCKZAmC8uDG0eYfJL4Pp/xbbCMKaVo=",
        version = "v0.4.3",
    )
    go_repository(
        name = "org_golang_google_api",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/api",
        sum = "h1:Z9k22qD289SZ8gCJrk4DrWXkNjtfvKAUo/l1ma8eBYE=",
        version = "v0.150.0",
    )
    go_repository(
        name = "org_golang_google_appengine",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/appengine",
        sum = "h1:FZR1q0exgwxzPzp/aF+VccGrSfxfPpkBqjIIEq3ru6c=",
        version = "v1.6.7",
    )
    go_repository(
        name = "org_golang_google_cloud",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/cloud",
        sum = "h1:Cpp2P6TPjujNoC5M2KHY6g7wfyLYfIWRZaSdIKfDasA=",
        version = "v0.0.0-20151119220103-975617b05ea8",
    )
    go_repository(
        name = "org_golang_google_genproto",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/genproto",
        sum = "h1:W12Pwm4urIbRdGhMEg2NM9O3TWKjNcxQhs46V0ypf/k=",
        version = "v0.0.0-20231127180814-3a041ad873d4",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_api",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/genproto/googleapis/api",
        sum = "h1:ZcOkrmX74HbKFYnpPY8Qsw93fC29TbJXspYKaBkSXDQ=",
        version = "v0.0.0-20231127180814-3a041ad873d4",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_bytestream",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/genproto/googleapis/bytestream",
        sum = "h1:o4S3HvTUEXgRsNSUQsALDVog0O9F/U1JJlHmmUN8Uas=",
        version = "v0.0.0-20231030173426-d783a09b4405",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_rpc",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/genproto/googleapis/rpc",
        sum = "h1:DC7wcm+i+P1rN3Ff07vL+OndGg5OhNddHyTA+ocPqYE=",
        version = "v0.0.0-20231127180814-3a041ad873d4",
    )
    go_repository(
        name = "org_golang_google_grpc",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/grpc",
        sum = "h1:Z5Iec2pjwb+LEOqzpB2MR12/eKFhDPhuqW91O+4bwUk=",
        version = "v1.59.0",
    )
    go_repository(
        name = "org_golang_google_protobuf",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/protobuf",
        sum = "h1:g0LDEJHgrBl9N9r17Ru3sqWhkIx2NB67okBHPwC7hs8=",
        version = "v1.31.0",
    )
    go_repository(
        name = "org_golang_x_crypto",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/crypto",
        sum = "h1:PGVlW0xEltQnzFZ55hkuX5+KLyrMYhHld1YHO4AKcdc=",
        version = "v0.18.0",
    )
    go_repository(
        name = "org_golang_x_exp",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/exp",
        sum = "h1:MGwJjxBy0HJshjDNfLsYO8xppfqWlA5ZT9OhtUUhTNw=",
        version = "v0.0.0-20230713183714-613f0c0eb8a1",
    )
    go_repository(
        name = "org_golang_x_image",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/image",
        sum = "h1:bR8b5okrPI3g/gyZakLZHeWxAR8Dn5CyxXv1hLH5g/4=",
        version = "v0.6.0",
    )
    go_repository(
        name = "org_golang_x_lint",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/lint",
        sum = "h1:VLliZ0d+/avPrXXH+OakdXhpJuEoBZuwh1m2j7U6Iug=",
        version = "v0.0.0-20210508222113-6edffad5e616",
    )
    go_repository(
        name = "org_golang_x_mobile",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/mobile",
        sum = "h1:4+4C/Iv2U4fMZBiMCc98MG1In4gJY5YRhtpDNeDeHWs=",
        version = "v0.0.0-20190719004257-d2bd2a29d028",
    )
    go_repository(
        name = "org_golang_x_mod",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/mod",
        sum = "h1:dGoOF9QVLYng8IHTm7BAyWqCqSheQ5pYWGhzW00YJr0=",
        version = "v0.14.0",
    )
    go_repository(
        name = "org_golang_x_net",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/net",
        sum = "h1:mIYleuAkSbHh0tCv7RvjL3F6ZVbLjq4+R7zbOn3Kokg=",
        version = "v0.18.0",
    )
    go_repository(
        name = "org_golang_x_oauth2",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/oauth2",
        sum = "h1:P0Vrf/2538nmC0H+pEQ3MNFRRnVR7RlqyVw+bvm26z0=",
        version = "v0.14.0",
    )
    go_repository(
        name = "org_golang_x_sync",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/sync",
        sum = "h1:60k92dhOjHxJkrqnwsfl8KuaHbn/5dl0lUPUklKo3qE=",
        version = "v0.5.0",
    )
    go_repository(
        name = "org_golang_x_sys",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/sys",
        sum = "h1:xWw16ngr6ZMtmxDyKyIgsE93KNKz5HKmMa3b8ALHidU=",
        version = "v0.16.0",
    )
    go_repository(
        name = "org_golang_x_term",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/term",
        sum = "h1:m+B6fahuftsE9qjo0VWp2FW0mB3MTJvR0BaMQrq0pmE=",
        version = "v0.16.0",
    )
    go_repository(
        name = "org_golang_x_text",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/text",
        sum = "h1:ScX5w1eTa3QqT8oi6+ziP7dTV1S2+ALU0bI+0zXKWiQ=",
        version = "v0.14.0",
    )
    go_repository(
        name = "org_golang_x_time",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/time",
        sum = "h1:Z81tqI5ddIoXDPvVQ7/7CC9TnLM7ubaFG2qXYd5BbYY=",
        version = "v0.4.0",
    )
    go_repository(
        name = "org_golang_x_tools",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/tools",
        sum = "h1:zdAyfUGbYmuVokhzVmghFl2ZJh5QhcfebBgmVPFYA+8=",
        version = "v0.15.0",
    )
    go_repository(
        name = "org_golang_x_xerrors",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/xerrors",
        sum = "h1:H2TDz8ibqkAF6YGhCdN3jS9O0/s90v0rJh3X/OLHEUk=",
        version = "v0.0.0-20220907171357-04be3eba64a2",
    )
    go_repository(
        name = "org_gonum_v1_gonum",
        build_file_proto_mode = "disable_global",
        importpath = "gonum.org/v1/gonum",
        sum = "h1:a0T3bh+7fhRyqeNbiC3qVHYmkiQgit3wnNan/2c0HMM=",
        version = "v0.13.0",
    )
    go_repository(
        name = "org_gonum_v1_plot",
        build_file_proto_mode = "disable_global",
        importpath = "gonum.org/v1/plot",
        sum = "h1:dnifSs43YJuNMDzB7v8wV64O4ABBHReuAVAoBxqBqS4=",
        version = "v0.10.1",
    )
    go_repository(
        name = "org_modernc_cc_v3",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/cc/v3",
        sum = "h1:P3g79IUS/93SYhtoeaHW+kRCIrYaxJ27MFPv+7kaTOw=",
        version = "v3.40.0",
    )
    go_repository(
        name = "org_modernc_ccgo_v3",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/ccgo/v3",
        sum = "h1:Mkgdzl46i5F/CNR/Kj80Ri59hC8TKAhZrYSaqvkwzUw=",
        version = "v3.16.13",
    )
    go_repository(
        name = "org_modernc_libc",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/libc",
        sum = "h1:4U7v51GyhlWqQmwCHj28Rdq2Yzwk55ovjFrdPjs8Hb0=",
        version = "v1.22.2",
    )
    go_repository(
        name = "org_modernc_mathutil",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/mathutil",
        sum = "h1:rV0Ko/6SfM+8G+yKiyI830l3Wuz1zRutdslNoQ0kfiQ=",
        version = "v1.5.0",
    )
    go_repository(
        name = "org_modernc_memory",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/memory",
        sum = "h1:N+/8c5rE6EqugZwHii4IFsaJ7MUhoWX07J5tC/iI5Ds=",
        version = "v1.5.0",
    )
    go_repository(
        name = "org_modernc_opt",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/opt",
        sum = "h1:3XOZf2yznlhC+ibLltsDGzABUGVx8J6pnFMS3E4dcq4=",
        version = "v0.1.3",
    )
    go_repository(
        name = "org_modernc_sqlite",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/sqlite",
        sum = "h1:S2uFiaNPd/vTAP/4EmyY8Qe2Quzu26A2L1e25xRNTio=",
        version = "v1.18.2",
    )
    go_repository(
        name = "org_modernc_strutil",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/strutil",
        sum = "h1:fNMm+oJklMGYfU9Ylcywl0CO5O6nTfaowNsh2wpPjzY=",
        version = "v1.1.3",
    )
    go_repository(
        name = "org_modernc_token",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/token",
        sum = "h1:Xl7Ap9dKaEs5kLoOQeQmPWevfnk/DM5qcLcYlA8ys6Y=",
        version = "v1.1.0",
    )
    go_repository(
        name = "org_mongodb_go_mongo_driver",
        build_file_proto_mode = "disable_global",
        importpath = "go.mongodb.org/mongo-driver",
        sum = "h1:Ql6K6qYHEzB6xvu4+AU0BoRoqf9vFPcc4o7MUIdPW8Y=",
        version = "v1.11.3",
    )
    go_repository(
        name = "org_uber_go_atomic",
        build_file_proto_mode = "disable_global",
        importpath = "go.uber.org/atomic",
        sum = "h1:ZvwS0R+56ePWxUNi+Atn9dWONBPp/AUETXlHW0DxSjE=",
        version = "v1.11.0",
    )
    go_repository(
        name = "org_uber_go_automaxprocs",
        build_file_proto_mode = "disable_global",
        importpath = "go.uber.org/automaxprocs",
        sum = "h1:2LxUOGiR3O6tw8ui5sZa2LAaHnsviZdVOUZw4fvbnME=",
        version = "v1.5.2",
    )
    go_repository(
        name = "org_uber_go_goleak",
        build_file_proto_mode = "disable_global",
        importpath = "go.uber.org/goleak",
        sum = "h1:NBol2c7O1ZokfZ0LEU9K6Whx/KnwvepVetCUhtKja4A=",
        version = "v1.2.1",
    )
    go_repository(
        name = "org_uber_go_multierr",
        build_file_proto_mode = "disable_global",
        importpath = "go.uber.org/multierr",
        sum = "h1:blXXJkSxSSfBVBlC76pxqeO+LN3aDfLQo+309xJstO0=",
        version = "v1.11.0",
    )
    go_repository(
        name = "org_uber_go_ratelimit",
        build_file_proto_mode = "disable_global",
        importpath = "go.uber.org/ratelimit",
        sum = "h1:UQE2Bgi7p2B85uP5dC2bbRtig0C+OeNRnNEafLjsLPA=",
        version = "v0.2.0",
    )
    go_repository(
        name = "org_uber_go_zap",
        build_file_proto_mode = "disable_global",
        importpath = "go.uber.org/zap",
        sum = "h1:sI7k6L95XOKS281NhVKOFCUNIvv9e0w4BF8N3u+tCRo=",
        version = "v1.26.0",
    )
    go_repository(
        name = "tools_gotest",
        build_file_proto_mode = "disable_global",
        importpath = "gotest.tools",
        sum = "h1:VsBPFP1AI068pPrMxtb/S8Zkgf9xEmTLJjfM+P5UIEo=",
        version = "v2.2.0+incompatible",
    )
    go_repository(
        name = "tools_gotest_v3",
        build_file_proto_mode = "disable_global",
        importpath = "gotest.tools/v3",
        sum = "h1:4AuOwCGf4lLR9u3YOe2awrHygurzhO/HeQ6laiA6Sx0=",
        version = "v3.0.3",
    )
