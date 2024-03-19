# using gRPC alongside REST for internal APIs

New internal APIs must have both gRPC and REST implementations so that we can provide a grace period for customers. The
REST implementation should use
the [canonical JSON representation of the generated protobuf structs](https://protobuf.dev/programming-guides/proto3/#json)
in the HTTP body for arguments and responses.

> NOTE: An "internal" API is one that's solely used for intra-service communication/RPCs (think `searcher` fetching an archive from `gitserver`). Internal APIs don't include things like the graphQL API that external people can use (including our web interface).

We expect only to maintain both implementations for the `5.1.X` release in June. Afterward, we'll only use the gRPC API and can delete the redundant REST implementations.

> NOTE: Even after the `5.1.X` release, we can't translate some endpoints into gRPC in the first place. Examples include endpoints used by the git protocol directly and services we have no control over that don't support gRPC (such as Postgres). See the [gRPC June 2023 milestone issue](https://github.com/sourcegraph/sourcegraph/issues/51069) for more details.

## simple example

The following example demonstrates how to implement a simple service in Go that provides both gRPC and REST APIs, using
the [canonical JSON representation of the generated Protobuf structs](https://protobuf.dev/programming-guides/proto3/#json).

**Notes**:

- The Go service
  uses [google.golang.org/protobuf/encoding/protojson](https://google.golang.org/protobuf/encoding/protojson) to Marshal
  and Unmarshal Protobuf structs to/from JSON. The standard "encoding/json" package should **not** be used here: it
  doesn't correctly operate on protobuf structs.
- In this example, the gRPC and REST implementations share a helper function that does the actual work. This is not
  strictly required, but it's a good practice to follow (especially if the service is more complex than this example).

### gRPC definition

```proto
syntax = "proto3";

package greeting;

service GreeterService {
  rpc SayHello (HelloRequest) returns (HelloReply);
}

message HelloRequest {
  string name = 1;
}

message HelloReply {
  string message = 1;
}
```

### generate the Go protobuf structs

> NOTE: Unless you're adding an entirely new service to sourcegraph/sourcegraph, you should be able to reuse
the `buf.gen.yaml` files that have already been written for you. For the purposes of this example, we'll write a new
one.

Create the following buf configuration file:

#### buf.gen.yaml

The buf configuration file generates the Go code for the Protobuf definition. This file specifies the plugins
to use and the output directory for the generated code. The generated code includes the Protobuf structs we can
reuse in gRPC and REST implementations.

```yaml
# Configuration file for https://buf.build/, which we use for Protobuf code generation.
version: v1
plugins:
  - plugin: buf.build/protocolbuffers/go:v1.29.1
    out: .
    opt:
      - paths=source_relative
  - plugin: buf.build/grpc/go:v1.3.0
    out: .
    opt:
      - paths=source_relative
```

Now, run `sg generate buf` to use the above configuration file to generate the Go code for the protobuf definition
above. That command creates the following files:

#### greeter.pb.go

```go
package greeter

type HelloRequest struct {
  // ...
  Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
}

type HelloReply struct {
  // ...
  Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
}

// ... (omitted)
```

##### greeter_grpc.pb.go

```go
package greeter

import (
  context "context"
  grpc "google.golang.org/grpc"
)

type GreeterServiceClient interface {
  SayHello(ctx context.Context, in *HelloRequest, opts ...grpc.CallOption) (*HelloReply, error)
}

// ... (omitted)
```

### go service implementation

```go
package main

import (
  "context"
  "fmt"
  "io"
  "log"
  "net"
  "net/http"

  "github.com/gorilla/mux"
  "google.golang.org/grpc"
  "google.golang.org/grpc/codes"
  "google.golang.org/grpc/status"

  "example.com/greeting"

  // 🚨🚨🚨 note the use of this package instead of "encoding/json"!
  // "encoding/json" doesn't correctly serialize protobuf structs
  "google.golang.org/protobuf/encoding/protojson"
)

type server struct {
  greeting.UnimplementedGreeterServiceServer
}

func (s *server) SayHello(ctx context.Context, in *greeting.HelloRequest) (*greeting.HelloReply, error) {
  reply, err := getReply(ctx, in.GetName())
  if err != nil {
    return nil, err
  }

  return &greeting.HelloReply{Message: reply}, nil
}

func main() {
  // Start gRPC server
  lis, err := net.Listen("tcp", ":50051")
  if err != nil {
    log.Fatalf("failed to listen: %v", err)
  }
  grpcServer := grpc.NewServer()
  greeting.RegisterGreeterServiceServer(grpcServer, &server{})
  go func() {
    if err := grpcServer.Serve(lis); err != nil {
      log.Fatalf("failed to serve: %v", err)
    }
  }()

  // Start REST server
  r := mux.NewRouter()
  r.HandleFunc("/sayhello", sayHelloREST).Methods("POST")
  http.ListenAndServe(":8080", r)
}

func sayHelloREST(w http.ResponseWriter, r *http.Request) {
  // First, grab the arguments from the request body

  body, err := io.ReadAll(r.Body)
  if err != nil {
    http.Error(w, fmt.Sprintf("reading request json body: %s", err.Error()), http.StatusInternalServerError)
  }
  defer r.Body.Close()

  var req greeting.HelloRequest
  err = protojson.Unmarshal(body, &req)
  if err != nil {
    http.Error(w, "invalid request", http.StatusBadRequest)
    return
  }

  // Next, get the reply from the shared helper function

  reply, err := getReply(r.Context(), req.GetName())
  if err != nil {
    code, message := convertGRPCErrorToHTTPStatus(err)
    http.Error(w, message, code)
    return
  }

  // Finally, prepare the response and send it

  resp := &greeting.HelloReply{Message: reply}
  jsonBytes, err := protojson.Marshal(resp)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }

  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(http.StatusOK)
  w.Write(jsonBytes)
}

// getReply is a helper function that we can reuse in both the gRPC and REST APIs
// so that we don't have to duplicate the implementation logic.
func getReply(_ context.Context, name string) (message string, err error) {
  if name == "" {
    return "", status.Error(codes.InvalidArgument, "name was not provided")
  }

  return fmt.Sprintf("Hello, %s!", name), nil
}

// convertGRPCErrorToHTTPStatus translates gRPC error codes to HTTP status codes. See
// https://chromium.googlesource.com/external/github.com/grpc/grpc/+/refs/tags/v1.21.4-pre1/doc/statuscodes.md
// for more information.
func convertGRPCErrorToHTTPStatus(err error) (httpCode int, errorText string) {
  s, ok := status.FromError(err)
  if !ok {
    return http.StatusInternalServerError, err.Error()
  }

  switch s.Code() {
  case codes.InvalidArgument:
    return http.StatusBadRequest, s.Message()
  default:
    return http.StatusInternalServerError, s.Message()
  }
}
```

As you can see, this service reuses the generated protobuf structs in both the gRPC and REST APIs.

It also extracts the core implementation logic into a shared helper function, `getReply`, that can be reused in both interfaces. This:

- reduces code duplication (reducing the chance of drift in either implementation)
- makes testing easier (we only need to test `getReply` once)
- limits the scope of what the gRPC and REST functions are doing (only deserializing the requests and serializing the responses)
