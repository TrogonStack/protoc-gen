package main

import (
	"path"
	"strings"
	"testing"
	"testing/quick"

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
	assert.Contains(t, out, "let count: (I32 | None)")

	// The `kind` oneof (type_a + type_b) is now supported via oneof codegen.
	assert.Contains(t, out, "let kind: ZooKind")
	assert.NotContains(t, out, "TODO protoc-gen-pony: field type_a")
	assert.NotContains(t, out, "TODO protoc-gen-pony: field type_b")

	// map<string, int32> is now generated.
	assert.Contains(t, out, "let metadata: Map[String val, I32] val")
	assert.NotContains(t, out, "TODO protoc-gen-pony: field metadata")
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

func TestOneofWrapperTypes(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{zooFileProto()}, "zoo.pony")

	// Wrapper classes for each oneof member.
	assert.Contains(t, out, "class val ZooKindTypeA")
	assert.Contains(t, out, "let value: String val")
	assert.Contains(t, out, "new val create(value': String val = \"\") => value = value'")

	assert.Contains(t, out, "class val ZooKindTypeB")
	assert.Contains(t, out, "let value: I32")
	assert.Contains(t, out, "new val create(value': I32 = 0) => value = value'")

	// Type alias union includes None.
	assert.Contains(t, out, "type ZooKind is (ZooKindTypeA | ZooKindTypeB | None)")
}

func TestOneofConstructorParam(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{zooFileProto()}, "zoo.pony")
	assert.Contains(t, out, "kind': ZooKind = None")
	assert.Contains(t, out, "kind = kind'")
}

func TestOneofDecode(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{zooFileProto()}, "zoo.pony")

	// Declare the oneof var.
	assert.Contains(t, out, "var kind: ZooKind = None")

	// String arm (type_a = field 4) wraps in ZooKindTypeA.
	assert.Contains(t, out, "(4, WireLenDelim)")
	assert.Contains(t, out, "| let v: String val => kind = ZooKindTypeA(v)")

	// Int32 arm (type_b = field 5) wraps in ZooKindTypeB.
	assert.Contains(t, out, "(5, WireVarint)")
	assert.Contains(t, out, "| let v: I32 => kind = ZooKindTypeB(v)")

	// Constructor call includes kind.
	assert.Contains(t, out, "kind)")
}

func TestOneofEncode(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{zooFileProto()}, "zoo.pony")

	assert.Contains(t, out, "match msg.kind")
	assert.Contains(t, out, "| let v: ZooKindTypeA =>")
	assert.Contains(t, out, "writer.write_tag(Tag(4, WireLenDelim))")
	assert.Contains(t, out, "writer.write_string(v.value)")
	assert.Contains(t, out, "| let v: ZooKindTypeB =>")
	assert.Contains(t, out, "writer.write_tag(Tag(5, WireVarint))")
	assert.Contains(t, out, "Scalar.write_int32(writer, v.value)")
	assert.Contains(t, out, "| None => None")
}

func TestOneofWithMessageMember(t *testing.T) {
	t.Parallel()

	childField := field("child", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	childField.TypeName = proto.String(".pkg.Child")
	childField.OneofIndex = proto.Int32(0)

	numField := field("num", 3, descriptorpb.FieldDescriptorProto_TYPE_INT64)
	numField.OneofIndex = proto.Int32(0)

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("pkg.proto"),
		Package: proto.String("pkg"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  proto.String("Parent"),
				Field: []*descriptorpb.FieldDescriptorProto{childField, numField},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{
					{Name: proto.String("payload")},
				},
			},
			{Name: proto.String("Child")},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "pkg.pony")

	// Message member: no default in wrapper constructor (no = value).
	assert.Contains(t, out, "class val ParentPayloadChild")
	assert.Contains(t, out, "let value: Child val")
	assert.Contains(t, out, "new val create(value': Child val) => value = value'")
	assert.NotContains(t, out, "new val create(value': Child val =")

	// Int64 member has default.
	assert.Contains(t, out, "class val ParentPayloadNum")
	assert.Contains(t, out, "new val create(value': I64 = 0)")

	// Type alias.
	assert.Contains(t, out, "type ParentPayload is (ParentPayloadChild | ParentPayloadNum | None)")

	// Decode: message arm reads sub-codec.
	assert.Contains(t, out, "match ChildCodec.decode(WireReader(b))")
	assert.Contains(t, out, "| let v: Child val => payload = ParentPayloadChild(v)")

	// Encode: message arm uses sub-writer.
	assert.Contains(t, out, "| let v: ParentPayloadChild =>")
	assert.Contains(t, out, "ChildCodec.encode(sub, v.value)")
}

