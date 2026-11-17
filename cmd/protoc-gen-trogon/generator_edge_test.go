package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func edgeWireBytes(number protowire.Number, value []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, number, protowire.BytesType), value)
}

func edgeValue(name, template string, number int32) *descriptorpb.EnumValueDescriptorProto {
	options := new(descriptorpb.EnumValueOptions)
	options.ProtoReflect().SetUnknown(edgeWireBytes(870002, edgeWireBytes(1, edgeWireBytes(2, []byte(template)))))
	return &descriptorpb.EnumValueDescriptorProto{Name: proto.String(name), Number: proto.Int32(number), Options: options}
}

func edgeRequest(t *testing.T, values ...*descriptorpb.EnumValueDescriptorProto) *pluginpb.CodeGeneratorRequest {
	t.Helper()
	var descriptors descriptorpb.FileDescriptorSet
	require.NoError(t, proto.Unmarshal(testdataRecipes, &descriptors))
	var files []*descriptorpb.FileDescriptorProto
	for _, file := range descriptors.File {
		if file.GetName() != "example/catalog/v1/identity.proto" {
			files = append(files, file)
		}
	}
	options := new(descriptorpb.EnumOptions)
	options.ProtoReflect().SetUnknown(edgeWireBytes(870001, edgeWireBytes(1, edgeWireBytes(2, []byte("inventory.example.com")))))
	file := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("example/edge.proto"),
		Package:    proto.String("example.edge.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"trogon/uuid/v1/options.proto"},
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: proto.String("IdentityVersion"), Options: options, Value: values,
		}},
	}
	return &pluginpb.CodeGeneratorRequest{
		Parameter: proto.String("language=rust,features=uuid"), FileToGenerate: []string{file.GetName()},
		ProtoFile: append(files, file),
	}
}

func edgeOutput(t *testing.T, request *pluginpb.CodeGeneratorRequest) string {
	t.Helper()
	response := generate(request)
	require.Empty(t, response.GetError())
	require.Len(t, response.GetFile(), 1)
	return response.GetFile()[0].GetContent()
}

func TestGenerateVisitsNestedEnums(t *testing.T) {
	request := edgeRequest(t, edgeValue("NESTED_ID", "{key}", 0))
	file := request.ProtoFile[len(request.ProtoFile)-1]
	file.MessageType = []*descriptorpb.DescriptorProto{{
		Name: proto.String("Outer"), NestedType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Inner"), EnumType: file.EnumType,
		}},
	}}
	file.EnumType = nil
	content := edgeOutput(t, request)
	require.Contains(t, content, "pub struct NestedId;")
	require.Contains(t, content, "pub struct NestedIdInput<'a>")
}

func TestGenerateRejectsRustTypeCollisions(t *testing.T) {
	for _, tc := range []struct {
		name, first, second, template, collision string
	}{
		{"marker", "FOO_BAR", "FOO__BAR", "singleton", "FooBar"},
		{"marker_and_input", "FOO", "FOO_INPUT", "{key}", "FooInput"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := edgeRequest(t, edgeValue(tc.first, tc.template, 0))
			file := request.ProtoFile[len(request.ProtoFile)-1]
			file.EnumType = append(file.EnumType, &descriptorpb.EnumDescriptorProto{
				Name: proto.String("OtherIdentityVersion"), Options: proto.Clone(file.EnumType[0].Options).(*descriptorpb.EnumOptions),
				Value: []*descriptorpb.EnumValueDescriptorProto{edgeValue(tc.second, "singleton", 0)},
			})
			response := generate(request)
			require.Contains(t, response.GetError(), "duplicate")
			require.Contains(t, response.GetError(), tc.collision)
			require.Empty(t, response.GetFile())
		})
	}
}

func TestGenerateRejectsInvalidRustMarkers(t *testing.T) {
	for _, name := range []string{"SELF", "_"} {
		t.Run(name, func(t *testing.T) {
			response := generate(edgeRequest(t, edgeValue(name, "singleton", 0)))
			require.Contains(t, response.GetError(), "marker")
			require.Empty(t, response.GetFile())
		})
	}
}

func TestGenerateRejectsUnrepresentablePlaceholders(t *testing.T) {
	for _, field := range []string{"_", "self", "Self", "super", "crate", "ключ"} {
		t.Run(field, func(t *testing.T) {
			response := generate(edgeRequest(t, edgeValue("IDENTITY", "{"+field+"}", 0)))
			require.Contains(t, response.GetError(), "placeholder")
			require.Empty(t, response.GetFile())
		})
	}
}

func TestGenerateEscapesRustKeywordFieldsConsistently(t *testing.T) {
	content := edgeOutput(t, edgeRequest(t, edgeValue("IDENTITY", "{type}/{try}/{gen}", 0)))
	for _, field := range []string{"type", "try", "gen"} {
		require.Contains(t, content, "pub r#"+field+": &'a str")
		require.Contains(t, content, "input.r#"+field+".as_bytes()")
	}
}

func TestGenerateEmitsRustStringEscapes(t *testing.T) {
	content := edgeOutput(t, edgeRequest(t, edgeValue("IDENTITY", "\u2028/\u00a0/\a/\v/\f/\\\"/{key}", 0)))
	for _, escape := range []string{`\u{2028}`, `\u{a0}`, `\u{7}`, `\u{b}`, `\u{c}`, `\\`, `\"`} {
		require.Contains(t, content, escape)
	}
	for _, goEscape := range []string{`\u2028`, `\u00a0`, `\a/`, `\v/`, `\f/`} {
		require.NotContains(t, content, goEscape)
	}
}

func TestGeneratePreservesStaticAndEmptyUuidVectors(t *testing.T) {
	content := edgeOutput(t, edgeRequest(t,
		edgeValue("STATIC_TEMPLATE", "singleton", 0), edgeValue("EMPTY_TEMPLATE", "", 1),
	))
	require.Contains(t, content, "[0xe2, 0x41, 0x52, 0x5a, 0x82, 0x5f, 0x50, 0x88, 0xba, 0x4b, 0x73, 0x94, 0x6b, 0x79, 0xc8, 0xd0]")
	require.Contains(t, content, "[0x94, 0x18, 0x68, 0xaf, 0x9b, 0x87, 0x5e, 0x64, 0x90, 0xc5, 0xa6, 0xa2, 0x1b, 0xb4, 0x60, 0x3e]")
	require.Contains(t, content, "type Input<'a> = ();")
	require.NotContains(t, content, "StaticTemplateInput")
	require.NotContains(t, content, "EmptyTemplateInput")
}

func TestGenerateRejectsDriftedTemplateFieldNumber(t *testing.T) {
	request := edgeRequest(t, edgeValue("IDENTITY", "{key}", 0))
	for _, file := range request.ProtoFile {
		if file.GetName() != "trogon/uuid/v1/options.proto" {
			continue
		}
		for _, message := range file.MessageType {
			if message.GetName() != "EnumValueOptions" {
				continue
			}
			for _, nested := range message.NestedType {
				if nested.GetName() != "Format" {
					continue
				}
				for _, field := range nested.Field {
					if field.GetName() == "template" {
						field.Number = proto.Int32(3)
					}
				}
			}
		}
	}
	response := generate(request)
	require.Contains(t, response.GetError(), "template")
	require.Empty(t, response.GetFile())
}
