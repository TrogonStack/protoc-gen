package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func testMethod(name, inputType, outputType string, clientStreaming, serverStreaming bool) *descriptorpb.MethodDescriptorProto {
	return &descriptorpb.MethodDescriptorProto{
		Name:            proto.String(name),
		InputType:       proto.String(inputType),
		OutputType:      proto.String(outputType),
		ClientStreaming: proto.Bool(clientStreaming),
		ServerStreaming: proto.Bool(serverStreaming),
	}
}

// serviceTypes is a TypeRegistry pre-populated with the two message types
// service.proto's TestService cross-references, mirroring the real
// cross-file resolution the golden fixture exercises (Request/Reply are
// defined in test.proto, not service.proto itself).
var serviceTypes = &TypeRegistry{
	modNames: map[string]string{
		".test.Request": "Test.Request",
		".test.Reply":   "Test.Reply",
	},
}

// TestRenderService_Basic covers basic rpc rendering and name derivation:
// the .Service/.Stub module names come from the Elixir-camelized service
// name, while the `name:` use-option uses the raw proto-qualified name -
// verified byte-for-byte against testdata/golden/grpc/test/service.pb.ex.
func TestRenderService_Basic(t *testing.T) {
	t.Parallel()

	svc := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String("TestService"),
		Method: []*descriptorpb.MethodDescriptorProto{
			testMethod("test", ".test.Request", ".test.Reply", false, false),
		},
	}

	docComment := "An example test service that has\na test method. It expects a Request\nand returns a Reply."

	got, err := RenderService(svc, "Test.TestService.Service", "Test.TestService.Stub", "test.TestService", true, docComment, false, "", serviceTypes)
	require.NoError(t, err)

	want := `defmodule Test.TestService.Service do
  @moduledoc """
  An example test service that has
  a test method. It expects a Request
  and returns a Reply.
  """

  use GRPC.Service, name: "test.TestService", protoc_gen_elixir_version: "0.17.0"

  rpc :test, Test.Request, Test.Reply
end

defmodule Test.TestService.Stub do
  use GRPC.Stub, service: Test.TestService.Service
end`

	assert.Equal(t, want, got)
}

// TestRenderService_ProtoSourcePlacement covers def proto_source() placement
// (standalone function, after `use GRPC.Service`, before the rpc lines) and
// its exclusion from the .Stub module - verified byte-for-byte against
// testdata/golden/grpc_proto_source/test/service.pb.ex, which also exercises
// include_docs=false (so both the .Service and .Stub modules get
// @moduledoc false).
func TestRenderService_ProtoSourcePlacement(t *testing.T) {
	t.Parallel()

	svc := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String("TestService"),
		Method: []*descriptorpb.MethodDescriptorProto{
			testMethod("test", ".test.Request", ".test.Reply", false, false),
		},
	}

	got, err := RenderService(svc, "Test.TestService.Service", "Test.TestService.Stub", "test.TestService", false, "", false, "service.proto", serviceTypes)
	require.NoError(t, err)

	want := `defmodule Test.TestService.Service do
  @moduledoc false

  use GRPC.Service, name: "test.TestService", protoc_gen_elixir_version: "0.17.0"

  def proto_source(), do: "service.proto"

  rpc :test, Test.Request, Test.Reply
end

defmodule Test.TestService.Stub do
  @moduledoc false

  use GRPC.Stub, service: Test.TestService.Service
end`

	assert.Equal(t, want, got)
}

// TestRenderService_StubModuledocConditional covers both directions of
// the Stub-specific @moduledoc rule (distinct from the ordinary
// three-way branch RenderMessage/RenderEnum use): @moduledoc false only when
// include_docs=false; no @moduledoc line at all when include_docs=true,
// regardless of the service's own doc comment.
func TestRenderService_StubModuledocConditional(t *testing.T) {
	t.Parallel()

	svc := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String("TestService"),
		Method: []*descriptorpb.MethodDescriptorProto{
			testMethod("test", ".test.Request", ".test.Reply", false, false),
		},
	}

	t.Run("include_docs=false emits @moduledoc false on Stub", func(t *testing.T) {
		t.Parallel()

		got, err := RenderService(svc, "Test.TestService.Service", "Test.TestService.Stub", "test.TestService", false, "", false, "", serviceTypes)
		require.NoError(t, err)
		assert.Contains(t, got, "defmodule Test.TestService.Stub do\n  @moduledoc false\n\n  use GRPC.Stub")
	})

	t.Run("include_docs=true emits no @moduledoc line on Stub even with a doc comment", func(t *testing.T) {
		t.Parallel()

		got, err := RenderService(svc, "Test.TestService.Service", "Test.TestService.Stub", "test.TestService", true, "Some doc comment.", false, "", serviceTypes)
		require.NoError(t, err)
		assert.Contains(t, got, "defmodule Test.TestService.Stub do\n  use GRPC.Stub")
		assert.NotContains(t, got, "Stub do\n  @moduledoc")
	})
}

