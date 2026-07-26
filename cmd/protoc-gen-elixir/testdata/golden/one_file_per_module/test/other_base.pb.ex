defmodule Test.OtherBase do
  use Protobuf, full_name: "test.OtherBase", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :name, 1, optional: true, type: :string

  extensions [{100, 111}, {199, 200}]
end
