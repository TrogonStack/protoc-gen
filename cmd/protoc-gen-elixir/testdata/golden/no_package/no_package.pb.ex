defmodule NoPackageMessage.NumberMappingEntry do
  use Protobuf,
    full_name: "NoPackageMessage.NumberMappingEntry",
    map: true,
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto3

  field :key, 1, type: :uint32
  field :value, 2, type: :uint32
end

defmodule NoPackageMessage do
  use Protobuf,
    full_name: "NoPackageMessage",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto3

  field :number_mapping, 1,
    repeated: true,
    type: NoPackageMessage.NumberMappingEntry,
    json_name: "numberMapping",
    map: true
end