// TestRenderService_Streaming is hand-derived strictly from the spec's
// illustrative example under "Service Modules" - no fixture in the current
// corpus (service.proto's only rpc is plain unary) exercises streaming at
// all, so this is spec-derived, NOT golden-verified, matching the honesty
// standard already established for Phase 3's proto3_optional gap.
func TestRenderService_Streaming(t *testing.T) {
	t.Parallel()

	svc := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String("MyService"),
		Method: []*descriptorpb.MethodDescriptorProto{
			testMethod("Unary", ".test.Request", ".test.Reply", false, false),
			testMethod("ClientStream", ".test.Request", ".test.Reply", true, false),
			testMethod("ServerStream", ".test.Request", ".test.Reply", false, true),
			testMethod("Bidi", ".test.Request", ".test.Reply", true, true),
		},
	}

	got, err := RenderService(svc, "Pkg.MyService.Service", "Pkg.MyService.Stub", "pkg.MyService", false, "", false, "", serviceTypes)
	require.NoError(t, err)

	assert.Contains(t, got, "  rpc :Unary, Test.Request, Test.Reply")
	assert.Contains(t, got, "  rpc :ClientStream, stream(Test.Request), Test.Reply")
	assert.Contains(t, got, "  rpc :ServerStream, Test.Request, stream(Test.Reply)")
	assert.Contains(t, got, "  rpc :Bidi, stream(Test.Request), stream(Test.Reply)")
}

// TestRenderService_CrossFileTypeResolution exercises the first
// cross-file (as opposed to same-file) TypeRegistry.Resolve lookup in this
// codebase's own test corpus - Request/Reply are defined in a different
// proto file than the service, mirroring the real service.proto/test.proto
// split in testdata/proto/.
func TestRenderService_CrossFileTypeResolution(t *testing.T) {
	t.Parallel()

	types := &TypeRegistry{
		modNames: map[string]string{
			".other.pkg.Foo": "Other.Pkg.Foo",
			".other.pkg.Bar": "Other.Pkg.Bar",
		},
	}

	svc := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String("Svc"),
		Method: []*descriptorpb.MethodDescriptorProto{
			testMethod("Call", ".other.pkg.Foo", ".other.pkg.Bar", false, false),
		},
	}

	got, err := RenderService(svc, "Mine.Svc.Service", "Mine.Svc.Stub", "mine.Svc", false, "", false, "", types)
	require.NoError(t, err)
	assert.Contains(t, got, "  rpc :Call, Other.Pkg.Foo, Other.Pkg.Bar")
}

// TestRenderService_UnresolvableType covers the error path (no panic) when a
// method's input/output type isn't in the registry, mirroring
// renderFieldTypeValue's error pattern in message.go.
func TestRenderService_UnresolvableType(t *testing.T) {
	t.Parallel()

	svc := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String("Svc"),
		Method: []*descriptorpb.MethodDescriptorProto{
			testMethod("Call", ".unknown.Foo", ".test.Reply", false, false),
		},
	}

	_, err := RenderService(svc, "Mine.Svc.Service", "Mine.Svc.Stub", "mine.Svc", false, "", false, "", serviceTypes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown.Foo")
}

// TestRenderService_UnresolvableOutputType covers the same error path for
// the output type specifically, since renderRPC resolves input and output
// independently.
func TestRenderService_UnresolvableOutputType(t *testing.T) {
	t.Parallel()

	svc := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String("Svc"),
		Method: []*descriptorpb.MethodDescriptorProto{
			testMethod("Call", ".test.Request", ".unknown.Bar", false, false),
		},
	}

	_, err := RenderService(svc, "Mine.Svc.Service", "Mine.Svc.Stub", "mine.Svc", false, "", false, "", serviceTypes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown.Bar")
}
