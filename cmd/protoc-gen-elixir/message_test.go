package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func scalarField(name string, number int32, label descriptorpb.FieldDescriptorProto_Label, t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:     proto.String(name),
		Number:   proto.Int32(number),
		Label:    label.Enum(),
		Type:     t.Enum(),
		JsonName: proto.String(name),
	}
}

// noTypes is a TypeRegistry with nothing registered, valid for any test
// exercising only scalar (non-message, non-enum) field types, which never
// call TypeRegistry.Resolve.
var noTypes = &TypeRegistry{}

// noOneof is an OneofContext for a message with no oneof_decl entries at
// all, valid for any test not exercising oneof-index field options.
var noOneof = OneofContext{}

func TestRenderField(t *testing.T) {
	t.Parallel()

	t.Run("proto3 non-repeated omits label key", func(t *testing.T) {
		t.Parallel()

		f := scalarField("name", 1, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_STRING)
		got, err := RenderField(f, "proto3", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, "  field :name, 1, type: :string", got)
	})

	t.Run("proto3 repeated emits repeated true", func(t *testing.T) {
		t.Parallel()

		f := scalarField("names", 1, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, descriptorpb.FieldDescriptorProto_TYPE_STRING)
		got, err := RenderField(f, "proto3", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, "  field :names, 1, repeated: true, type: :string", got)
	})

	t.Run("proto2 optional emits optional true", func(t *testing.T) {
		t.Parallel()

		f := scalarField("key", 1, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_INT32)
		got, err := RenderField(f, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, "  field :key, 1, optional: true, type: :int32", got)
	})

	t.Run("proto2 required emits required true", func(t *testing.T) {
		t.Parallel()

		f := scalarField("key", 1, descriptorpb.FieldDescriptorProto_LABEL_REQUIRED, descriptorpb.FieldDescriptorProto_TYPE_INT64)
		got, err := RenderField(f, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, "  field :key, 1, required: true, type: :int64", got)
	})

	t.Run("proto2 repeated emits repeated true", func(t *testing.T) {
		t.Parallel()

		f := scalarField("key", 1, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, descriptorpb.FieldDescriptorProto_TYPE_INT64)
		got, err := RenderField(f, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, "  field :key, 1, repeated: true, type: :int64", got)
	})

	t.Run("empty syntax behaves like proto2 for label purposes", func(t *testing.T) {
		t.Parallel()

		f := scalarField("key", 1, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_INT32)
		got, err := RenderField(f, "", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, "  field :key, 1, optional: true, type: :int32", got)
	})

	t.Run("all scalar type atoms", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			t    descriptorpb.FieldDescriptorProto_Type
			atom string
		}{
			{descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, "double"},
			{descriptorpb.FieldDescriptorProto_TYPE_FLOAT, "float"},
			{descriptorpb.FieldDescriptorProto_TYPE_INT64, "int64"},
			{descriptorpb.FieldDescriptorProto_TYPE_UINT64, "uint64"},
			{descriptorpb.FieldDescriptorProto_TYPE_INT32, "int32"},
			{descriptorpb.FieldDescriptorProto_TYPE_FIXED64, "fixed64"},
			{descriptorpb.FieldDescriptorProto_TYPE_FIXED32, "fixed32"},
			{descriptorpb.FieldDescriptorProto_TYPE_BOOL, "bool"},
			{descriptorpb.FieldDescriptorProto_TYPE_STRING, "string"},
			{descriptorpb.FieldDescriptorProto_TYPE_GROUP, "group"},
			{descriptorpb.FieldDescriptorProto_TYPE_BYTES, "bytes"},
			{descriptorpb.FieldDescriptorProto_TYPE_UINT32, "uint32"},
			{descriptorpb.FieldDescriptorProto_TYPE_SFIXED32, "sfixed32"},
			{descriptorpb.FieldDescriptorProto_TYPE_SFIXED64, "sfixed64"},
			{descriptorpb.FieldDescriptorProto_TYPE_SINT32, "sint32"},
			{descriptorpb.FieldDescriptorProto_TYPE_SINT64, "sint64"},
		}

		for _, tt := range tests {
			f := scalarField("f", 1, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, tt.t)
			got, err := RenderField(f, "proto2", noTypes, noOneof)
			require.NoError(t, err)
			assert.Equal(t, "  field :f, 1, optional: true, type: :"+tt.atom, got)
		}
	})

	t.Run("json_name emitted only when it diverges from name", func(t *testing.T) {
		t.Parallel()

		same := scalarField("key", 1, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_INT32)
		got, err := RenderField(same, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.NotContains(t, got, "json_name")

		diverges := &descriptorpb.FieldDescriptorProto{
			Name:     proto.String("get_key"),
			Number:   proto.Int32(16),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			JsonName: proto.String("getKey"),
		}
		got, err = RenderField(diverges, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, `  field :get_key, 16, optional: true, type: :string, json_name: "getKey"`, got)
	})

	t.Run("packed emitted only when explicitly present in descriptor", func(t *testing.T) {
		t.Parallel()

		absent := scalarField("key", 1, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, descriptorpb.FieldDescriptorProto_TYPE_INT32)
		got, err := RenderField(absent, "proto3", noTypes, noOneof)
		require.NoError(t, err)
		assert.NotContains(t, got, "packed")

		explicitTrue := scalarField("key", 1, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, descriptorpb.FieldDescriptorProto_TYPE_INT32)
		explicitTrue.Options = &descriptorpb.FieldOptions{Packed: proto.Bool(true)}
		got, err = RenderField(explicitTrue, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Contains(t, got, "packed: true")

		explicitFalse := scalarField("key", 1, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, descriptorpb.FieldDescriptorProto_TYPE_INT32)
		explicitFalse.Options = &descriptorpb.FieldOptions{Packed: proto.Bool(false)}
		got, err = RenderField(explicitFalse, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Contains(t, got, "packed: false")
	})

	t.Run("deprecated emitted whenever FieldOptions is non-nil, omitted when options is nil entirely", func(t *testing.T) {
		t.Parallel()

		// No Options message at all: no deprecated key.
		absent := scalarField("key", 1, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_INT32)
		got, err := RenderField(absent, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.NotContains(t, got, "deprecated")

		explicitTrue := scalarField("opt1", 1, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_STRING)
		explicitTrue.Options = &descriptorpb.FieldOptions{Deprecated: proto.Bool(true)}
		got, err = RenderField(explicitTrue, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, `  field :opt1, 1, optional: true, type: :string, deprecated: true`, got)

		explicitFalse := scalarField("key", 1, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_INT32)
		explicitFalse.Options = &descriptorpb.FieldOptions{Deprecated: proto.Bool(false)}
		got, err = RenderField(explicitFalse, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Contains(t, got, "deprecated: false")

		// FieldOptions present but Deprecated itself unset (nil): still
		// emits "deprecated: false" using the effective default - unlike
		// packed, which stays gated on its own explicit presence. Evidenced
		// by testdata/golden/package_prefix/my/test/test.pb.ex's
		// Reply.compact_keys: the real descriptor only sets options.packed
		// (see testdata/proto/test.proto's `[packed = true]`), yet the
		// golden fixture still shows `deprecated: false`.
		optionsPresentDeprecatedUnset := scalarField("key", 1, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, descriptorpb.FieldDescriptorProto_TYPE_INT32)
		optionsPresentDeprecatedUnset.Options = &descriptorpb.FieldOptions{Packed: proto.Bool(true)}
		got, err = RenderField(optionsPresentDeprecatedUnset, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Contains(t, got, "deprecated: false")
	})

	t.Run("golden: Reply.compact_keys wraps across multiple lines", func(t *testing.T) {
		t.Parallel()

		f := &descriptorpb.FieldDescriptorProto{
			Name:     proto.String("compact_keys"),
			Number:   proto.Int32(2),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
			JsonName: proto.String("compactKeys"),
			Options: &descriptorpb.FieldOptions{
				Packed:     proto.Bool(true),
				Deprecated: proto.Bool(false),
			},
		}

		got, err := RenderField(f, "proto2", noTypes, noOneof)
		require.NoError(t, err)

		want := "  field :compact_keys, 2,\n" +
			"    repeated: true,\n" +
			"    type: :int32,\n" +
			"    json_name: \"compactKeys\",\n" +
			"    packed: true,\n" +
			"    deprecated: false"
		assert.Equal(t, want, got)
	})

	t.Run("golden: Request.deadline float default inf", func(t *testing.T) {
		t.Parallel()

		f := &descriptorpb.FieldDescriptorProto{
			Name:         proto.String("deadline"),
			Number:       proto.Int32(7),
			Label:        descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:         descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum(),
			JsonName:     proto.String("deadline"),
			DefaultValue: proto.String("inf"),
		}

		got, err := RenderField(f, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, `  field :deadline, 7, optional: true, type: :float, default: "inf"`, got)
	})

	t.Run("golden: Reply.Entry.value int default", func(t *testing.T) {
		t.Parallel()

		f := &descriptorpb.FieldDescriptorProto{
			Name:         proto.String("value"),
			Number:       proto.Int32(2),
			Label:        descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:         descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
			JsonName:     proto.String("value"),
			DefaultValue: proto.String("7"),
		}

		got, err := RenderField(f, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, `  field :value, 2, optional: true, type: :int64, default: 7`, got)
	})

	t.Run("bool default", func(t *testing.T) {
		t.Parallel()

		f := &descriptorpb.FieldDescriptorProto{
			Name:         proto.String("flag"),
			Number:       proto.Int32(1),
			Label:        descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:         descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
			JsonName:     proto.String("flag"),
			DefaultValue: proto.String("true"),
		}

		got, err := RenderField(f, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, `  field :flag, 1, optional: true, type: :bool, default: true`, got)
	})

	t.Run("string default is quoted verbatim", func(t *testing.T) {
		t.Parallel()

		f := &descriptorpb.FieldDescriptorProto{
			Name:         proto.String("greeting"),
			Number:       proto.Int32(1),
			Label:        descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:         descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			JsonName:     proto.String("greeting"),
			DefaultValue: proto.String(`hello "world"`),
		}

		got, err := RenderField(f, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, `  field :greeting, 1, optional: true, type: :string, default: "hello \"world\""`, got)
	})

	t.Run("empty default_value is not emitted", func(t *testing.T) {
		t.Parallel()

		f := &descriptorpb.FieldDescriptorProto{
			Name:         proto.String("key"),
			Number:       proto.Int32(1),
			Label:        descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:         descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			JsonName:     proto.String("key"),
			DefaultValue: proto.String(""),
		}

		got, err := RenderField(f, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.NotContains(t, got, "default")
	})

	t.Run("invalid field name surfaces an error instead of panicking", func(t *testing.T) {
		t.Parallel()

		f := scalarField("bad-name", 1, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_STRING)
		_, err := RenderField(f, "proto2", noTypes, noOneof)
		require.Error(t, err)
	})

	t.Run("message/enum field types with an unresolvable type name surface an error instead of panicking", func(t *testing.T) {
		t.Parallel()

		f := &descriptorpb.FieldDescriptorProto{
			Name:     proto.String("msg"),
			Number:   proto.Int32(1),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: proto.String(".test.Unknown"),
			JsonName: proto.String("msg"),
		}
		_, err := RenderField(f, "proto2", noTypes, noOneof)
		require.Error(t, err)
	})

	t.Run("golden: Request.hue enum-typed field resolves to referenced module with enum: true", func(t *testing.T) {
		t.Parallel()

		types := &TypeRegistry{modNames: map[string]string{
			".test.Request.Color": "My.Test.Request.Color",
		}}

		f := &descriptorpb.FieldDescriptorProto{
			Name:     proto.String("hue"),
			Number:   proto.Int32(3),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
			TypeName: proto.String(".test.Request.Color"),
			JsonName: proto.String("hue"),
		}

		got, err := RenderField(f, "proto2", types, noOneof)
		require.NoError(t, err)
		assert.Equal(t, "  field :hue, 3, optional: true, type: My.Test.Request.Color, enum: true", got)
	})

	t.Run("golden: Reply.found message-typed field resolves to referenced module without enum: true", func(t *testing.T) {
		t.Parallel()

		types := &TypeRegistry{modNames: map[string]string{
			".test.Reply.Entry": "My.Test.Reply.Entry",
		}}

		f := &descriptorpb.FieldDescriptorProto{
			Name:     proto.String("found"),
			Number:   proto.Int32(1),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: proto.String(".test.Reply.Entry"),
			JsonName: proto.String("found"),
		}

		got, err := RenderField(f, "proto2", types, noOneof)
		require.NoError(t, err)
		assert.Equal(t, "  field :found, 1, repeated: true, type: My.Test.Reply.Entry", got)
	})

	t.Run("group field renders bare :group atom even though it has a TypeName", func(t *testing.T) {
		t.Parallel()

		f := &descriptorpb.FieldDescriptorProto{
			Name:     proto.String("somegroup"),
			Number:   proto.Int32(8),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_GROUP.Enum(),
			TypeName: proto.String(".test.Request.SomeGroup"),
			JsonName: proto.String("somegroup"),
		}

		got, err := RenderField(f, "proto2", noTypes, noOneof)
		require.NoError(t, err)
		assert.Equal(t, "  field :somegroup, 8, optional: true, type: :group", got)
	})

	t.Run("map field emits map: true trailing option after resolving to the entry module", func(t *testing.T) {
		t.Parallel()

		types := &TypeRegistry{
			modNames: map[string]string{
				".test.Request.NameMappingEntry": "My.Test.Request.NameMappingEntry",
			},
			mapEntries: map[string]bool{
				".test.Request.NameMappingEntry": true,
			},
		}

		f := &descriptorpb.FieldDescriptorProto{
			Name:     proto.String("name_mapping"),
			Number:   proto.Int32(14),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: proto.String(".test.Request.NameMappingEntry"),
			JsonName: proto.String("nameMapping"),
		}

		got, err := RenderField(f, "proto2", types, noOneof)
		require.NoError(t, err)
		want := "  field :name_mapping, 14,\n" +
			"    repeated: true,\n" +
			"    type: My.Test.Request.NameMappingEntry,\n" +
			"    json_name: \"nameMapping\",\n" +
			"    map: true"
		assert.Equal(t, want, got)
	})

	t.Run("real oneof emits trailing oneof index field-option", func(t *testing.T) {
		t.Parallel()

		oneofCtx := NewOneofContext([]*descriptorpb.OneofDescriptorProto{{Name: proto.String("union")}})

		f := &descriptorpb.FieldDescriptorProto{
			Name:       proto.String("number"),
			Number:     proto.Int32(5),
			Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:       descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
			JsonName:   proto.String("number"),
			OneofIndex: proto.Int32(0),
		}

		got, err := RenderField(f, "proto2", noTypes, oneofCtx)
		require.NoError(t, err)
		assert.Equal(t, "  field :number, 5, optional: true, type: :int32, oneof: 0", got)
	})

	t.Run("synthetic (proto3-optional) oneof does not emit oneof field-option", func(t *testing.T) {
		t.Parallel()

		oneofCtx := NewOneofContext([]*descriptorpb.OneofDescriptorProto{{Name: proto.String("_name")}})

		f := &descriptorpb.FieldDescriptorProto{
			Name:           proto.String("name"),
			Number:         proto.Int32(1),
			Label:          descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:           descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			JsonName:       proto.String("name"),
			OneofIndex:     proto.Int32(0),
			Proto3Optional: proto.Bool(true),
		}

		got, err := RenderField(f, "proto3", noTypes, oneofCtx)
		require.NoError(t, err)
		assert.Equal(t, "  field :name, 1, proto3_optional: true, type: :string", got)
		assert.NotContains(t, got, "oneof")
	})

	t.Run("proto3_optional emits no label key at all, even for a real (non-synthetic-named) oneof_index", func(t *testing.T) {
		t.Parallel()

		// Spec-derived (from the field definitions / proto3 optional
		// prose), not golden-verified: no fixture in this repo's corpus
		// exercises an explicit proto3 `optional` scalar field (test.proto
		// is proto2; no_package.proto/full_name.proto's proto3 messages have
		// no bare `optional` field). Implemented strictly per that spec: emit
		// "proto3_optional: true, type: ...", no label key, no oneof key -
		// regardless of whether oneof_decl[index]'s name happens to start
		// with "_" (it always does in real protoc output, but RenderField
		// suppresses the oneof key purely from Proto3Optional, not from
		// re-deriving synthetic-ness of the oneof name).
		oneofCtx := NewOneofContext([]*descriptorpb.OneofDescriptorProto{{Name: proto.String("_bar")}})

		f := &descriptorpb.FieldDescriptorProto{
			Name:           proto.String("bar"),
			Number:         proto.Int32(1),
			Label:          descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:           descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
			JsonName:       proto.String("bar"),
			OneofIndex:     proto.Int32(0),
			Proto3Optional: proto.Bool(true),
		}

		got, err := RenderField(f, "proto3", noTypes, oneofCtx)
		require.NoError(t, err)
		assert.Equal(t, "  field :bar, 1, proto3_optional: true, type: :int32", got)
	})
}

func TestRenderMessage(t *testing.T) {
	t.Parallel()

	t.Run("golden: Options (proto2, single field, field-level deprecated true)", func(t *testing.T) {
		t.Parallel()

		msg := &descriptorpb.DescriptorProto{
			Name: proto.String("Options"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:     proto.String("opt1"),
					Number:   proto.Int32(1),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					JsonName: proto.String("opt1"),
					Options:  &descriptorpb.FieldOptions{Deprecated: proto.Bool(true)},
				},
			},
		}

		got, err := RenderMessage(msg, "My.Test.Options", "test.Options", "proto2", false, "", false, "", nil, noTypes)
		require.NoError(t, err)

		want := `defmodule My.Test.Options do
  @moduledoc false

  use Protobuf, full_name: "test.Options", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :opt1, 1, optional: true, type: :string, deprecated: true
end`
		assert.Equal(t, want, got)
	})

	t.Run("golden: OtherReplyExtensions (proto2, single plain field)", func(t *testing.T) {
		t.Parallel()

		msg := &descriptorpb.DescriptorProto{
			Name: proto.String("OtherReplyExtensions"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:     proto.String("key"),
					Number:   proto.Int32(1),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					JsonName: proto.String("key"),
				},
			},
		}

		got, err := RenderMessage(msg, "My.Test.OtherReplyExtensions", "test.OtherReplyExtensions", "proto2", false, "", false, "", nil, noTypes)
		require.NoError(t, err)

		want := `defmodule My.Test.OtherReplyExtensions do
  @moduledoc false

  use Protobuf,
    full_name: "test.OtherReplyExtensions",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :int32
end`
		assert.Equal(t, want, got)
	})

	t.Run("zero fields: no field lines, no extra blank line", func(t *testing.T) {
		t.Parallel()

		msg := &descriptorpb.DescriptorProto{
			Name: proto.String("MessageWithCustomOptions"),
		}

		got, err := RenderMessage(msg, "Test.MessageWithCustomOptions", "test.MessageWithCustomOptions", "proto3", false, "", false, "", nil, noTypes)
		require.NoError(t, err)

		want := `defmodule Test.MessageWithCustomOptions do
  @moduledoc false

  use Protobuf,
    full_name: "test.MessageWithCustomOptions",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto3
end`
		assert.Equal(t, want, got)
	})

	t.Run("include_docs true with doc comment", func(t *testing.T) {
		t.Parallel()

		msg := &descriptorpb.DescriptorProto{
			Name: proto.String("Foo"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:     proto.String("a"),
					Number:   proto.Int32(1),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					JsonName: proto.String("a"),
				},
			},
		}

		got, err := RenderMessage(msg, "Protobuf.Protoc.ExtTest.Foo", "ext.Foo", "proto2", true, "", false, "", nil, noTypes)
		require.NoError(t, err)

		want := `defmodule Protobuf.Protoc.ExtTest.Foo do
  use Protobuf, full_name: "ext.Foo", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :a, 1, optional: true, type: :string
end`
		assert.Equal(t, want, got)
	})

	t.Run("message-level deprecated true is emitted as use-option", func(t *testing.T) {
		t.Parallel()

		msg := &descriptorpb.DescriptorProto{
			Name:    proto.String("Old"),
			Options: &descriptorpb.MessageOptions{Deprecated: proto.Bool(true)},
		}

		got, err := RenderMessage(msg, "My.Old", "my.Old", "proto3", false, "", false, "", nil, noTypes)
		require.NoError(t, err)
		assert.Contains(t, got, "deprecated: true")
	})

	t.Run("message-level deprecated false is not emitted", func(t *testing.T) {
		t.Parallel()

		msg := &descriptorpb.DescriptorProto{
			Name:    proto.String("New"),
			Options: &descriptorpb.MessageOptions{Deprecated: proto.Bool(false)},
		}

		got, err := RenderMessage(msg, "My.New", "my.New", "proto3", false, "", false, "", nil, noTypes)
		require.NoError(t, err)
		assert.NotContains(t, got, "deprecated")
	})

	t.Run("gen_proto_source emits proto_source use-option sorted alphabetically", func(t *testing.T) {
		t.Parallel()

		msg := &descriptorpb.DescriptorProto{Name: proto.String("Message")}

		got, err := RenderMessage(msg, "Foo.Bar.Message", "foo.bar.Message", "proto3", false, "", false, "full_name.proto", nil, noTypes)
		require.NoError(t, err)

		want := `defmodule Foo.Bar.Message do
  @moduledoc false

  use Protobuf,
    full_name: "foo.bar.Message",
    proto_source: "full_name.proto",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto3
end`
		assert.Equal(t, want, got)
	})

	t.Run("empty syntax defaults to proto2", func(t *testing.T) {
		t.Parallel()

		msg := &descriptorpb.DescriptorProto{Name: proto.String("Foo")}

		got, err := RenderMessage(msg, "My.Foo", "my.Foo", "", false, "", false, "", nil, noTypes)
		require.NoError(t, err)
		assert.Contains(t, got, "syntax: :proto2")
	})

	t.Run("invalid message name surfaces an error instead of panicking", func(t *testing.T) {
		t.Parallel()

		msg := &descriptorpb.DescriptorProto{Name: proto.String("123Bad")}
		_, err := RenderMessage(msg, "My.Bad", "my.Bad", "proto2", false, "", false, "", nil, noTypes)
		require.Error(t, err)
	})
}

// IsRenderable (Phase 3's extension-based exclusion predicate) was removed
// in Phase 5: extension ranges and message-embedded extend blocks are now
// both rendered (see message.go's renderExtensionRanges and
// extension.go's RenderMessageExtension), so every message renders
// unconditionally - there's no remaining gate to test here. See
// extension_test.go for extension-range and PbExtension rendering coverage.
