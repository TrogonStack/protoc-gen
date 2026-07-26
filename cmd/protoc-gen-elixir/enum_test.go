package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func enumValue(name string, number int32) *descriptorpb.EnumValueDescriptorProto {
	return &descriptorpb.EnumValueDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
	}
}

func TestRenderEnum(t *testing.T) {
	t.Parallel()

	t.Run("HatType with doc comment", func(t *testing.T) {
		t.Parallel()

		enum := &descriptorpb.EnumDescriptorProto{
			Name: proto.String("HatType"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				enumValue("FEDORA", 1),
				enumValue("FEZ", 2),
			},
		}

		got, err := RenderEnum(enum, "My.Test.HatType", "test.HatType", "proto2", true,
			"This enum represents different kinds of hats.", false)
		require.NoError(t, err)

		want := `defmodule My.Test.HatType do
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
end`
		assert.Equal(t, want, got)
	})

	t.Run("Days with doc comment and duplicate numbers", func(t *testing.T) {
		t.Parallel()

		enum := &descriptorpb.EnumDescriptorProto{
			Name: proto.String("Days"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				enumValue("MONDAY", 1),
				enumValue("TUESDAY", 2),
				enumValue("LUNDI", 1),
			},
		}

		got, err := RenderEnum(enum, "My.Test.Days", "test.Days", "proto2", true,
			"This enum represents days of the week.", false)
		require.NoError(t, err)

		want := `defmodule My.Test.Days do
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
end`
		assert.Equal(t, want, got)
	})

	t.Run("MapEnum with include_docs true but no source comment omits moduledoc", func(t *testing.T) {
		t.Parallel()

		enum := &descriptorpb.EnumDescriptorProto{
			Name: proto.String("MapEnum"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				enumValue("HELLO", 0),
				enumValue("WORLD", 2),
			},
		}

		got, err := RenderEnum(enum, "My.Test.MapEnum", "test.MapEnum", "proto2", true, "", false)
		require.NoError(t, err)

		want := `defmodule My.Test.MapEnum do
  use Protobuf,
    enum: true,
    full_name: "test.MapEnum",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :HELLO, 0
  field :WORLD, 2
end`
		assert.Equal(t, want, got)
	})

	t.Run("Unit proto3 with trailing-comment moduledoc", func(t *testing.T) {
		t.Parallel()

		enum := &descriptorpb.EnumDescriptorProto{
			Name: proto.String("Unit"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				enumValue("VOID", 0),
			},
		}

		got, err := RenderEnum(enum, "Foo.Bar.Unit", "foo.bar.Unit", "proto3", true, "foo.bar.Unit", false)
		require.NoError(t, err)

		want := `defmodule Foo.Bar.Unit do
  @moduledoc """
  foo.bar.Unit
  """

  use Protobuf,
    enum: true,
    full_name: "foo.bar.Unit",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto3

  field :VOID, 0
end`
		assert.Equal(t, want, got)
	})

	t.Run("include_docs false always emits @moduledoc false regardless of comment", func(t *testing.T) {
		t.Parallel()

		enum := &descriptorpb.EnumDescriptorProto{
			Name: proto.String("HatType"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				enumValue("FEDORA", 1),
			},
		}

		got, err := RenderEnum(enum, "My.Test.HatType", "test.HatType", "proto2", false,
			"This enum represents different kinds of hats.", false)
		require.NoError(t, err)

		want := `defmodule My.Test.HatType do
  @moduledoc false

  use Protobuf,
    enum: true,
    full_name: "test.HatType",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :FEDORA, 1
end`
		assert.Equal(t, want, got)
	})

	t.Run("empty syntax defaults to proto2", func(t *testing.T) {
		t.Parallel()

		enum := &descriptorpb.EnumDescriptorProto{
			Name: proto.String("HatType"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				enumValue("FEDORA", 1),
			},
		}

		got, err := RenderEnum(enum, "My.Test.HatType", "test.HatType", "", false, "", false)
		require.NoError(t, err)
		assert.Contains(t, got, "syntax: :proto2")
	})

	t.Run("invalid enum name surfaces an error instead of panicking", func(t *testing.T) {
		t.Parallel()

		enum := &descriptorpb.EnumDescriptorProto{
			Name: proto.String("123Bad"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				enumValue("FEDORA", 1),
			},
		}

		_, err := RenderEnum(enum, "My.Test.Bad", "test.Bad", "proto2", false, "", false)
		require.Error(t, err)
	})

	t.Run("invalid enum value name surfaces an error instead of panicking", func(t *testing.T) {
		t.Parallel()

		enum := &descriptorpb.EnumDescriptorProto{
			Name: proto.String("HatType"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				enumValue("bad-name", 1),
			},
		}

		_, err := RenderEnum(enum, "My.Test.HatType", "test.HatType", "proto2", false, "", false)
		require.Error(t, err)
	})
}
