defmodule Test.Communique.SomeGroup do
  use Protobuf,
    full_name: "test.Communique.SomeGroup",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :member, 15, optional: true, type: :string
end
