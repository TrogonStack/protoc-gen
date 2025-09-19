# protoc-gen-connect-go-servicestruct

A protobuf compiler plugin that generates Go service structs with handler
functions for Connect RPC services. **Requires `protoc-gen-connect-go` as a dependency.**

## How-tos

### Installation

```bash
go install github.com/TrogonStack/protoc-gen/cmd/protoc-gen-elixir-tesla@latest
```

### Basic Usage

**Required**: This plugin must be used alongside `protoc-gen-connect-go` as it generates structs that implement the
Connect handler interfaces:

```bash
protoc --elixir_out=gen --elixir-tesla_out=gen path/to/service.proto
```

### With buf

**Required**: Add to your `buf.gen.yaml` alongside the required `buf.build/connectrpc/go` plugin:

```yaml
version: v2
plugins:
  - local: protoc-gen-elixir-tesla
    out: gen
    opt: 
      - paths=source_relative
```
