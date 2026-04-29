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

// zooFileProto is a shared fixture for tests that need same-file message,
// enum, repeated, and unsupported shapes together.
func zooFileProto() *descriptorpb.FileDescriptorProto {
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

	return &descriptorpb.FileDescriptorProto{
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
}

func TestUnsupportedShapesEmitTodo(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{zooFileProto()}, "zoo.pony")

	assert.Contains(t, out, "let id: I32")

	// These shapes are now generated — confirm they are NOT TODO comments.
	assert.Contains(t, out, "let tags: Array[String val] val")
	assert.Contains(t, out, "let parent: (Parent val | None)")
	assert.Contains(t, out, "let status: Status")

	// proto3 optional int32 is now supported.
	assert.Contains(t, out, "let count: (I32 | None)")

	// These shapes remain unsupported — confirm TODO comments, not constructor params.
	stillUnsupported := []string{"type_a", "type_b", "metadata"}
	for _, name := range stillUnsupported {
		assert.Contains(t, out, "TODO protoc-gen-pony: field "+name,
			"missing TODO for %q", name)
		assert.NotContains(t, out, name+"': ",
			"%q should be skipped from the constructor signature", name)
	}
}

func TestEnumGeneration(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{zooFileProto()}, "zoo.pony")

	// Primitives for each enum value.
	assert.Contains(t, out, "primitive Unknown   fun value(): I32 => 0")
	assert.Contains(t, out, "primitive Active   fun value(): I32 => 1")

	// Raw class val preserves unknown numeric values (proto3 forward-compat).
	assert.Contains(t, out, "class val StatusRaw")
	assert.Contains(t, out, "fun value(): I32 => _v")

	// Type alias union includes the Raw fallback.
	assert.Contains(t, out, "type Status is (Unknown | Active | StatusRaw)")

	// FromValue: all known values explicit, unknowns go to Raw(v).
	assert.Contains(t, out, "primitive StatusFromValue")
	assert.Contains(t, out, "| 0 => Unknown")
	assert.Contains(t, out, "| 1 => Active")
	assert.Contains(t, out, "else StatusRaw(v)")
}

func TestEnumField_ClassAndCodec(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{zooFileProto()}, "zoo.pony")

	// Class declaration and constructor default.
	assert.Contains(t, out, "let status: Status")
	assert.Contains(t, out, "status': Status = Unknown")

	// Decode: reads an I32 and applies FromValue.
	assert.Contains(t, out, "status = StatusFromValue(v)")

	// Encode: skips zero value.
	assert.Contains(t, out, "if msg.status.value() != 0 then")
	assert.Contains(t, out, "Scalar.write_int32(writer, msg.status.value())")
}

func TestMessageField_ClassAndCodec(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{zooFileProto()}, "zoo.pony")

	// Class declaration and constructor default.
	assert.Contains(t, out, "let parent: (Parent val | None)")
	assert.Contains(t, out, "parent': (Parent val | None) = None")

	// Decode: reads len-delim bytes, hands to sub-codec.
	assert.Contains(t, out, "match ParentCodec.decode(WireReader(b))")
	assert.Contains(t, out, "| let v: Parent val => parent = v")

	// Encode: match on None, emit sub-writer only when present.
	assert.Contains(t, out, "match msg.parent")
	assert.Contains(t, out, "| let v: Parent val =>")
	assert.Contains(t, out, "ParentCodec.encode(sub, v)")
}

func TestRepeatedScalarField(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{zooFileProto()}, "zoo.pony")

	// Class field and constructor default.
	assert.Contains(t, out, "let tags: Array[String val] val")
	assert.Contains(t, out, "tags': Array[String val] val = recover val Array[String val] end")

	// Decode: trn accumulator + per-element read (string is never packed).
	assert.Contains(t, out, "var tags: Array[String val] trn = recover trn Array[String val] end")
	assert.Contains(t, out, "reader.read_string()")
	assert.Contains(t, out, "| let v: String val => tags.push(v)")

	// Constructor call consumes the trn.
	assert.Contains(t, out, "consume tags")

	// Encode: per-element (no packed blob for strings).
	assert.Contains(t, out, "for v in msg.tags.values() do")
	assert.Contains(t, out, "writer.write_tag(Tag(2, WireLenDelim))")
	assert.Contains(t, out, "writer.write_string(v)")
}