func TestOneofUnsupportedWhenMemberIsWKT(t *testing.T) {
	t.Parallel()

	// google/protobuf/struct.proto is in the blocklist — a Value oneof member
	// keeps the whole oneof as TODO.
	valueField := field("payload", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	valueField.TypeName = proto.String(".google.protobuf.Value")
	valueField.OneofIndex = proto.Int32(0)
	strField := field("label", 3, descriptorpb.FieldDescriptorProto_TYPE_STRING)
	strField.OneofIndex = proto.Int32(0)

	structFile := &descriptorpb.FileDescriptorProto{
		Name:        proto.String("google/protobuf/struct.proto"),
		Package:     proto.String("google.protobuf"),
		Syntax:      proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{Name: proto.String("Value")}},
	}
	eventFile := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("event.proto"),
		Package:    proto.String("event"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/struct.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  proto.String("Event"),
				Field: []*descriptorpb.FieldDescriptorProto{valueField, strField},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{
					{Name: proto.String("when")},
				},
			},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{structFile, eventFile}, "event.pony")

	// Whole oneof stays TODO because one member (Value from struct.proto) is blocked.
	assert.Contains(t, out, "TODO protoc-gen-pony: field payload")
	assert.Contains(t, out, "TODO protoc-gen-pony: field label")
	assert.NotContains(t, out, "type EventWhen")
}

func TestCrossDirectoryRef(t *testing.T) {
	t.Parallel()

	addrField := field("address", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	addrField.TypeName = proto.String(".common.Address")

	personFile := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("geo/person.proto"),
		Package:    proto.String("geo"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"common/address.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  proto.String("Person"),
				Field: []*descriptorpb.FieldDescriptorProto{addrField},
			},
		},
	}
	addressFile := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("common/address.proto"),
		Package: proto.String("common"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Address")},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{addressFile, personFile}, "geo/person.pony")

	// use directive for the cross-directory dep.
	assert.Contains(t, out, `use "../common"`)

	// Field generated (not TODO).
	assert.Contains(t, out, "let address: (Address val | None)")
	assert.NotContains(t, out, "TODO protoc-gen-pony: field address")

	// Codec calls generated.
	assert.Contains(t, out, "AddressCodec.decode(WireReader(b))")
	assert.Contains(t, out, "AddressCodec.encode(sub, v)")
}

func TestCrossDirectoryDedupedUse(t *testing.T) {
	t.Parallel()

	cityField := field("city", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	cityField.TypeName = proto.String(".common.City")
	countryField := field("country", 3, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	countryField.TypeName = proto.String(".common.Country")

	personFile := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("geo/person.proto"),
		Package:    proto.String("geo"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"common/city.proto", "common/country.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  proto.String("Person"),
				Field: []*descriptorpb.FieldDescriptorProto{cityField, countryField},
			},
		},
	}
	cityFile := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("common/city.proto"),
		Package: proto.String("common"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("City")},
		},
	}
	countryFile := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("common/country.proto"),
		Package: proto.String("common"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Country")},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{cityFile, countryFile, personFile}, "geo/person.pony")

	// Only one use directive for common/ even though two deps come from there.
	assert.Equal(t, 1, strings.Count(out, `use "../common"`))
}

