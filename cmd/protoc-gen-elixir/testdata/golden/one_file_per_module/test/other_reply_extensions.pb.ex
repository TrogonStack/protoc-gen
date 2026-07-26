defmodule Test.OtherReplyExtensions do
  use Protobuf,
    full_name: "test.OtherReplyExtensions",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :int32
end
