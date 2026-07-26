package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// extensionTypes is a TypeRegistry pre-populated with the extendee/type
// references test.proto's ReplyExtensions/top-level extend blocks exercise,
// mirroring the real cross-message resolution the golden fixtures exercise.
var extensionTypes = &TypeRegistry{
	modNames: map[string]string{
		".test.Reply":                "My.Test.Reply",
		".test.OtherBase":            "My.Test.OtherBase",
		".test.ReplyExtensions":      "My.Test.ReplyExtensions",
		".test.OtherReplyExtensions": "My.Test.OtherReplyExtensions",
	},
}

func extendField(name string, number int32, extendee string, t descriptorpb.FieldDescriptorProto_Type, typeName string) *descriptorpb.FieldDescriptorProto {
	f := &descriptorpb.FieldDescriptorProto{
		Name:     proto.String(name),
		Number:   proto.Int32(number),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:     t.Enum(),
		JsonName: proto.String(name),
		Extendee: proto.String(extendee),
	}
	if typeName != "" {
		f.TypeName = proto.String(typeName)
	}
	return f
}

// TestRenderExtensionRanges_SingleRange covers the ordinary "extensions ...
// to max;" case: End == 0x20000000 renders the Protobuf.Extension.max()
// sentinel, verified against Reply's `extensions [{100,
// Protobuf.Extension.max()}]` in
// testdata/golden/package_prefix/my/test/test.pb.ex.
func TestRenderExtensionRanges_SingleRange(t *testing.T) {
	t.Parallel()

	ranges := []*descriptorpb.DescriptorProto_ExtensionRange{
		{Start: proto.Int32(100), End: proto.Int32(extensionRangeMaxEnd)},
	}

	got := renderExtensionRanges(ranges)
	assert.Equal(t, "  extensions [{100, Protobuf.Extension.max()}]", got)
}

// TestRenderExtensionRanges_MultipleDisjointRanges covers multiple
// comma-separated tuples in declaration order, verified against OtherBase's
// `extensions [{100, 111}, {199, 200}]`.
func TestRenderExtensionRanges_MultipleDisjointRanges(t *testing.T) {
	t.Parallel()

	ranges := []*descriptorpb.DescriptorProto_ExtensionRange{
		{Start: proto.Int32(100), End: proto.Int32(111)},
		{Start: proto.Int32(199), End: proto.Int32(200)},
	}

	got := renderExtensionRanges(ranges)
	assert.Equal(t, "  extensions [{100, 111}, {199, 200}]", got)
}

// TestRenderExtensionRanges_MessageSetWireFormatLiteral covers the
// message_set_wire_format edge case: OldReply's descriptor End is
// 2147483647 (INT32_MAX), NOT the ordinary max sentinel value, so it renders
// as a raw underscore-grouped integer literal instead of
// Protobuf.Extension.max(). Hand-constructed directly with Start/End (this
// function only looks at the raw integers, not option_message_set_wire_format
// itself), verified against OldReply's `extensions [{100, 2_147_483_647}]`.
func TestRenderExtensionRanges_MessageSetWireFormatLiteral(t *testing.T) {
	t.Parallel()

	ranges := []*descriptorpb.DescriptorProto_ExtensionRange{
		{Start: proto.Int32(100), End: proto.Int32(2147483647)},
	}

	got := renderExtensionRanges(ranges)
	assert.Equal(t, "  extensions [{100, 2_147_483_647}]", got)
}

