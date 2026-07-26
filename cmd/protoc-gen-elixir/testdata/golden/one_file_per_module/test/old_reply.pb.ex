defmodule Test.OldReply do
  use Protobuf, full_name: "test.OldReply", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  extensions [{100, 2_147_483_647}]
end
