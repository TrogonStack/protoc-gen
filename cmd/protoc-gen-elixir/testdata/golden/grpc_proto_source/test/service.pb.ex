defmodule Test.TestService.Service do
  @moduledoc false

  use GRPC.Service, name: "test.TestService", protoc_gen_elixir_version: "0.17.0"

  def proto_source(), do: "service.proto"

  rpc :test, Test.Request, Test.Reply
end

defmodule Test.TestService.Stub do
  @moduledoc false

  use GRPC.Stub, service: Test.TestService.Service
end
