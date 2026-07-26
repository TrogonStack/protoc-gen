defmodule Test.HatType do
  @moduledoc """
  This enum represents different kinds of hats.
  """

  use Protobuf,
    enum: true,
    full_name: "test.HatType",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :FEDORA, 1
  field :FEZ, 2
end
