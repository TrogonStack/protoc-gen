defmodule Test.Request.Color do
  @moduledoc """
  This enum represents three different colors.
  """

  use Protobuf,
    enum: true,
    full_name: "test.Request.Color",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :RED, 0
  field :GREEN, 1
  field :BLUE, 2
end
