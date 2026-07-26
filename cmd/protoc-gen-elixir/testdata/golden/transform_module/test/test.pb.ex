defmodule Test.HatType do
  @moduledoc false

  use Protobuf,
    enum: true,
    full_name: "test.HatType",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :FEDORA, 1
  field :FEZ, 2
end

defmodule Test.Days do
  @moduledoc false

  use Protobuf,
    enum: true,
    full_name: "test.Days",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :MONDAY, 1
  field :TUESDAY, 2
  field :LUNDI, 1
end

defmodule Test.MapEnum do
  @moduledoc false

  use Protobuf,
    enum: true,
    full_name: "test.MapEnum",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :HELLO, 0
  field :WORLD, 2
end

defmodule Test.Request.Color do
  @moduledoc false

  use Protobuf,
    enum: true,
    full_name: "test.Request.Color",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :RED, 0
  field :GREEN, 1
  field :BLUE, 2
end

defmodule Test.Reply.Entry.Game do
  @moduledoc false

  use Protobuf,
    enum: true,
    full_name: "test.Reply.Entry.Game",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :FOOTBALL, 1
  field :TENNIS, 2
end

defmodule Test.Request.SomeGroup do
  @moduledoc false

  use Protobuf,
    full_name: "test.Request.SomeGroup",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :group_field, 9, optional: true, type: :int32, json_name: "groupField"

  def transform_module(), do: My.App.Transform
end

defmodule Test.Request.NameMappingEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.Request.NameMappingEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :int32
  field :value, 2, optional: true, type: :string

  def transform_module(), do: My.App.Transform
end

defmodule Test.Request.MsgMappingEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.Request.MsgMappingEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :sint64
  field :value, 2, optional: true, type: Test.Reply

  def transform_module(), do: My.App.Transform
end

defmodule Test.Request do
  @moduledoc false

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

  def transform_module(), do: My.App.Transform
end

defmodule Test.Reply.Entry do
  @moduledoc false

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

  def transform_module(), do: My.App.Transform
end

defmodule Test.Reply do
  @moduledoc false

  use Protobuf, full_name: "test.Reply", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :found, 1, repeated: true, type: Test.Reply.Entry

  field :compact_keys, 2,
    repeated: true,
    type: :int32,
    json_name: "compactKeys",
    packed: true,
    deprecated: false

  def transform_module(), do: My.App.Transform

  extensions [{100, Protobuf.Extension.max()}]
end

defmodule Test.OtherBase do
  @moduledoc false

  use Protobuf, full_name: "test.OtherBase", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :name, 1, optional: true, type: :string

  def transform_module(), do: My.App.Transform

  extensions [{100, 111}, {199, 200}]
end

defmodule Test.ReplyExtensions do
  @moduledoc false

  use Protobuf,
    full_name: "test.ReplyExtensions",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  def transform_module(), do: My.App.Transform
end

defmodule Test.OtherReplyExtensions do
  @moduledoc false

  use Protobuf,
    full_name: "test.OtherReplyExtensions",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :int32

  def transform_module(), do: My.App.Transform
end

defmodule Test.OldReply do
  @moduledoc false

  use Protobuf, full_name: "test.OldReply", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  def transform_module(), do: My.App.Transform

  extensions [{100, 2_147_483_647}]
end

defmodule Test.Communique.SomeGroup do
  @moduledoc false

  use Protobuf,
    full_name: "test.Communique.SomeGroup",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :member, 15, optional: true, type: :string

  def transform_module(), do: My.App.Transform
end

defmodule Test.Communique.Delta do
  @moduledoc false

  use Protobuf,
    full_name: "test.Communique.Delta",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  def transform_module(), do: My.App.Transform
end

defmodule Test.Communique do
  @moduledoc false

  use Protobuf, full_name: "test.Communique", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  oneof :union, 0

  field :make_me_cry, 1, optional: true, type: :bool, json_name: "makeMeCry"
  field :number, 5, optional: true, type: :int32, oneof: 0
  field :name, 6, optional: true, type: :string, oneof: 0
  field :data, 7, optional: true, type: :bytes, oneof: 0
  field :temp_c, 8, optional: true, type: :double, json_name: "tempC", oneof: 0
  field :height, 9, optional: true, type: :float, oneof: 0
  field :today, 10, optional: true, type: Test.Days, enum: true, oneof: 0
  field :maybe, 11, optional: true, type: :bool, oneof: 0
  field :delta, 12, optional: true, type: :sint32, oneof: 0
  field :msg, 13, optional: true, type: Test.Reply, oneof: 0
  field :somegroup, 14, optional: true, type: :group, oneof: 0

  def transform_module(), do: My.App.Transform
