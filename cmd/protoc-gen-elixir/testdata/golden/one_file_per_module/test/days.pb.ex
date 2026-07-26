defmodule Test.Days do
  @moduledoc """
  This enum represents days of the week.
  """

  use Protobuf,
    enum: true,
    full_name: "test.Days",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :MONDAY, 1
  field :TUESDAY, 2
  field :LUNDI, 1
end
