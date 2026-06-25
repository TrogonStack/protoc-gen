# protoc-gen-elixir-grpc

A protobuf compiler plugin that generates Elixir gRPC server modules with `defdelegate` patterns 
for clean handler organization. **Requires `protoc-gen-elixir` as a dependency.**

## How-tos

### Installation

```bash
go install github.com/TrogonStack/protoc-gen/cmd/protoc-gen-elixir-grpc@latest
```

### Basic Usage

**Required**: This plugin must be used alongside `protoc-gen-elixir` as it generates server modules that reference 
the protobuf message and service definitions:

```bash
protoc --elixir_out=lib --elixir-grpc_out=lib path/to/greeter.proto
```

### With buf

**Required**: Add to your `buf.gen.yaml` alongside the required `protoc-gen-elixir` plugin:

```yaml
version: v2
plugins:
  - local: protoc-gen-elixir  # Required dependency
    out: lib
  - local: protoc-gen-elixir-grpc
    out: lib
```

Server modules are generated into the same directory structure as the protobuf definitions, with `.server.pb.ex` suffix.

If you enable Buf's top-level `clean: true`, configure this plugin's `out` as a generated-only directory. Buf deletes each plugin `out` directory before invoking the plugin, so pointing `out` at a directory that also contains hand-written handlers will delete those files before this generator runs.

```yaml
version: v2
clean: true
plugins:
  - local: protoc-gen-elixir-grpc
    out: lib/generated_grpc
```

If generated stubs and hand-written handlers share the same output tree, keep `clean: false`.

### Configuration Options

You can configure the plugin using parameters:

#### HTTP Transcoding

Enable HTTP/JSON transcoding support for your gRPC services:

```yaml
version: v2
plugins:
  - local: protoc-gen-elixir
    out: lib
  - local: protoc-gen-elixir-grpc
    out: lib
    opt:
      - http_transcode=true
```

This generates:

```elixir
defmodule Greeter.Server do
  use GRPC.Server,
    service: Greeter.Service,
    http_transcode: true
  
  # ... method delegates
end
```

#### Custom Handler Module Prefix

Organize handlers under a custom module prefix instead of using the protobuf package:

```yaml
version: v2
plugins:
  - local: protoc-gen-elixir
    out: lib
  - local: protoc-gen-elixir-grpc
    out: lib
    opt:
      - handler_module_prefix=MyApp.Handlers
```

#### Custom Codecs

Specify custom codec modules for your gRPC server:

```yaml
version: v2
plugins:
  - local: protoc-gen-elixir
    out: lib
  - local: protoc-gen-elixir-grpc
    out: lib
    opt:
      - codecs=GRPC.Codec.Proto,GRPC.Codec.WebText,GRPC.Codec.JSON
```

This generates:

```elixir
defmodule Greeter.Server do
  use GRPC.Server,
    service: Greeter.Service,
    codecs: [GRPC.Codec.Proto, GRPC.Codec.WebText, GRPC.Codec.JSON]
  
  # ... method delegates
end
```

#### Custom Compressors

Specify custom compressor modules for your gRPC server:

```yaml
version: v2
plugins:
  - local: protoc-gen-elixir
    out: lib
  - local: protoc-gen-elixir-grpc
    out: lib
    opt:
      - compressors=GRPC.Compressor.Gzip
```

This generates:

```elixir
defmodule Greeter.Server do
  use GRPC.Server,
    service: Greeter.Service,
    compressors: [GRPC.Compressor.Gzip]
  
  # ... method delegates
end
```

#### Combining Options

You can combine multiple options:

```yaml
version: v2
plugins:
  - local: protoc-gen-elixir
    out: lib
  - local: protoc-gen-elixir-grpc
    out: lib
    opt:
      - http_transcode=true
      - handler_module_prefix=MyApp.Handlers
      - codecs=GRPC.Codec.Proto,GRPC.Codec.WebText,GRPC.Codec.JSON
      - compressors=GRPC.Compressor.Gzip
```

### Basic Service Implementation

For more realistic applications that require dependencies like database connections, implement handlers 
with proper dependency injection:

```elixir
# lib/helloworld/greeter/server/say_hello_handler.ex
defmodule Helloworld.Greeter.Server.SayHelloHandler do
  def handle_message(request, _stream) do
    # Your business logic with dependencies
    case MyApp.Users.get_user_by_name(request.name) do
      {:ok, user} -> 
        reply = %Helloworld.HelloReply{message: "Hello #{user.display_name}!"}
        {:ok, reply}
      {:error, :not_found} ->
        raise GRPC.RPCError, status: :not_found, message: "User not found"
    end
  end
end

# lib/helloworld/greeter/server/say_goodbye_handler.ex
defmodule Helloworld.Greeter.Server.SayGoodbyeHandler do  
  def handle_message(request, _stream) do
    case MyApp.Users.log_goodbye(request.name) do
      :ok ->
        reply = %Helloworld.GoodbyeReply{message: "Goodbye #{request.name}!"}
        {:ok, reply}
      {:error, reason} ->
        raise GRPC.RPCError, status: :internal, message: "Failed to log goodbye: #{reason}"
    end
  end
end
```

## Explanations

### Overview

This plugin generates gRPC server modules that provide a convenient way to organize handler functions using 
Elixir's `defdelegate` pattern. **This plugin requires `protoc-gen-elixir` to function** - it generates 
server modules that reference the protobuf message and service definitions created by the standard Elixir 
protobuf plugin.

### Features

- **Handler Delegation**: Uses `defdelegate` to separate transport concerns from business logic
- **Clean Organization**: Each RPC method gets its own dedicated handler module  
- **Streaming Support**: Handles all gRPC method types automatically
- **Package Structure**: Maintains protobuf package hierarchy in generated modules
