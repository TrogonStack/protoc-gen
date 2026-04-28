package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// runPlugin builds a synthetic plugin invocation around the supplied
// FileDescriptorProtos and returns the generated content for the file
// matching `wantFilename`. Goes through injectGoImportStubs so that helper
// gets exercised by the test suite.
func runPlugin(t *testing.T, files []*descriptorpb.FileDescriptorProto, wantFilename string) string {
	t.Helper()
	toGenerate := make([]string, 0, len(files))
	for _, f := range files {
		toGenerate = append(toGenerate, f.GetName())
	}
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: toGenerate,
		ProtoFile:      files,
	}
	injectGoImportStubs(req)
	plugin, err := protogen.Options{}.New(req)
	require.NoError(t, err)
	for _, f := range plugin.Files {
		if f.Generate {
			generateFile(plugin, f)
		}
	}
	resp := plugin.Response()
	require.Empty(t, resp.GetError(), "plugin reported error")
	for _, f := range resp.GetFile() {
		if f.GetName() == wantFilename {
			return f.GetContent()
		}
	}
	t.Fatalf("expected output file %q not in plugin response (got %v)", wantFilename, fileNames(resp.GetFile()))
	return ""
}

func fileNames(files []*pluginpb.CodeGeneratorResponse_File) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.GetName()
	}
	return names
}

// field is a fixture builder for FieldDescriptorProto that defaults Label
// to OPTIONAL — the singular-presence shape the v1 plugin generates code
// for. Tests that need a different label build the descriptor inline.
func field(name string, num int32, kind descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:     proto.String(name),
		Number:   proto.Int32(num),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:     kind.Enum(),
		JsonName: proto.String(name),
	}
}

// scalarMessageProto returns a FileDescriptor with one User message
// containing one int32, one string, and one bool field.
func scalarMessageProto() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("user.proto"),
		Package: proto.String("acme.v1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("User"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("id", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32),
					field("name", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING),
					field("active", 3, descriptorpb.FieldDescriptorProto_TYPE_BOOL),
				},
			},
		},
	}
}

func TestScalarMessage_ClassDecl(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{scalarMessageProto()}, "user.pony")
	assert.Contains(t, out, "class val User")
	assert.Contains(t, out, "let id: I32")
	assert.Contains(t, out, "let name: String val")
	assert.Contains(t, out, "let active: Bool")
}

func TestScalarMessage_Constructor(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{scalarMessageProto()}, "user.pony")
	assert.Contains(t, out, "new val create(")
	assert.Contains(t, out, "id': I32 = 0")
	assert.Contains(t, out, `name': String val = ""`)
	assert.Contains(t, out, "active': Bool = false")
}

func TestScalarMessage_Codec(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{scalarMessageProto()}, "user.pony")
	assert.Contains(t, out, "primitive UserCodec")
	assert.Contains(t, out, "fun decode(reader: WireReader ref): (User val | WireError)")
	assert.Contains(t, out, "fun encode(writer: WireWriter ref, msg: User val)")
}

func TestScalarMessage_DecodeDispatch(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{scalarMessageProto()}, "user.pony")
	assert.Contains(t, out, "(1, WireVarint)")
	assert.Contains(t, out, "(2, WireLenDelim)")
	assert.Contains(t, out, "(3, WireVarint)")
	assert.Contains(t, out, "Scalar.read_int32(reader)")
	assert.Contains(t, out, "reader.read_string()")
	assert.Contains(t, out, "Scalar.read_bool(reader)")
}

func TestScalarMessage_EncodePresence(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{scalarMessageProto()}, "user.pony")
	// String fields go through write_string_field (handles empty check).
	assert.Contains(t, out, "writer.write_string_field(2, msg.name)")
	// Numeric fields gate emission on != 0.
	assert.Contains(t, out, "if msg.id != 0 then")
	// Bool fields gate on the value itself.
	assert.Contains(t, out, "if msg.active then")
}

func TestEmptyMessage(t *testing.T) {
	t.Parallel()
	file := &descriptorpb.FileDescriptorProto{
		Name:   proto.String("empty.proto"),
		Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Empty")},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "empty.pony")
	assert.Contains(t, out, "class val Empty")
	assert.Contains(t, out, "new val create() => None")
	assert.Contains(t, out, "primitive EmptyCodec")
}

func TestUnsupportedShapesEmitTodo(t *testing.T) {
	t.Parallel()
	tags := field("tags", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING)
	tags.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("zoo.proto"),
		Package: proto.String("zoo"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Zoo"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("id", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32),
					tags,
				},
			},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "zoo.pony")
	assert.Contains(t, out, "let id: I32")
	assert.Contains(t, out, "TODO protoc-gen-pony: field tags")
	// Repeated fields don't appear in the constructor's supported list.
	supportedConstructorLine := strings.Contains(out, "tags':")
	assert.False(t, supportedConstructorLine, "tags should be skipped from the constructor signature")
}

func TestNestedMessageFlatNaming(t *testing.T) {
	t.Parallel()
	file := &descriptorpb.FileDescriptorProto{
		Name:   proto.String("nested.proto"),
		Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Outer"),
				NestedType: []*descriptorpb.DescriptorProto{
					{
						Name: proto.String("Inner"),
						Field: []*descriptorpb.FieldDescriptorProto{
							field("value", 1, descriptorpb.FieldDescriptorProto_TYPE_INT64),
						},
					},
				},
			},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "nested.pony")
	assert.Contains(t, out, "class val Outer")
	assert.Contains(t, out, "class val Outer_Inner")
	assert.Contains(t, out, "primitive Outer_InnerCodec")
}

func TestFileHeaderHasSourceComment(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{scalarMessageProto()}, "user.pony")
	assert.Contains(t, out, "// Generated by protoc-gen-pony. DO NOT EDIT.")
	assert.Contains(t, out, "// Source: user.proto")
}
