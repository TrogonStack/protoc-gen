defmodule Test.Reply.Entry do
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