end

defmodule Test.Options do
  @moduledoc false

  use Protobuf, full_name: "test.Options", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :opt1, 1, optional: true, type: :string, deprecated: true

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.Int32MapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.Int32MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :int32

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.Sint32MapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.Sint32MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :sint32

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.Sfixed32MapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.Sfixed32MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :sfixed32

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.Fixed32MapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.Fixed32MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :fixed32

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.Uint32MapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.Uint32MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :uint32

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.Int64MapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.Int64MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :int64

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.Sint64MapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.Sint64MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :sint64

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.Sfixed64MapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.Sfixed64MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :sfixed64

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.Fixed64MapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.Fixed64MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :fixed64

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.Uint64MapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.Uint64MapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :uint64

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.FloatMapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.FloatMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :float

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.DoubleMapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.DoubleMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :double

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.StringMapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.StringMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :string

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.BoolMapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.BoolMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :bool

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.BytesMapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.BytesMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: :bytes

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput.EnumMapEntry do
  @moduledoc false

  use Protobuf,
    full_name: "test.MapInput.EnumMapEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :string
  field :value, 2, optional: true, type: Test.MapEnum, enum: true

  def transform_module(), do: My.App.Transform
end

defmodule Test.MapInput do
  @moduledoc false

  use Protobuf, full_name: "test.MapInput", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :int32_map, 1,
    repeated: true,
    type: Test.MapInput.Int32MapEntry,
    json_name: "int32Map",
    map: true

  field :sint32_map, 2,
    repeated: true,
    type: Test.MapInput.Sint32MapEntry,
    json_name: "sint32Map",
    map: true

  field :sfixed32_map, 3,
    repeated: true,
    type: Test.MapInput.Sfixed32MapEntry,
    json_name: "sfixed32Map",
    map: true

  field :fixed32_map, 4,
    repeated: true,
    type: Test.MapInput.Fixed32MapEntry,
    json_name: "fixed32Map",
    map: true

  field :uint32_map, 5,
    repeated: true,
    type: Test.MapInput.Uint32MapEntry,
    json_name: "uint32Map",
    map: true

  field :int64_map, 6,
    repeated: true,
    type: Test.MapInput.Int64MapEntry,
    json_name: "int64Map",
    map: true

  field :sint64_map, 7,
    repeated: true,
    type: Test.MapInput.Sint64MapEntry,
    json_name: "sint64Map",
    map: true

  field :sfixed64_map, 8,
    repeated: true,
    type: Test.MapInput.Sfixed64MapEntry,
    json_name: "sfixed64Map",
    map: true

  field :fixed64_map, 9,
    repeated: true,
    type: Test.MapInput.Fixed64MapEntry,
    json_name: "fixed64Map",
    map: true

  field :uint64_map, 10,
    repeated: true,
    type: Test.MapInput.Uint64MapEntry,
    json_name: "uint64Map",
    map: true

  field :float_map, 11,
    repeated: true,
    type: Test.MapInput.FloatMapEntry,
    json_name: "floatMap",
    map: true

  field :double_map, 12,
    repeated: true,
    type: Test.MapInput.DoubleMapEntry,
    json_name: "doubleMap",
    map: true

  field :string_map, 13,
    repeated: true,
    type: Test.MapInput.StringMapEntry,
    json_name: "stringMap",
    map: true

  field :bool_map, 14,
    repeated: true,
    type: Test.MapInput.BoolMapEntry,
    json_name: "boolMap",
    map: true

  field :bytes_map, 15,
    repeated: true,
    type: Test.MapInput.BytesMapEntry,
    json_name: "bytesMap",
    map: true

  field :enum_map, 16,
    repeated: true,
    type: Test.MapInput.EnumMapEntry,
    json_name: "enumMap",
    map: true

  def transform_module(), do: My.App.Transform
end

defmodule Test.ReplyExtensions.PbExtension do
  @moduledoc false

  use Protobuf, protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  extend Test.Reply, :time, 101, optional: true, type: :double

  extend Test.Reply, :carrot, 105, optional: true, type: Test.ReplyExtensions

  extend Test.OtherBase, :donut, 101, optional: true, type: Test.ReplyExtensions
end
