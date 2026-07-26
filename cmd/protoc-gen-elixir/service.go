package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
)

// RenderService mirrors priv/templates/service.ex.eex + Protobuf.Protoc.Generator.Service
// at the pinned escript HEAD, rendering a service's paired `.Service` and
// `.Stub` modules (gated on plugins=grpc). serviceModName/stubModName are the already-qualified Elixir
// module names (e.g. "Test.TestService.Service" / "Test.TestService.Stub");
// fullName is the raw proto-qualified service name (e.g. "test.TestService"),
// used verbatim as the `name:` value in `use GRPC.Service` - NOT the Elixir
// module name.
//
// types resolves each method's input/output message type. genDescriptors,
// when true, emits a `def descriptor do ... end` function rendering the
// service's own ServiceDescriptorProto as an Elixir struct literal - see
// descriptor.go. Placement (immediately after `def proto_source()` when both
// gen_proto_source and gen_descriptors are set, or in that same slot right
// after `use GRPC.Service` otherwise) is ENTIRELY UNEVIDENCED: no fixture in
// the corpus combines gen_descriptors=true with plugins=grpc. protoSource is
// the originating .proto path, emitted as a standalone
// `def proto_source(), do: "..."` function on the .Service module only (never
// on .Stub) when non-empty (gen_proto_source=true): unlike message modules,
// GRPC.Service has no proto_source option key, so this can't be a use-option.
func RenderService(
	svc *descriptorpb.ServiceDescriptorProto,
	serviceModName string,
	stubModName string,
	fullName string,
	includeDocs bool,
	docComment string,
	genDescriptors bool,
	protoSource string,
	types *TypeRegistry,
) (string, error) {
	serviceModule, stubModule, err := RenderServiceModules(svc, serviceModName, stubModName, fullName, includeDocs, docComment, genDescriptors, protoSource, types)
	if err != nil {
		return "", err
	}

	return serviceModule + "\n\n" + stubModule, nil
}

// RenderServiceModules renders the paired .Service/.Stub module bodies as two
// independent strings, for callers (generator.go's one_file_per_module=true
// path) that need each module's own text on its own rather than joined by
// RenderService's "\n\n" separator. Splitting RenderService's combined output
// back apart with strings.Cut is NOT safe: the .Service module body itself
// contains internal blank lines (e.g. right after "@moduledoc false", before
// "use GRPC.Service") that appear before the true Service/Stub boundary, so
// this is the only correct way to get both texts separately.
func RenderServiceModules(
	svc *descriptorpb.ServiceDescriptorProto,
	serviceModName string,
	stubModName string,
	fullName string,
	includeDocs bool,
	docComment string,
	genDescriptors bool,
	protoSource string,
	types *TypeRegistry,
) (string, string, error) {
	if err := ValidateProtoName(svc.GetName()); err != nil {
		return "", "", err
	}

	serviceModule, err := renderServiceModule(svc, serviceModName, fullName, includeDocs, docComment, genDescriptors, protoSource, types)
	if err != nil {
		return "", "", err
	}

	return serviceModule, renderStubModule(stubModName, serviceModName, includeDocs), nil
}

func renderServiceModule(
	svc *descriptorpb.ServiceDescriptorProto,
	modName string,
	fullName string,
	includeDocs bool,
	docComment string,
	genDescriptors bool,
	protoSource string,
	types *TypeRegistry,
) (string, error) {
	var b strings.Builder

	fmt.Fprintf(&b, "defmodule %s do\n", modName)

	switch {
	case !includeDocs:
		b.WriteString("  @moduledoc false\n\n")
	case docComment != "":
		fmt.Fprintf(&b, "  @moduledoc \"\"\"\n%s\n  \"\"\"\n\n", indentDocComment(docComment, 2))
	}

	b.WriteString(RenderUseProtobufService(2, fullName))
	b.WriteString("\n")

	if protoSource != "" {
		fmt.Fprintf(&b, "\n  def proto_source(), do: %q\n", protoSource)
	}

	if genDescriptors {
		b.WriteString("\n")
		b.WriteString(RenderServiceDescriptor(svc))
		b.WriteString("\n")
	}

	methods := svc.GetMethod()
	if len(methods) > 0 {
		rpcLines := make([]string, len(methods))
		for i, method := range methods {
			rendered, err := renderRPC(method, types)
			if err != nil {
				return "", err
			}
			rpcLines[i] = rendered
		}
		b.WriteString("\n")
		b.WriteString(strings.Join(rpcLines, "\n"))
	}

	b.WriteString("\nend")

	return b.String(), nil
}

