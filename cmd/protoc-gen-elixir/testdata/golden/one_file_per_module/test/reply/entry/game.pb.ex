defmodule Test.Reply.Entry.Game do
  use Protobuf,
    enum: true,
    full_name: "test.Reply.Entry.Game",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :FOOTBALL, 1
  field :TENNIS, 2
end
