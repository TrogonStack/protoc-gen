defmodule Protobuf.Protoc.ExtTest.Foo do
  use Protobuf, full_name: "ext.Foo", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :a, 1, optional: true, type: :string
end