// renderStubModule renders the paired `.Stub` module. The Stub module's
// @moduledoc handling is NOT the same three-way branch RenderMessage/RenderEnum use: it emits
// "@moduledoc false" only when includeDocs is false, and emits NO @moduledoc
// line at all when includeDocs is true - regardless of whether the service
// itself had a doc comment (the Stub module never repeats or re-derives the
// service's own doc comment).
func renderStubModule(stubModName, serviceModName string, includeDocs bool) string {
	var b strings.Builder

	fmt.Fprintf(&b, "defmodule %s do\n", stubModName)

	if !includeDocs {
		b.WriteString("  @moduledoc false\n\n")
	}

	fmt.Fprintf(&b, "  use GRPC.Stub, service: %s\n", serviceModName)
	b.WriteString("end")

	return b.String()
}

// RenderUseProtobufService renders the `use GRPC.Service, name: "...", protoc_gen_elixir_version: "..."`
// line. Unlike RenderUseProtobuf (used by messages/enums), this is not
// alphabetically sorted from an arbitrary option set - the golden fixture
// (testdata/golden/grpc/test/service.pb.ex) only ever shows these two keys,
// already in the shown order, which happens to also be alphabetical (name
// before protoc_gen_elixir_version), so no separate ordering evidence exists
// beyond RenderOptionsBody's existing alphabetical sort being reused here for
// consistency with every other use-option list in this codebase.
func RenderUseProtobufService(indent int, fullName string) string {
	pad := strings.Repeat(" ", indent)
	body := RenderOptionsBody([]Option{
		{Key: "name", Value: fullName},
		{Key: "protoc_gen_elixir_version", Value: protocGenElixirVersion},
	})
	return pad + "use GRPC.Service, " + body
}

// renderRPC renders a single `rpc :name, InputType, OutputType` line (or with
// stream(...) wrappers - see below). The method name is emitted verbatim as a bare atom, with NO camelization
// (evidenced by the golden fixture: proto method "test" renders as the
// lowercase atom `:test`, not `:Test`).
//
// Streaming: wrapping the resolved type name in stream(...) when
// GetClientStreaming()/GetServerStreaming() is true is implemented strictly
// from the spec's illustrative example (`rpc :ClientStream, stream(Request), Reply`
// etc.) - no fixture in the current corpus (service.proto's only rpc is
// plain unary) exercises streaming at all, so this specific wrapper
// rendering is UNVERIFIED against any golden fixture or differential run.
func renderRPC(method *descriptorpb.MethodDescriptorProto, types *TypeRegistry) (string, error) {
	if err := ValidateProtoName(method.GetName()); err != nil {
		return "", err
	}

	inputType, err := resolveRPCType(method.GetName(), method.GetInputType(), types)
	if err != nil {
		return "", err
	}
	if method.GetClientStreaming() {
		inputType = "stream(" + inputType + ")"
	}

	outputType, err := resolveRPCType(method.GetName(), method.GetOutputType(), types)
	if err != nil {
		return "", err
	}
	if method.GetServerStreaming() {
		outputType = "stream(" + outputType + ")"
	}

	return fmt.Sprintf("  rpc :%s, %s, %s", method.GetName(), inputType, outputType), nil
}

func resolveRPCType(methodName, typeName string, types *TypeRegistry) (string, error) {
	modName, ok := types.Resolve(typeName)
	if !ok {
		return "", fmt.Errorf("method %q: unresolved type reference %q", methodName, typeName)
	}
	return modName, nil
}
