defmodule Test.Request.MsgMappingEntry do
  use Protobuf,
    full_name: "test.Request.MsgMappingEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :sint64
  field :value, 2, optional: true, type: Test.Reply
end
