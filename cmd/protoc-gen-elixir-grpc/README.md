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
        %Helloworld.HelloReply{message: "Hello #{user.display_name}!"}
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
        %Helloworld.GoodbyeReply{message: "Goodbye #{request.name}!"}
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
- **Type Specifications**: Generates `@spec` annotations for all RPC methods with proper types
- **Clean Organization**: Each RPC method gets its own dedicated handler module  
- **Streaming Support**: Handles all gRPC method types automatically
- **Package Structure**: Maintains protobuf package hierarchy in generated modules
