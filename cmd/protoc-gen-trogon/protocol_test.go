package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestMain(m *testing.M) {
	if os.Getenv("TROGON_PLUGIN_PROCESS") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func invokePlugin(t *testing.T, input []byte, args ...string) []byte {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	command := exec.Command(executable, args...)
	command.Env = append(os.Environ(), "TROGON_PLUGIN_PROCESS=1")
	command.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	require.NoError(t, command.Run(), stderr.String())
	require.Empty(t, stderr.String())
	return stdout.Bytes()
}

func invokeRequest(t *testing.T, request *pluginpb.CodeGeneratorRequest) *pluginpb.CodeGeneratorResponse {
	t.Helper()
	input, err := proto.Marshal(request)
	require.NoError(t, err)
	var response pluginpb.CodeGeneratorResponse
	require.NoError(t, proto.Unmarshal(invokePlugin(t, input), &response), "stdout must contain only a protoc response")
	return &response
}

func TestProtocolRejectsMissingUnsupportedAndAmbiguousSelections(t *testing.T) {
	for _, test := range []struct{ options, message string }{
		{"", "language"},
		{"features=uuid", "language"},
		{"language=go,features=uuid", "unsupported language"},
		{"language=rust", "features"},
		{"language=rust,features=", "features"},
		{"language=rust,features=messages", "unsupported feature"},
		{"language=rust,features=uuid+messages", "unsupported feature"},
		{"language=rust,language=rust,features=uuid", "duplicate"},
		{"language=rust,features=uuid,features=uuid", "duplicate"},
		{"language=rust,features=uuid+uuid", "duplicate"},
		{"language=rust,features=uuid,framework=prost", "unknown option"},
		{"language=rust,features=uuid,", "empty option"},
		{"language=rust,features=uuid,runtime_path=::", "runtime_path"},
		{"language=rust,features=uuid,runtime_path=::uuid_runtime::::x", "runtime_path"},
		{"language=rust,features=uuid,runtime_path=::uuid_runtime::<String>", "runtime_path"},
		{"language=rust,features=uuid,runtime_path=::uuid_runtime;panic!()", "runtime_path"},
	} {
		t.Run(test.options, func(t *testing.T) {
			request := validRequest(t)
			request.Parameter = proto.String(test.options)
			response := invokeRequest(t, request)
			require.Contains(t, response.GetError(), test.message)
			require.Empty(t, response.File, "invalid selection must produce no generated files")
		})
	}
}

func TestProtocolRustUUIDSelectionAndRuntimeAlias(t *testing.T) {
	request := validRequest(t)
	request.Parameter = proto.String("features=uuid,language=rust,runtime_path=::uuid_runtime")
	response := invokeRequest(t, request)
	require.Empty(t, response.GetError())
	require.Len(t, response.File, 1)
	require.Equal(t, "example/catalog/v1/identity.uuid.rs", response.File[0].GetName())
	require.Contains(t, response.File[0].GetContent(), "::uuid_runtime::UuidTemplate")
	require.NotContains(t, response.File[0].GetContent(), "::trogon_uuid::")
	require.NotZero(t, response.GetSupportedFeatures()&uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL))
}

func TestProtocolFailureDiscardsPreviouslyGeneratedFiles(t *testing.T) {
	request := validRequest(t)
	request.FileToGenerate = append(request.FileToGenerate, "missing.proto")
	response := invokeRequest(t, request)
	require.Contains(t, response.GetError(), "find missing.proto")
	require.Empty(t, response.File)
}

func TestProtocolMalformedInput(t *testing.T) {
	var response pluginpb.CodeGeneratorResponse
	require.NoError(t, proto.Unmarshal(invokePlugin(t, []byte{0xff}), &response))
	require.Contains(t, response.GetError(), "decode CodeGeneratorRequest")
	require.Empty(t, response.File)
}

func TestMonolithicCommandIdentity(t *testing.T) {
	require.Contains(t, string(invokePlugin(t, nil, "--version")), "protoc-gen-trogon dev (commit:")
	help := string(invokePlugin(t, nil, "--help"))
	require.Contains(t, help, "--trogon_out")
	require.Contains(t, help, "language=rust,features=uuid")
}

func TestProtocolUnannotatedFilesNeedNoUUIDSchema(t *testing.T) {
	for _, shape := range []string{"messages_only", "top_level_enum", "nested_enum"} {
		t.Run(shape, func(t *testing.T) {
			file := &descriptorpb.FileDescriptorProto{
				Name: proto.String("example/plain.proto"), Syntax: proto.String("proto3"),
				MessageType: []*descriptorpb.DescriptorProto{{Name: proto.String("Plain")}},
			}
			enum := &descriptorpb.EnumDescriptorProto{
				Name:  proto.String("Status"),
				Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("UNKNOWN"), Number: proto.Int32(0)}},
			}
			switch shape {
			case "top_level_enum":
				file.EnumType = []*descriptorpb.EnumDescriptorProto{enum}
			case "nested_enum":
				file.MessageType[0].NestedType = []*descriptorpb.DescriptorProto{{
					Name: proto.String("Nested"), EnumType: []*descriptorpb.EnumDescriptorProto{enum},
				}}
			}
			response := invokeRequest(t, &pluginpb.CodeGeneratorRequest{
				Parameter:      proto.String("language=rust,features=uuid"),
				FileToGenerate: []string{file.GetName()}, ProtoFile: []*descriptorpb.FileDescriptorProto{file},
			})
			require.Empty(t, response.GetError())
			require.Empty(t, response.File)
		})
	}
}

func TestProtocolConfiguredFilesRequireUUIDSchema(t *testing.T) {
	for _, location := range []string{"enum", "value", "nested_value"} {
		t.Run(location, func(t *testing.T) {
			enum := &descriptorpb.EnumDescriptorProto{
				Name:  proto.String("Identity"),
				Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("IDENTITY"), Number: proto.Int32(0)}},
			}
			file := &descriptorpb.FileDescriptorProto{
				Name: proto.String("example/configured.proto"), Syntax: proto.String("proto3"),
				EnumType: []*descriptorpb.EnumDescriptorProto{enum},
			}
			if location == "enum" {
				enum.Options = new(descriptorpb.EnumOptions)
				enum.Options.ProtoReflect().SetUnknown(protowire.AppendBytes(protowire.AppendTag(nil, 870001, protowire.BytesType), nil))
			} else {
				enum.Value[0].Options = new(descriptorpb.EnumValueOptions)
				enum.Value[0].Options.ProtoReflect().SetUnknown(protowire.AppendBytes(protowire.AppendTag(nil, 870002, protowire.BytesType), nil))
			}
			if location == "nested_value" {
				file.EnumType = nil
				file.MessageType = []*descriptorpb.DescriptorProto{{
					Name: proto.String("Container"), EnumType: []*descriptorpb.EnumDescriptorProto{enum},
				}}
			}
			response := invokeRequest(t, &pluginpb.CodeGeneratorRequest{
				Parameter:      proto.String("language=rust,features=uuid"),
				FileToGenerate: []string{file.GetName()}, ProtoFile: []*descriptorpb.FileDescriptorProto{file},
			})
			require.Contains(t, response.GetError(), "required extension")
			require.Empty(t, response.File)
		})
	}
}