// TestRenderMessage_ExtensionRangeBodySection covers the extension-range
// body section's placement within RenderMessage's overall output: after
// fields, joined via the same bodySections blank-line mechanism, and as the
// ONLY body section when a message has no fields at all (OldReply).
func TestRenderMessage_ExtensionRangeBodySection(t *testing.T) {
	t.Parallel()

	t.Run("extensions after fields", func(t *testing.T) {
		t.Parallel()

		msg := &descriptorpb.DescriptorProto{
			Name: proto.String("OtherBase"),
			Field: []*descriptorpb.FieldDescriptorProto{
				scalarField("name", 1, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_STRING),
			},
			ExtensionRange: []*descriptorpb.DescriptorProto_ExtensionRange{
				{Start: proto.Int32(100), End: proto.Int32(111)},
				{Start: proto.Int32(199), End: proto.Int32(200)},
			},
		}

		got, err := RenderMessage(msg, "My.Test.OtherBase", "test.OtherBase", "proto2", false, "", false, "", nil, noTypes)
		require.NoError(t, err)

		want := `defmodule My.Test.OtherBase do
  @moduledoc false

  use Protobuf, full_name: "test.OtherBase", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :name, 1, optional: true, type: :string

  extensions [{100, 111}, {199, 200}]
end`
		assert.Equal(t, want, got)
	})

	t.Run("extensions as the only body section when there are no fields", func(t *testing.T) {
		t.Parallel()

		msg := &descriptorpb.DescriptorProto{
			Name: proto.String("OldReply"),
			ExtensionRange: []*descriptorpb.DescriptorProto_ExtensionRange{
				{Start: proto.Int32(100), End: proto.Int32(2147483647)},
			},
		}

		got, err := RenderMessage(msg, "My.Test.OldReply", "test.OldReply", "proto2", false, "", false, "", nil, noTypes)
		require.NoError(t, err)

		want := `defmodule My.Test.OldReply do
  @moduledoc false

  use Protobuf, full_name: "test.OldReply", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  extensions [{100, 2_147_483_647}]
end`
		assert.Equal(t, want, got)
	})
}

// TestRenderMessageExtension_NoExtendFields covers the "return false" case:
// a message with no embedded extend fields at all must not produce a
// PbExtension submodule (nil, not an empty one).
func TestRenderMessageExtension_NoExtendFields(t *testing.T) {
	t.Parallel()

	msg := &descriptorpb.DescriptorProto{Name: proto.String("Options")}

	_, ok, err := RenderMessageExtension(msg, "My.Test.Options.PbExtension", "proto2", false, noTypes)
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestRenderMessageExtension_MultiExtendeeMergeOrder covers the core
// message-embedded case: ReplyExtensions' two extend blocks (targeting Reply
// then OtherBase) merge into ONE PbExtension module, in source declaration
// order (NOT grouped/reordered by extendee) - verified byte-for-byte against
// testdata/golden/package_prefix/my/test/test.pb.ex's
// My.Test.ReplyExtensions.PbExtension. Also covers full_name: exclusion and
// syntax: inclusion (from the message's own file syntax).
func TestRenderMessageExtension_MultiExtendeeMergeOrder(t *testing.T) {
	t.Parallel()

	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("ReplyExtensions"),
		Extension: []*descriptorpb.FieldDescriptorProto{
			extendField("time", 101, ".test.Reply", descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, ""),
			extendField("carrot", 105, ".test.Reply", descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".test.ReplyExtensions"),
			extendField("donut", 101, ".test.OtherBase", descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".test.ReplyExtensions"),
		},
	}

	got, ok, err := RenderMessageExtension(msg, "My.Test.ReplyExtensions.PbExtension", "proto2", true, extensionTypes)
	require.NoError(t, err)
	require.True(t, ok)

	want := `defmodule My.Test.ReplyExtensions.PbExtension do
  use Protobuf, protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  extend My.Test.Reply, :time, 101, optional: true, type: :double

  extend My.Test.Reply, :carrot, 105, optional: true, type: My.Test.ReplyExtensions

  extend My.Test.OtherBase, :donut, 101, optional: true, type: My.Test.ReplyExtensions
end`
	assert.Equal(t, want, got)

	assert.NotContains(t, got, "full_name")
	assert.Contains(t, got, "syntax: :proto2")
}

