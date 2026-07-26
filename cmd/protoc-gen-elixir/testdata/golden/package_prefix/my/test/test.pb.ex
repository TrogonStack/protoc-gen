defmodule My.Test.HatType do
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

defmodule My.Test.Days do
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

defmodule My.Test.MapEnum do
  use Protobuf,
    enum: true,
    full_name: "test.MapEnum",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :HELLO, 0
  field :WORLD, 2
end

defmodule My.Test.Request.Color do
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

defmodule My.Test.Reply.Entry.Game do
  use Protobuf,
    enum: true,
    full_name: "test.Reply.Entry.Game",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :FOOTBALL, 1
  field :TENNIS, 2
end

defmodule My.Test.Request.SomeGroup do
  use Protobuf,
    full_name: "test.Request.SomeGroup",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :group_field, 9, optional: true, type: :int32, json_name: "groupField"
end

defmodule My.Test.Request.NameMappingEntry do
  use Protobuf,
    full_name: "test.Request.NameMappingEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :int32
  field :value, 2, optional: true, type: :string
end

defmodule My.Test.Request.MsgMappingEntry do
  use Protobuf,
    full_name: "test.Request.MsgMappingEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :sint64
  field :value, 2, optional: true, type: My.Test.Reply
end

defmodule My.Test.Request do
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
  field :hue, 3, optional: true, type: My.Test.Request.Color, enum: true
  field :hat, 4, optional: true, type: My.Test.HatType, default: :FEDORA, enum: true
  field :deadline, 7, optional: true, type: :float, default: "inf"
  field :somegroup, 8, optional: true, type: :group

  field :name_mapping, 14,
    repeated: true,
    type: My.Test.Request.NameMappingEntry,
    json_name: "nameMapping",
    map: true

  field :msg_mapping, 15,
    repeated: true,
    type: My.Test.Request.MsgMappingEntry,
    json_name: "msgMapping",
    map: true

  field :reset, 12, optional: true, type: :int32
  field :get_key, 16, optional: true, type: :string, json_name: "getKey"
end

defmodule My.Test.Reply.Entry do
  use Protobuf,
    full_name: "test.Reply.Entry",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key_that_needs_1234camel_CasIng, 1,
    required: true,
    type: :int64,
    json_name: "keyThatNeeds1234camelCasIng"

  field :value, 2, optional: true, type: :int64, default: 7
  field :_my_field_name_2, 3, optional: true, type: :int64, json_name: "MyFieldName2"
end

defmodule My.Test.Reply do
  use Protobuf, full_name: "test.Reply", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :found, 1, repeated: true, type: My.Test.Reply.Entry

  field :compact_keys, 2,
    repeated: true,
    type: :int32,
    json_name: "compactKeys",
    packed: true,
    deprecated: false

  extensions [{100, Protobuf.Extension.max()}]
end

defmodule My.Test.OtherBase do
  use Protobuf, full_name: "test.OtherBase", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :name, 1, optional: true, type: :string

  extensions [{100, 111}, {199, 200}]
end

defmodule My.Test.ReplyExtensions do
  use Protobuf,
    full_name: "test.ReplyExtensions",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2
end

defmodule My.Test.OtherReplyExtensions do
  use Protobuf,
    full_name: "test.OtherReplyExtensions",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :int32
end

defmodule My.Test.OldReply do
  use Protobuf, full_name: "test.OldReply", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  extensions [{100, 2_147_483_647}]
end

defmodule My.Test.Communique.SomeGroup do
  use Protobuf,
    full_name: "test.Communique.SomeGroup",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :member, 15, optional: true, type: :string
end

defmodule My.Test.Communique.Delta do
  use Protobuf,
    full_name: "test.Communique.Delta",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2
end

defmodule My.Test.Communique do
  use Protobuf, full_name: "test.Communique", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  oneof :union, 0

  field :make_me_cry, 1, optional: true, type: :bool, json_name: "makeMeCry"
  field :number, 5, optional: true, type: :int32, oneof: 0
  field :name, 6, optional: true, type: :string, oneof: 0
  field :data, 7, optional: true, type: :bytes, oneof: 0
  field :temp_c, 8, optional: true, type: :double, json_name: "tempC", oneof: 0
  field :height, 9, optional: true, type: :float, oneof: 0
  field :today, 10, optional: true, type: My.Test.Days, enum: true, oneof: 0
  field :maybe, 11, optional: true, type: :bool, oneof: 0
  field :delta, 12, optional: true, type: :sint32, oneof: 0
  field :msg, 13, optional: true, type: My.Test.Reply, oneof: 0
  field :somegroup, 14, optional: true, type: :group, oneof: 0
end

defmodule My.Test.Options do
  use Protobuf, full_name: "test.Options", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :opt1, 1, optional: true, type: :string, deprecated: true
end

defmodule My.Test.MapInput.Int32MapEntry do
  use Protobuf,
    full_name: "test.MapInput.Int32MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :int32
end

defmodule My.Test.MapInput.Sint32MapEntry do
  use Protobuf,
    full_name: "test.MapInput.Sint32MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :sint32
end

