defmodule Test.PbExtension do
  use Protobuf, protoc_gen_elixir_version: "0.17.0"

  extend Google.Protobuf.EnumValueOptions, :my_custom_option, 50005,
    optional: true,
    type: :string,
    json_name: "myCustomOption"

  extend Google.Protobuf.MessageOptions, :lowercase_name, 51300,
    optional: true,
    type: :string,
    json_name: "lowercaseName"
end
