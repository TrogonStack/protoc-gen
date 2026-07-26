defmodule Test.MapEnum do
  use Protobuf,
    enum: true,
    full_name: "test.MapEnum",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :HELLO, 0
  field :WORLD, 2
end
