# CDKTF resources

Resources are composable units of CDKTF resources.
A set of resources composes a CDKTF [stack](../stack/README.md).

Each resource package must declare the following interface:

```go
import (
  "github.com/aws/constructs-go/constructs/v10"

  "github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resourceid"
)

type Output struct {}

type Config struct {}

func New(scope constructs.Construct, id resourceid.ID, config Config) (*Output, error) {
  // ...
}
```

In each resource, apply the following conventions to all CDKTF resources created:

- Use IDs _prefixed_ with the resource's `id` using `(resourceid.ID).TerraformID(...)`, to avoid collisions. Within each scope/[stack](../stack/README.md), IDs must be unique.
- Set _display_ names to the resource's `id`, as these do not have uniqueness constraints.
Hello World