// TestRenderFileExtension_NoFields covers the "return false" case for the
// top-level merged module: an empty field group must not produce a
// PbExtension module at all (mirrors the rule to never emit an extension
// module with zero extend lines, applied as "don't produce the
// output" rather than erroring).
func TestRenderFileExtension_NoFields(t *testing.T) {
	t.Parallel()

	_, ok, err := RenderFileExtension(nil, "My.Test.PbExtension", true, noTypes)
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestRenderFileExtension_TopLevelMerge covers the top-level, file-merged
// PbExtension module: test.proto's top-level `extend Reply { tag = 103;
// donut = 106; }` block, verified byte-for-byte against
// testdata/golden/package_prefix/my/test/pb_extension.pb.ex. Confirms BOTH
// full_name: AND syntax: exclusion (contrast with the message-embedded case,
// which keeps syntax:).
func TestRenderFileExtension_TopLevelMerge(t *testing.T) {
	t.Parallel()

	fields := []*descriptorpb.FieldDescriptorProto{
		extendField("tag", 103, ".test.Reply", descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
		extendField("donut", 106, ".test.Reply", descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".test.OtherReplyExtensions"),
	}

	got, ok, err := RenderFileExtension(fields, "My.Test.PbExtension", true, extensionTypes)
	require.NoError(t, err)
	require.True(t, ok)

	want := `defmodule My.Test.PbExtension do
  use Protobuf, protoc_gen_elixir_version: "0.17.0"

  extend My.Test.Reply, :tag, 103, optional: true, type: :string

  extend My.Test.Reply, :donut, 106, optional: true, type: My.Test.OtherReplyExtensions
end`
	assert.Equal(t, want, got)

	assert.NotContains(t, got, "full_name")
	assert.NotContains(t, got, "syntax:")
}

// TestRenderFileExtension_ModuledocConditional covers the three-way
// @moduledoc branch reduced to its always-empty-docComment behavior: this
// synthesized module never has its own single SourceCodeInfo location, so
// docComment is always "" - meaning include_docs=true always lands in the
// "no @moduledoc line at all" branch (never the real-doc-comment branch),
// while include_docs=false still emits "@moduledoc false" same as any other
// module. Verified against testdata/golden/package_prefix (include_docs=true,
// no @moduledoc line) vs. testdata/golden/grpc_proto_source and
// testdata/golden/transform_module (no include_docs, "@moduledoc false").
func TestRenderFileExtension_ModuledocConditional(t *testing.T) {
	t.Parallel()

	fields := []*descriptorpb.FieldDescriptorProto{
		extendField("tag", 103, ".test.Reply", descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
	}

	t.Run("include_docs=false emits @moduledoc false", func(t *testing.T) {
		t.Parallel()

		got, ok, err := RenderFileExtension(fields, "My.Test.PbExtension", false, extensionTypes)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Contains(t, got, "defmodule My.Test.PbExtension do\n  @moduledoc false\n\n  use Protobuf")
	})

	t.Run("include_docs=true emits no @moduledoc line at all", func(t *testing.T) {
		t.Parallel()

		got, ok, err := RenderFileExtension(fields, "My.Test.PbExtension", true, extensionTypes)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Contains(t, got, "defmodule My.Test.PbExtension do\n  use Protobuf")
		assert.NotContains(t, got, "@moduledoc")
	})
}

// TestRenderExtendLine_UnresolvableExtendee covers the error path (no
// panic) when an extend field's Extendee isn't in the registry, mirroring
// renderFieldTypeValue's and resolveRPCType's error patterns.
func TestRenderExtendLine_UnresolvableExtendee(t *testing.T) {
	t.Parallel()

	field := extendField("tag", 103, ".unknown.Foo", descriptorpb.FieldDescriptorProto_TYPE_STRING, "")

	_, err := renderExtendLine(field, extensionTypes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown.Foo")
}

// TestRenderMessageExtension_UnresolvableExtendee covers the same error path
// surfacing through RenderMessageExtension's public entry point.
func TestRenderMessageExtension_UnresolvableExtendee(t *testing.T) {
	t.Parallel()

	msg := &descriptorpb.DescriptorProto{
		Name: proto.String("Bad"),
		Extension: []*descriptorpb.FieldDescriptorProto{
			extendField("tag", 103, ".unknown.Foo", descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
		},
	}

	_, _, err := RenderMessageExtension(msg, "My.Test.Bad.PbExtension", "proto2", false, extensionTypes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown.Foo")
}
