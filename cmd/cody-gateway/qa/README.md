# Basic E2E / smoke test suite for Cody Gateway

To run against local instance ($TOKEN is a `sgd_` token):

```
bazel test  --test_output=all //cmd/cody-gateway/qa:qa_test --test_env=E2E_GATEWAY_ENDPOINT=http://localhost:9992 --test_env=E2E_GATEWAY_TOKEN=$TOKEN
```

To run against dev instance using dotcom user ($TOKEN is a `sgd_` token):

```
bazel test  --test_output=all //cmd/cody-gateway/qa:qa_test --test_env=E2E_GATEWAY_ENDPOINT=https://cody-gateway.sgdev.org --test_env=E2E_GATEWAY_TOKEN=$TOKEN
```

To run against prod using a dotcom user ($TOKEN is a `sgd_` token):

```
bazel test  --test_output=all //cmd/cody-gateway/qa:qa_test --test_env=E2E_GATEWAY_ENDPOINT=https://cody-gateway.sourcegraph.com --test_env=E2E_GATEWAY_TOKEN=$TOKEN
```
Hello World
