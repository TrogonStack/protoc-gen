defmodule Test.ReplyExtensions.PbExtension do
  use Protobuf, protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  extend Test.Reply, :time, 101, optional: true, type: :double

  extend Test.Reply, :carrot, 105, optional: true, type: Test.ReplyExtensions

  extend Test.OtherBase, :donut, 101, optional: true, type: Test.ReplyExtensions
end
