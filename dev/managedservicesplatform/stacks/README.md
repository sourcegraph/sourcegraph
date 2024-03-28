# CDKTF stacks

A stack is a fully composed set of CDKTF [resources](../resource/README.md) that maps to a Terraform workspace.
A set of stacks composes a CDKTF application.

Each stack package must declare the following interface:

```go
import (
  "github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack"
  "github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack/options/googleprovider"
)

// CrossStackOutput allows programatic access to stack outputs across stacks.
// For human reference outputs, use (stack.ExplicitStackOutputs).Add(...)
type CrossStackOutput struct {}

type Variables struct {}

const StackName = "..."

func NewStack(stacks *stack.Set, vars Variables) (*CrossStackOutput, error) {
  stack, outputs := stacks.New(StackName,
    googleprovider.With(vars.ProjectID),
    // ... other stack-wide options
  )

  // ...
}
```
Hello World
