defmodule Test.Request.SomeGroup do
  use Protobuf,
    full_name: "test.Request.SomeGroup",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :group_field, 9, optional: true, type: :int32, json_name: "groupField"
end
