# protoc-gen-connect-go-servicestruct

A protobuf compiler plugin that generates Go service structs with handler
functions for Connect RPC services. **Requires `protoc-gen-connect-go` as a dependency.**

## How-tos

### Installation

```bash
go install github.com/TrogonStack/protoc-gen/cmd/protoc-gen-connect-go-servicestruct@latest
```

### Basic Usage

**Required**: This plugin must be used alongside `protoc-gen-connect-go` as it generates structs that implement the
Connect handler interfaces:

```bash
protoc --go_out=gen --connect-go_out=gen --connect-go-servicestruct_out=gen path/to/service.proto
```

### With buf

**Required**: Add to your `buf.gen.yaml` alongside the required `buf.build/connectrpc/go` plugin:

```yaml
version: v2
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt: paths=source_relative
  - remote: buf.build/connectrpc/go:v1.18.1  # Required dependency
    out: gen
    opt: paths=source_relative
  - local: protoc-gen-connect-go-servicestruct
    out: gen
    opt: 
      - paths=source_relative
```

Files are generated into `{packagename}connect` subdirectories alongside the connect plugin output.

### Basic Service Implementation

For more realistic applications that require dependencies like database connections, use constructor functions to
create handlers:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"

    "connectrpc.com/connect"
    "github.com/jackc/pgx/v5/pgxpool"
    greeterv1 "example.com/greeter/gen/greeter/v1"
    "example.com/greeter/gen/greeter/v1/greeterv1connect"
)

func main() {
    // Initialize database connection
    dbPool, err := pgxpool.New(context.Background(), "postgres://user:pass@localhost/db")
    if err != nil {
        log.Fatal("Failed to create connection pool:", err)
    }
    defer dbPool.Close()
    
    mux := http.NewServeMux()
    path, handler := greeterv1connect.NewGreeterServiceHandler(&greeterv1connect.GreeterServiceStruct{
        GreetFunc:       newGreetHandler(dbPool),
        GreetStreamFunc: newGreetStreamHandler(dbPool),
    })
    mux.Handle(path, handler)
    
    log.Println("Greeter server starting on :8080")
    http.ListenAndServe(":8080", mux)
}

func newGreetHandler(db *pgxpool.Pool) greeterv1.GreeterServiceGreetHandlerFunc {
    return func(ctx context.Context, req *connect.Request[greeterv1.GreetRequest]) (*connect.Response[greeterv1.GreetResponse], error) {
        //    ...
    }
}

func newGreetStreamHandler(db *pgxpool.Pool) greeterv1.GreeterServiceGreetStreamHandlerFunc {
    return func(ctx context.Context, stream *connect.BidiStream[greeterv1.GreetRequest, greeterv1.GreetResponse]) error {
        // ...
    }
}
```

## Explanations

### Overview

This plugin generates service structs that provide a convenient way to organize and access handler functions for
Connect RPC services. **This plugin requires `protoc-gen-connect-go` (or `buf.build/connectrpc/go`) to function** - it
generates structs that implement the Connect handler interfaces created by the standard Connect plugin.

### Features

- **Service Structs**: Generates public structs with handler function fields.
- **Type Safety**: Uses strongly-typed handler function types.
- **Interface Compatibility**: Generated structs implement the standard handler interfaces for backwards compatibility.
