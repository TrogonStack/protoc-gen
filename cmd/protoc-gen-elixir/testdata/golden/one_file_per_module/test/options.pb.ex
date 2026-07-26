defmodule Test.Options do
  use Protobuf, full_name: "test.Options", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :opt1, 1, optional: true, type: :string, deprecated: true
end
