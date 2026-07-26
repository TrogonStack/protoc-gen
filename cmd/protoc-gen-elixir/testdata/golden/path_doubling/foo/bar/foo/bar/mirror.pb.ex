defmodule Foo.Bar.Mirror do
  @moduledoc false

  use Protobuf, full_name: "foo.bar.Mirror", protoc_gen_elixir_version: "0.17.0", syntax: :proto3

  field :name, 1, type: :string
end
