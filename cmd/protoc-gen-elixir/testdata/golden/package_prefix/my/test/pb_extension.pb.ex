defmodule My.Test.PbExtension do
  use Protobuf, protoc_gen_elixir_version: "0.17.0"

  extend My.Test.Reply, :tag, 103, optional: true, type: :string

  extend My.Test.Reply, :donut, 106, optional: true, type: My.Test.OtherReplyExtensions
end
