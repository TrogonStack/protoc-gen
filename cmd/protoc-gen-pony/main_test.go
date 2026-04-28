package main

import (
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

	// Synthetic oneofs (proto3 `optional`) must come AFTER real oneofs in
	// OneofDecl, so the real "kind" oneof is declared first at index 0.
	typeA := field("type_a", 4, descriptorpb.FieldDescriptorProto_TYPE_STRING)
	typeA.OneofIndex = proto.Int32(0)
	typeB := field("type_b", 5, descriptorpb.FieldDescriptorProto_TYPE_INT32)
	typeB.OneofIndex = proto.Int32(0)

	// proto3 explicit `optional` — synthesized into a single-field oneof
	// at OneofIndex 1.
	optCount := field("count", 3, descriptorpb.FieldDescriptorProto_TYPE_INT32)
	optCount.Proto3Optional = proto.Bool(true)
	optCount.OneofIndex = proto.Int32(1)

	parent := field("parent", 6, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	parent.TypeName = proto.String(".zoo.Parent")

	status := field("status", 7, descriptorpb.FieldDescriptorProto_TYPE_ENUM)
	status.TypeName = proto.String(".zoo.Status")

	// map<string, int32> — modeled in descriptors as a repeated MESSAGE
	// field pointing at a synthetic nested MapEntry type with
	// MessageOptions.map_entry=true.
	metadata := field("metadata", 8, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	metadata.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	metadata.TypeName = proto.String(".zoo.Zoo.MetadataEntry")
	mapEntry := &descriptorpb.DescriptorProto{
		Name: proto.String("MetadataEntry"),
		Field: []*descriptorpb.FieldDescriptorProto{
			field("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING),
			field("value", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32),
		},
		Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
	}

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("zoo.proto"),
		Package: proto.String("zoo"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Zoo"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("id", 1, descriptorpb.FieldDescriptorProto_TYPE_INT32),
					tags, optCount, typeA, typeB, parent, status, metadata,
				},
				NestedType: []*descriptorpb.DescriptorProto{mapEntry},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{
					{Name: proto.String("kind")},
					{Name: proto.String("_count")},
				},
			},
			{Name: proto.String("Parent")},
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("Status"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("UNKNOWN"), Number: proto.Int32(0)},
					{Name: proto.String("ACTIVE"), Number: proto.Int32(1)},
				},
			},
		},
	}

	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "zoo.pony")

	assert.Contains(t, out, "let id: I32")

	unsupported := []string{"tags", "count", "type_a", "type_b", "parent", "status", "metadata"}
	for _, name := range unsupported {
		assert.Contains(t, out, "TODO protoc-gen-pony: field "+name,
			"missing TODO for %q", name)
		assert.NotContains(t, out, name+"': ",
			"%q should be skipped from the constructor signature", name)
	}
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

// "User-provided params still win" (main.go:50) relies on existing entries
// appearing verbatim after the injected stubs — protogen's later-wins
// semantics depend on it. Lock in: nothing dropped or reordered, including
// empty values and duplicates.
func TestInjectGoImportStubs_PreservesExistingParameters(t *testing.T) {
	t.Parallel()
	const existing = "foo=1,bar=,foo=2"
	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile: []*descriptorpb.FileDescriptorProto{
			{Name: proto.String("user.proto")},
			{Name: proto.String("admin.proto")},
		},
		Parameter: proto.String(existing),
	}
	injectGoImportStubs(req)
	assert.Equal(t,
		"Muser.proto=protoc-gen-pony/stub,Madmin.proto=protoc-gen-pony/stub,"+existing,
		req.GetParameter())
}
