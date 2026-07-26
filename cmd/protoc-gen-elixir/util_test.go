package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/TrogonStack/protoc-gen/cmd/protoc-gen-elixir/internal/elixirpb"
)

func TestModName(t *testing.T) {
	t.Parallel()

	t.Run("plain package", func(t *testing.T) {
		t.Parallel()

		file := &descriptorpb.FileDescriptorProto{Package: proto.String("foo.bar")}
		assert.Equal(t, "Foo.Bar", ModName(file, nil, nil))
	})

	t.Run("package_prefix combines with package", func(t *testing.T) {
		t.Parallel()

		file := &descriptorpb.FileDescriptorProto{Package: proto.String("test")}
		prefix := "my"
		assert.Equal(t, "My.Test", ModName(file, &prefix, nil))
	})

	t.Run("package_prefix with no package", func(t *testing.T) {
		t.Parallel()

		file := &descriptorpb.FileDescriptorProto{}
		prefix := "my_app"
		assert.Equal(t, "MyApp", ModName(file, &prefix, nil))
	})

	t.Run("empty package and no prefix yields empty mod name", func(t *testing.T) {
		t.Parallel()

		file := &descriptorpb.FileDescriptorProto{}
		assert.Equal(t, "", ModName(file, nil, nil))
	})

	t.Run("elixirpb module_prefix overrides package_prefix and package", func(t *testing.T) {
		t.Parallel()

		opts := &descriptorpb.FileOptions{}
		proto.SetExtension(opts, elixirpb.E_File, &elixirpb.FileOptions{
			ModulePrefix: proto.String("custom_prefix"),
		})
		file := &descriptorpb.FileDescriptorProto{
			Package: proto.String("test"),
			Options: opts,
		}
		prefix := "my"
		assert.Equal(t, "CustomPrefix", ModName(file, &prefix, nil))
	})

	t.Run("nested names are appended and camelized", func(t *testing.T) {
		t.Parallel()

		file := &descriptorpb.FileDescriptorProto{Package: proto.String("test")}
		assert.Equal(t, "Test.Hat_Type", ModName(file, nil, []string{"Hat_Type"}))
	})
}

func TestCamelizeEach(t *testing.T) {
	t.Parallel()

	t.Run("underscore splitting never crosses a dot boundary", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "FooBar.AbCd", CamelizeEach("foo_bar.ab_cd"))
	})

	t.Run("empty string stays empty", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "", CamelizeEach(""))
	})

	t.Run("already camelized segment is a no-op", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "HatType", CamelizeEach("HatType"))
	})
}

func TestValidateProtoName(t *testing.T) {
	t.Parallel()

	t.Run("valid names", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"foo", "_foo", "Foo123"} {
			assert.NoError(t, ValidateProtoName(name), name)
		}
	})

	t.Run("invalid names", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"123foo", "foo-bar", ""} {
			err := ValidateProtoName(name)
			require.Error(t, err, name)
			assert.Contains(t, err.Error(), "invalid name")
		}
	})
}

func TestRenderUseProtobuf(t *testing.T) {
	t.Parallel()

	t.Run("single line form", func(t *testing.T) {
		t.Parallel()

		got := RenderUseProtobuf(2, []Option{
			{Key: "full_name", Value: "foo.bar.Mirror"},
			{Key: "protoc_gen_elixir_version", Value: protocGenElixirVersion},
			{Key: "syntax", Value: Atom("proto3")},
		})
		want := `  use Protobuf, full_name: "foo.bar.Mirror", protoc_gen_elixir_version: "0.17.0", syntax: :proto3`
		assert.Equal(t, want, got)
	})

	t.Run("wrapped form", func(t *testing.T) {
		t.Parallel()

		got := RenderUseProtobuf(2, []Option{
			{Key: "enum", Value: true},
			{Key: "full_name", Value: "test.HatType"},
			{Key: "protoc_gen_elixir_version", Value: protocGenElixirVersion},
			{Key: "syntax", Value: Atom("proto2")},
		})
		want := "  use Protobuf,\n" +
			"    enum: true,\n" +
			"    full_name: \"test.HatType\",\n" +
			"    protoc_gen_elixir_version: \"0.17.0\",\n" +
			"    syntax: :proto2"
		assert.Equal(t, want, got)
	})
}

func TestExtractDocComment(t *testing.T) {
	t.Parallel()

	t.Run("no matching location returns empty string", func(t *testing.T) {
		t.Parallel()

		sci := &descriptorpb.SourceCodeInfo{}
		assert.Equal(t, "", ExtractDocComment(sci, []int32{5, 0}))
	})

	t.Run("leading, trailing, and detached comments combine with blank line separators", func(t *testing.T) {
		t.Parallel()

		sci := &descriptorpb.SourceCodeInfo{
			Location: []*descriptorpb.SourceCodeInfo_Location{
				{
					Path:                    []int32{5, 0},
					LeadingDetachedComments: []string{" detached one", " detached two"},
					LeadingComments:         proto.String(" leading comment\n"),
					TrailingComments:        proto.String(" trailing comment\n"),
				},
			},
		}

		// LeadingComments/TrailingComments end in "\n" here (as real
		// protoc-emitted comment strings do), so joining them with the
		// "\n\n" separator produces a 3-newline run that the collapse step
		// reduces back to a single blank line between those two parts.
		got := ExtractDocComment(sci, []int32{5, 0})
		want := "detached one\n\ndetached two\n\nleading comment\ntrailing comment"
		assert.Equal(t, want, got)
	})

	t.Run("collapses runs of 3+ newlines", func(t *testing.T) {
		t.Parallel()

		sci := &descriptorpb.SourceCodeInfo{
			Location: []*descriptorpb.SourceCodeInfo_Location{
				{
					Path:            []int32{5, 0},
					LeadingComments: proto.String("line one\n\n\n\nline two"),
				},
			},
		}

		assert.Equal(t, "line one\nline two", ExtractDocComment(sci, []int32{5, 0}))
	})

	t.Run("strips common leading whitespace and trims", func(t *testing.T) {
		t.Parallel()

		sci := &descriptorpb.SourceCodeInfo{
			Location: []*descriptorpb.SourceCodeInfo_Location{
				{
					Path:            []int32{5, 0},
					LeadingComments: proto.String("  line one\n  line two\n"),
				},
			},
		}

		assert.Equal(t, "line one\nline two", ExtractDocComment(sci, []int32{5, 0}))
	})

	t.Run("matches Request message comment from test.proto", func(t *testing.T) {
		t.Parallel()

		sci := &descriptorpb.SourceCodeInfo{
			Location: []*descriptorpb.SourceCodeInfo_Location{
				{
					Path: []int32{4, 0},
					LeadingComments: proto.String(
						" This is a message that might be sent somewhere.\n" +
							"\n" +
							" Here is another line for a documentation example. This comment\n" +
							" also contains an indented example:\n" +
							"\n" +
							"     message MyMessage {\n" +
							"       Request myField = 1;\n" +
							"     }\n",
					),
				},
			},
		}

		got := ExtractDocComment(sci, []int32{4, 0})
		want := "This is a message that might be sent somewhere.\n\n" +
			"Here is another line for a documentation example. This comment\n" +
			"also contains an indented example:\n\n" +
			"    message MyMessage {\n" +
			"      Request myField = 1;\n" +
			"    }"
		assert.Equal(t, want, got)
	})
}