func TestWKTRefEmitsTodo(t *testing.T) {
	t.Parallel()

	// google/protobuf/struct.proto is in the blocklist (circular Value type).
	valueField := field("payload", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	valueField.TypeName = proto.String(".google.protobuf.Value")

	structFile := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("google/protobuf/struct.proto"),
		Package: proto.String("google.protobuf"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Value")},
		},
	}
	eventFile := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("events/event.proto"),
		Package:    proto.String("events"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/struct.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  proto.String("Event"),
				Field: []*descriptorpb.FieldDescriptorProto{valueField},
			},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{structFile, eventFile}, "events/event.pony")

	// Blocked WKT field stays as TODO.
	assert.Contains(t, out, "TODO protoc-gen-pony: field payload")

	// No use directive for blocked WKT.
	assert.NotContains(t, out, `use "../google/protobuf"`)
}

func TestWKT_TimestampGenerates(t *testing.T) {
	t.Parallel()

	tsField := field("created_at", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	tsField.TypeName = proto.String(".google.protobuf.Timestamp")

	tsFile := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("google/protobuf/timestamp.proto"),
		Package: proto.String("google.protobuf"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Timestamp"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("seconds", 1, descriptorpb.FieldDescriptorProto_TYPE_INT64),
					field("nanos", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32),
				},
			},
		},
	}
	eventFile := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("events/event.proto"),
		Package:    proto.String("events"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/timestamp.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:  proto.String("Event"),
				Field: []*descriptorpb.FieldDescriptorProto{tsField},
			},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{tsFile, eventFile}, "events/event.pony")

	// Timestamp generates as a real class (not TODO).
	assert.Contains(t, out, "let created_at: (Timestamp val | None)")
	assert.NotContains(t, out, "TODO protoc-gen-pony: field created_at")

	// Cross-dir use directive emitted.
	assert.Contains(t, out, `use "../google/protobuf"`)
}

// cleanDirSegs filters an arbitrary []string down to a valid proto directory
// path (slash-separated ASCII alphanumeric components). Returns ("", false)
// when no valid segments remain.
func cleanDirSegs(segs []string) (string, bool) {
	var out []string
	for _, s := range segs {
		clean := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, s)
		if clean != "" {
			out = append(out, clean)
		}
	}
	if len(out) == 0 {
		return "", false
	}
	return strings.Join(out, "/"), true
}

// TestProtoRelDir_Inverse verifies that joining `from` with the result of
// protoRelDir always resolves back to `to`.
func TestProtoRelDir_Inverse(t *testing.T) {
	f := func(fromSegs, toSegs []string) bool {
		from, ok1 := cleanDirSegs(fromSegs)
		to, ok2 := cleanDirSegs(toSegs)
		if !ok1 || !ok2 || from == to {
			return true // skip: preconditions not met
		}
		return path.Join(from, protoRelDir(from, to)) == to
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 2000}); err != nil {
		t.Error(err)
	}
}

// TestSnakeToPascal_NeverContainsUnderscore verifies that snakeToPascal removes
// all underscores regardless of the input string.
func TestSnakeToPascal_NeverContainsUnderscore(t *testing.T) {
	f := func(s string) bool {
		return !strings.Contains(snakeToPascal(s), "_")
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 2000}); err != nil {
		t.Error(err)
	}
}

