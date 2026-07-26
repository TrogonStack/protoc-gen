defmodule Test.Communique do
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
end
