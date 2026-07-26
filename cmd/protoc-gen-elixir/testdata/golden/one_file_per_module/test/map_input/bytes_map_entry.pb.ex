defmodule Test.MapInput.BytesMapEntry do
  use Protobuf,
    full_name: "test.MapInput.BytesMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :bytes
end
