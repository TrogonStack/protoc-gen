defmodule Test.MapInput do
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
end