defmodule My.Test.MapInput.Sfixed32MapEntry do
  use Protobuf,
    full_name: "test.MapInput.Sfixed32MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :sfixed32
end

defmodule My.Test.MapInput.Fixed32MapEntry do
  use Protobuf,
    full_name: "test.MapInput.Fixed32MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :fixed32
end

defmodule My.Test.MapInput.Uint32MapEntry do
  use Protobuf,
    full_name: "test.MapInput.Uint32MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :uint32
end

defmodule My.Test.MapInput.Int64MapEntry do
  use Protobuf,
    full_name: "test.MapInput.Int64MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :int64
end

defmodule My.Test.MapInput.Sint64MapEntry do
  use Protobuf,
    full_name: "test.MapInput.Sint64MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :sint64
end

defmodule My.Test.MapInput.Sfixed64MapEntry do
  use Protobuf,
    full_name: "test.MapInput.Sfixed64MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :sfixed64
end

defmodule My.Test.MapInput.Fixed64MapEntry do
  use Protobuf,
    full_name: "test.MapInput.Fixed64MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :fixed64
end

defmodule My.Test.MapInput.Uint64MapEntry do
  use Protobuf,
    full_name: "test.MapInput.Uint64MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :uint64
end

defmodule My.Test.MapInput.FloatMapEntry do
  use Protobuf,
    full_name: "test.MapInput.FloatMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :float
end

defmodule My.Test.MapInput.DoubleMapEntry do
  use Protobuf,
    full_name: "test.MapInput.DoubleMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :double
end

defmodule My.Test.MapInput.StringMapEntry do
  use Protobuf,
    full_name: "test.MapInput.StringMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :string
end

defmodule My.Test.MapInput.BoolMapEntry do
  use Protobuf,
    full_name: "test.MapInput.BoolMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :bool
end

defmodule My.Test.MapInput.BytesMapEntry do
  use Protobuf,
    full_name: "test.MapInput.BytesMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :bytes
end

defmodule My.Test.MapInput.EnumMapEntry do
  use Protobuf,
    full_name: "test.MapInput.EnumMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: My.Test.MapEnum, enum: true
end

defmodule My.Test.MapInput do
  use Protobuf, full_name: "test.MapInput", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :int32_map, 1,
    repeated: true,
    type: My.Test.MapInput.Int32MapEntry,
    json_name: "int32Map",
    map: true

  field :sint32_map, 2,
    repeated: true,
    type: My.Test.MapInput.Sint32MapEntry,
    json_name: "sint32Map",
    map: true

  field :sfixed32_map, 3,
    repeated: true,
    type: My.Test.MapInput.Sfixed32MapEntry,
    json_name: "sfixed32Map",
    map: true

  field :fixed32_map, 4,
    repeated: true,
    type: My.Test.MapInput.Fixed32MapEntry,
    json_name: "fixed32Map",
    map: true

  field :uint32_map, 5,
    repeated: true,
    type: My.Test.MapInput.Uint32MapEntry,
    json_name: "uint32Map",
    map: true

  field :int64_map, 6,
    repeated: true,
    type: My.Test.MapInput.Int64MapEntry,
    json_name: "int64Map",
    map: true

  field :sint64_map, 7,
    repeated: true,
    type: My.Test.MapInput.Sint64MapEntry,
    json_name: "sint64Map",
    map: true

  field :sfixed64_map, 8,
    repeated: true,
    type: My.Test.MapInput.Sfixed64MapEntry,
    json_name: "sfixed64Map",
    map: true

  field :fixed64_map, 9,
    repeated: true,
    type: My.Test.MapInput.Fixed64MapEntry,
    json_name: "fixed64Map",
    map: true

  field :uint64_map, 10,
    repeated: true,
    type: My.Test.MapInput.Uint64MapEntry,
    json_name: "uint64Map",
    map: true

  field :float_map, 11,
    repeated: true,
    type: My.Test.MapInput.FloatMapEntry,
    json_name: "floatMap",
    map: true

  field :double_map, 12,
    repeated: true,
    type: My.Test.MapInput.DoubleMapEntry,
    json_name: "doubleMap",
    map: true

  field :string_map, 13,
    repeated: true,
    type: My.Test.MapInput.StringMapEntry,
    json_name: "stringMap",
    map: true

  field :bool_map, 14,
    repeated: true,
    type: My.Test.MapInput.BoolMapEntry,
    json_name: "boolMap",
    map: true

  field :bytes_map, 15,
    repeated: true,
    type: My.Test.MapInput.BytesMapEntry,
    json_name: "bytesMap",
    map: true

  field :enum_map, 16,
    repeated: true,
    type: My.Test.MapInput.EnumMapEntry,
    json_name: "enumMap",
    map: true
end

defmodule My.Test.ReplyExtensions.PbExtension do
  use Protobuf, protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  extend My.Test.Reply, :time, 101, optional: true, type: :double

  extend My.Test.Reply, :carrot, 105, optional: true, type: My.Test.ReplyExtensions

  extend My.Test.OtherBase, :donut, 101, optional: true, type: My.Test.ReplyExtensions
end
