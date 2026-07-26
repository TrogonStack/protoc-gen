defmodule Foo.Bar.Unit do
  @moduledoc """
  foo.bar.Unit
  """

  use Protobuf,
    enum: true,
    full_name: "foo.bar.Unit",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto3

  field :VOID, 0
end

defmodule Foo.Bar.Message.NestedMessage.Kind do
  @moduledoc """
  foo.bar.Message.NestedMessage.Kind
  """

  use Protobuf,
    enum: true,
    full_name: "foo.bar.Message.NestedMessage.Kind",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto3

  field :NULL, 0
  field :PRIMARY, 1
  field :SECONDARY, 2
end

defmodule Foo.Bar.Message.NestedMessage do
  @moduledoc """
  foo.bar.Message.NestedMessage
  """

  use Protobuf,
    full_name: "foo.bar.Message.NestedMessage",
    proto_source: "full_name.proto",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto3

  field :kind, 1, type: Foo.Bar.Message.NestedMessage.Kind, enum: true
end

defmodule Foo.Bar.Message do
  @moduledoc """
  foo.bar.Message
  """

  use Protobuf,
    full_name: "foo.bar.Message",
    proto_source: "full_name.proto",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto3

  oneof :id, 0

  field :name, 1, type: :string, oneof: 0
  field :num, 2, type: :uint64, oneof: 0
  field :extra, 3, type: Foo.Bar.Message.NestedMessage
end

defmodule Foo.Bar.Message.NestedMessage.PbExtension do
  use Protobuf, protoc_gen_elixir_version: "0.17.0", syntax: :proto3

  extend Google.Protobuf.MessageOptions, :fizz, 49999, optional: true, type: :string
end
