defmodule Test.Reply do
  use Protobuf, full_name: "test.Reply", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :found, 1, repeated: true, type: Test.Reply.Entry

  field :compact_keys, 2,
    repeated: true,
    type: :int32,
    json_name: "compactKeys",
    packed: true,
    deprecated: false

  extensions [{100, Protobuf.Extension.max()}]
end