func TestRepeatedMessageField(t *testing.T) {
	t.Parallel()

	item := field("item", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	item.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	item.TypeName = proto.String(".pkg.Item")

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("pkg.proto"),
		Package: proto.String("pkg"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  proto.String("Container"),
				Field: []*descriptorpb.FieldDescriptorProto{item},
			},
			{Name: proto.String("Item")},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "pkg.pony")

	// Class field and default.
	assert.Contains(t, out, "let item: Array[Item val] val")
	assert.Contains(t, out, "item': Array[Item val] val = recover val Array[Item val] end")

	// Decode: trn accumulator, per-entry sub-codec.
	assert.Contains(t, out, "var item: Array[Item val] trn = recover trn Array[Item val] end")
	assert.Contains(t, out, "match ItemCodec.decode(WireReader(b))")
	assert.Contains(t, out, "| let v: Item val => item.push(v)")
	assert.Contains(t, out, "consume item")

	// Encode: one tag+len-delim per element.
	assert.Contains(t, out, "for v in msg.item.values() do")
	assert.Contains(t, out, "ItemCodec.encode(sub, v)")
	assert.Contains(t, out, "writer.write_tag(Tag(2, WireLenDelim))")
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

func TestRepeatedNumericField(t *testing.T) {
	t.Parallel()

	scores := field("scores", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32)
	scores.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()

	file := &descriptorpb.FileDescriptorProto{
		Name:   proto.String("game.proto"),
		Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Game"), Field: []*descriptorpb.FieldDescriptorProto{scores}},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "game.pony")

	// Both packed (WireLenDelim) and unpacked (WireVarint) arms must be emitted.
	assert.Contains(t, out, "(2, WireLenDelim)")
	assert.Contains(t, out, "(2, WireVarint)")
	// Packed arm uses sub-reader; unpacked arm reads directly.
	assert.Contains(t, out, "Scalar.read_int32(sub)")
	assert.Contains(t, out, "Scalar.read_int32(reader)")

	// Encode uses packed format.
	assert.Contains(t, out, "if msg.scores.size() > 0 then")
	assert.Contains(t, out, "Scalar.write_int32(sub, v)")
}

func TestServiceEmitsTodo(t *testing.T) {
	t.Parallel()

	file := &descriptorpb.FileDescriptorProto{
		Name:   proto.String("svc.proto"),
		Syntax: proto.String("proto3"),
		Service: []*descriptorpb.ServiceDescriptorProto{
			{Name: proto.String("GreeterService")},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "svc.pony")
	assert.Contains(t, out, "// TODO protoc-gen-pony: service GreeterService (service)")
}

// "User-provided params still win" (main.go:50) relies on existing entries
// appearing verbatim after the injected stubs — protogen's later-wins
// semantics depend on it. Lock in: nothing dropped or reordered, including
// empty values and duplicates.
func TestOptionalScalarField(t *testing.T) {
	t.Parallel()

	score := field("score", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32)
	score.Proto3Optional = proto.Bool(true)
	score.OneofIndex = proto.Int32(0)

	file := &descriptorpb.FileDescriptorProto{
		Name:   proto.String("player.proto"),
		Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  proto.String("Player"),
				Field: []*descriptorpb.FieldDescriptorProto{score},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{
					{Name: proto.String("_score")},
				},
			},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "player.pony")

	// Class field and constructor default.
	assert.Contains(t, out, "let score: (I32 | None)")
	assert.Contains(t, out, "score': (I32 | None) = None")

	// Decode var: (I32 | None)
	assert.Contains(t, out, "var score: (I32 | None) = None")

	// Encode: match on None (explicit presence — zero is emitted when set).
	assert.Contains(t, out, "match msg.score")
	assert.Contains(t, out, "| let v: I32 =>")
	assert.NotContains(t, out, "if msg.score != 0")
}

func TestOptionalEnumField(t *testing.T) {
	t.Parallel()

	status := field("status", 1, descriptorpb.FieldDescriptorProto_TYPE_ENUM)
	status.TypeName = proto.String(".opt_test.Color")
	status.Proto3Optional = proto.Bool(true)
	status.OneofIndex = proto.Int32(0)

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("opt_test.proto"),
		Package: proto.String("opt_test"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  proto.String("Palette"),
				Field: []*descriptorpb.FieldDescriptorProto{status},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{
					{Name: proto.String("_status")},
				},
			},
		},
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("Color"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("RED"), Number: proto.Int32(0)},
					{Name: proto.String("BLUE"), Number: proto.Int32(1)},
				},
			},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "opt_test.pony")

	// Class field: (Color | None), default None.
	assert.Contains(t, out, "let status: (Color | None)")
	assert.Contains(t, out, "status': (Color | None) = None")

	// Encode: match on None (not zero-check).
	assert.Contains(t, out, "match msg.status")
	assert.Contains(t, out, "| let v: Color =>")
	assert.NotContains(t, out, "if msg.status.value() != 0")
}

func TestCrossFileSameDirectoryRef(t *testing.T) {
	t.Parallel()

	addrField := field("address", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	addrField.TypeName = proto.String(".geo.Address")

	personFile := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("geo/person.proto"),
		Package:    proto.String("geo"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"geo/address.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  proto.String("Person"),
				Field: []*descriptorpb.FieldDescriptorProto{addrField},
			},
		},
	}
	addressFile := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("geo/address.proto"),
		Package: proto.String("geo"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Address")},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{addressFile, personFile}, "geo/person.pony")

	// Cross-file same-directory ref should be generated, not TODO.
	assert.Contains(t, out, "let address: (Address val | None)")
	assert.NotContains(t, out, "TODO protoc-gen-pony: field address")

	// Sub-codec calls present.
	assert.Contains(t, out, "AddressCodec.decode(WireReader(b))")
	assert.Contains(t, out, "AddressCodec.encode(sub, v)")
}

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
