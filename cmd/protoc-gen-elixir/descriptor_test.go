package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestTermStructSingleLine(t *testing.T) {
	t.Parallel()

	term := termStruct{
		TypeName: "Google.Protobuf.Foo",
		Fields: []field{
			{"a", intTerm(1)},
			{"b", boolTerm(true)},
		},
	}

	got := term.render(0)
	assert.Equal(t, `%Google.Protobuf.Foo{a: 1, b: true}`, got)
}

func TestTermStructWrapsWhenTooWide(t *testing.T) {
	t.Parallel()

	// Force a wrap by using a long type name and several fields, pushing the
	// single-line rendering past descriptorLineThreshold.
	term := termStruct{
		TypeName: "Google.Protobuf.SomeVeryLongDescriptorTypeNameForTesting",
		Fields: []field{
			{"first_field_name", stringTerm("some reasonably long value")},
			{"second_field_name", stringTerm("another reasonably long value")},
			{"third_field_name", boolTerm(false)},
		},
	}

	got := term.render(0)
	want := `%Google.Protobuf.SomeVeryLongDescriptorTypeNameForTesting{
  first_field_name: "some reasonably long value",
  second_field_name: "another reasonably long value",
  third_field_name: false
}`
	assert.Equal(t, want, got)
}

func TestTermStructNestedIndentation(t *testing.T) {
	t.Parallel()

	inner := termStruct{
		TypeName: "Google.Protobuf.SomeVeryLongInnerDescriptorTypeNameForTesting",
		Fields: []field{
			{"first_field_name", stringTerm("some reasonably long value")},
			{"second_field_name", stringTerm("another reasonably long value")},
		},
	}
	outer := termStruct{
		TypeName: "Google.Protobuf.SomeVeryLongOuterDescriptorTypeNameForTesting",
		Fields: []field{
			{"name", stringTerm("x")},
			{"inner", inner},
		},
	}

	got := outer.render(0)
	want := `%Google.Protobuf.SomeVeryLongOuterDescriptorTypeNameForTesting{
  name: "x",
  inner: %Google.Protobuf.SomeVeryLongInnerDescriptorTypeNameForTesting{
    first_field_name: "some reasonably long value",
    second_field_name: "another reasonably long value"
  }
}`
	assert.Equal(t, want, got)
}

func TestTermListEmptyRendersBrackets(t *testing.T) {
	t.Parallel()

	got := termList{}.render(0)
	assert.Equal(t, "[]", got)
}

func TestTermListWrapsNestedStructs(t *testing.T) {
	t.Parallel()

	list := termList{Elements: []elixirTerm{
		termStruct{
			TypeName: "Google.Protobuf.SomeVeryLongElementDescriptorTypeNameForTesting",
			Fields: []field{
				{"first_field_name", stringTerm("some reasonably long value")},
				{"second_field_name", stringTerm("another reasonably long value")},
			},
		},
	}}

	got := list.render(2)
	want := `[
    %Google.Protobuf.SomeVeryLongElementDescriptorTypeNameForTesting{
      first_field_name: "some reasonably long value",
      second_field_name: "another reasonably long value"
    }
  ]`
	assert.Equal(t, want, got)
}

func TestDecodeUnknownFieldsStringBranch(t *testing.T) {
	t.Parallel()

	// tag: field 50005, wire type 2 (length-delimited) -> (50005 << 3) | 2 = 400042
	// varint(400042) = 0xAA 0xB5 0x18 (little-endian base-128 groups)
	raw := []byte{170, 181, 24, 5, 'h', 'e', 'l', 'l', 'o'}

	entries := decodeUnknownFields(raw)
	require.Len(t, entries, 1)
	assert.Equal(t, int32(50005), entries[0].Number)
	assert.Equal(t, int32(2), entries[0].WireType)
	assert.Equal(t, `"hello"`, entries[0].Value.render(0))
}

func TestDecodeUnknownFieldsMessageWithCustomOptionsBytes(t *testing.T) {
	t.Parallel()

	// Empirically captured raw unknown bytes for MessageWithCustomOptions'
	// MessageOptions (field 51300, wire type 2, "message_with_custom_options").
	raw := []byte{162, 134, 25, 27, 109, 101, 115, 115, 97, 103, 101, 95, 119, 105, 116, 104, 95, 99, 117, 115, 116, 111, 109, 95, 111, 112, 116, 105, 111, 110, 115}

	entries := decodeUnknownFields(raw)
	require.Len(t, entries, 1)
	assert.Equal(t, int32(51300), entries[0].Number)
	assert.Equal(t, int32(2), entries[0].WireType)
	assert.Equal(t, `"message_with_custom_options"`, entries[0].Value.render(0))
}

func TestDecodeUnknownFieldsEmpty(t *testing.T) {
	t.Parallel()

	entries := decodeUnknownFields(nil)
	assert.Empty(t, entries)
}

func TestDecodeUnknownFieldsNonPrintableBinary(t *testing.T) {
	t.Parallel()

	// tag: field 1, wire type 2 -> (1 << 3) | 2 = 10; length 2; raw bytes 0xff 0x00
	raw := []byte{10, 2, 0xff, 0x00}

	entries := decodeUnknownFields(raw)
	require.Len(t, entries, 1)
	assert.Equal(t, int32(1), entries[0].Number)
	assert.Equal(t, int32(2), entries[0].WireType)
	assert.Equal(t, "<<255, 0>>", entries[0].Value.render(0))
}

func TestDecodeUnknownFieldsVarint(t *testing.T) {
	t.Parallel()

	// tag: field 2, wire type 0 (varint) -> (2 << 3) | 0 = 16; value 150 (varint: 0x96 0x01)
	raw := []byte{16, 0x96, 0x01}

	entries := decodeUnknownFields(raw)
	require.Len(t, entries, 1)
	assert.Equal(t, int32(2), entries[0].Number)
	assert.Equal(t, int32(0), entries[0].WireType)
	assert.Equal(t, "150", entries[0].Value.render(0))
}

// TestGenerateDescriptorsIntegration builds a FileDescriptorSet from
// testdata/proto/custom_options.proto, runs the plugin with
// gen_descriptors=true,include_docs=true, and asserts BYTE-IDENTICAL output
// against both testdata/golden/gen_descriptors/test/custom_options.pb.ex and
// testdata/golden/gen_descriptors/test/pb_extension.pb.ex (the latter
// unaffected by gen_descriptors, included as a regression check that
// extension.go's rendering path is untouched by this feature).
func TestGenerateDescriptorsIntegration(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSet(t, "custom_options.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"custom_options.proto"},
		Parameter:      proto.String("gen_descriptors=true,include_docs=true"),
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())

	file, ok := findGeneratedFile(resp.GetFile(), "test/custom_options.pb.ex")
	require.True(t, ok, "expected test/custom_options.pb.ex among generated files")

	want, err := os.ReadFile(filepath.Join("testdata", "golden", "gen_descriptors", "test", "custom_options.pb.ex"))
	require.NoError(t, err)
	assert.Equal(t, string(want), file.GetContent())

	extFile, ok := findGeneratedFile(resp.GetFile(), "test/pb_extension.pb.ex")
	require.True(t, ok, "expected test/pb_extension.pb.ex among generated files")

	wantExt, err := os.ReadFile(filepath.Join("testdata", "golden", "gen_descriptors", "test", "pb_extension.pb.ex"))
	require.NoError(t, err)
	assert.Equal(t, string(wantExt), extFile.GetContent())
}