func TestMapField_ClassAndCodec(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{zooFileProto()}, "zoo.pony")

	// Class field and constructor default.
	assert.Contains(t, out, "let metadata: Map[String val, I32] val")
	assert.Contains(t, out, "metadata': Map[String val, I32] val = recover val Map[String val, I32] end")

	// Decode: trn accumulator, entry sub-reader, key/value arms, final assign.
	assert.Contains(t, out, "var metadata: Map[String val, I32] trn = recover trn Map[String val, I32] end")
	assert.Contains(t, out, "let entry_sub = WireReader(b)")
	assert.Contains(t, out, "var entry_k: String val")
	assert.Contains(t, out, "var entry_v: I32")
	assert.Contains(t, out, "metadata(entry_k) = entry_v")
	assert.Contains(t, out, "consume metadata")

	// Encode: pairs() loop, always-write key + value.
	assert.Contains(t, out, "for (k, v) in msg.metadata.pairs() do")
	assert.Contains(t, out, "sub.write_string(k)")
	assert.Contains(t, out, "Scalar.write_int32(sub, v)")
}

func TestMapField_UseCollections(t *testing.T) {
	t.Parallel()
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{zooFileProto()}, "zoo.pony")
	assert.Contains(t, out, `use "collections"`)
}

func TestMapField_MessageValue(t *testing.T) {
	t.Parallel()

	entryField := field("items", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	entryField.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	entryField.TypeName = proto.String(".pkg.Container.ItemsEntry")
	mapEntry := &descriptorpb.DescriptorProto{
		Name: proto.String("ItemsEntry"),
		Field: []*descriptorpb.FieldDescriptorProto{
			field("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING),
			{
				Name:     proto.String("value"),
				Number:   proto.Int32(2),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(".pkg.Item"),
				JsonName: proto.String("value"),
			},
		},
		Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
	}
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("pkg.proto"),
		Package: proto.String("pkg"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:       proto.String("Container"),
				Field:      []*descriptorpb.FieldDescriptorProto{entryField},
				NestedType: []*descriptorpb.DescriptorProto{mapEntry},
			},
			{Name: proto.String("Item")},
		},
	}
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "pkg.pony")

	// map<string, Item> now generates.
	assert.Contains(t, out, "let items: Map[String val, Item val] val")
	assert.NotContains(t, out, "TODO protoc-gen-pony: field items")

	// codec.default() emitted for Item and Container.
	assert.Contains(t, out, "fun default(): Item val => Item")
	assert.Contains(t, out, "fun default(): Container val => Container")

	// Decode: sub-codec with ItemCodec.default() as initial value.
	assert.Contains(t, out, "var entry_v: Item val = ItemCodec.default()")
	assert.Contains(t, out, "match ItemCodec.decode(WireReader(vb))")

	// Encode: sub-writer for value.
	assert.Contains(t, out, "ItemCodec.encode(vsub, v)")

	assert.Contains(t, out, `use "collections"`)
}

func TestMapField_EnumValue(t *testing.T) {
	t.Parallel()

	entryField := field("by_status", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	entryField.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	entryField.TypeName = proto.String(".pkg.Lookup.ByStatusEntry")
	mapEntry := &descriptorpb.DescriptorProto{
		Name: proto.String("ByStatusEntry"),
		Field: []*descriptorpb.FieldDescriptorProto{
			field("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING),
			{
				Name:     proto.String("value"),
				Number:   proto.Int32(2),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
				TypeName: proto.String(".pkg.Color"),
				JsonName: proto.String("value"),
			},
		},
		Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
	}
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("pkg.proto"),
		Package: proto.String("pkg"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:       proto.String("Lookup"),
				Field:      []*descriptorpb.FieldDescriptorProto{entryField},
				NestedType: []*descriptorpb.DescriptorProto{mapEntry},
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
	out := runPlugin(t, []*descriptorpb.FileDescriptorProto{file}, "pkg.pony")

	// Class field uses enum type as map value.
	assert.Contains(t, out, "let by_status: Map[String val, Color] val")

	// Decode: FromValue applied to I32.
	assert.Contains(t, out, "| let vv: I32 => entry_v = ColorFromValue(vv)")

	// Encode: .value() call on enum.
	assert.Contains(t, out, "Scalar.write_int32(sub, v.value())")

	// use "collections" emitted.
	assert.Contains(t, out, `use "collections"`)
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
