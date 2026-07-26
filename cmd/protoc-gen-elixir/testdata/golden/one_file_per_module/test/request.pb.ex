defmodule Test.Request do
  @moduledoc """
  This is a message that might be sent somewhere.

  Here is another line for a documentation example. This comment
  also contains an indented example:

      message MyMessage {
        Request myField = 1;
      }
  """

  use Protobuf, full_name: "test.Request", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :key, 1, repeated: true, type: :int64
  field :hue, 3, optional: true, type: Test.Request.Color, enum: true
  field :hat, 4, optional: true, type: Test.HatType, default: :FEDORA, enum: true
  field :deadline, 7, optional: true, type: :float, default: "inf"
  field :somegroup, 8, optional: true, type: :group

  field :name_mapping, 14,
    repeated: true,
    type: Test.Request.NameMappingEntry,
    json_name: "nameMapping",
    map: true

  field :msg_mapping, 15,
    repeated: true,
    type: Test.Request.MsgMappingEntry,
    json_name: "msgMapping",
    map: true

  field :reset, 12, optional: true, type: :int32
  field :get_key, 16, optional: true, type: :string, json_name: "getKey"
end
