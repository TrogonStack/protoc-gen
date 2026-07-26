defmodule Test.TestService.Service do
  @moduledoc """
  An example test service that has
  a test method. It expects a Request
  and returns a Reply.
  """

  use GRPC.Service, name: "test.TestService", protoc_gen_elixir_version: "0.17.0"

  rpc :test, Test.Request, Test.Reply
end

defmodule Test.TestService.Stub do
  use GRPC.Stub, service: Test.TestService.Service
end
