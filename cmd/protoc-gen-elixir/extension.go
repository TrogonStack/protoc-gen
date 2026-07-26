package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
)

// RenderMessageExtension renders a message's deferred `<Message>.PbExtension`
// submodule (the message-embedded case), merging ALL of msg's own embedded
// extend fields (msg.GetExtension()) -
// regardless of which message(s) they extend - into one module, in the same
// declaration order they appear in the source. Returns ("", false) when msg
// has no embedded extend fields at all, so callers can skip emitting it.
//
// "use Protobuf" options on this module are
// protoc_gen_elixir_version + syntax (from the file's own syntax, since a
// message-embedded extend block is scoped to a single file) but explicitly
// NO full_name: - confirmed against
// testdata/golden/package_prefix/my/test/test.pb.ex's
// My.Test.ReplyExtensions.PbExtension.
//
// This submodule's rendered position within the file is NOT alongside msg
// (see generator.go's deferred rendering pass) - RenderMessageExtension only
// renders the module's own text, the caller is responsible for placement.
//
// includeDocs gates "@moduledoc false" exactly like RenderMessage/RenderEnum's
// two false-case branch (never the doc-comment branch: this synthesized
// submodule has no single SourceCodeInfo location of its own to extract a
// comment from, so under includeDocs=true it always lands in the "no
// @moduledoc line at all" case) - confirmed against
// testdata/golden/grpc_proto_source/test/test.pb.ex's
// Test.ReplyExtensions.PbExtension, which shows "@moduledoc false"
// (include_docs defaults to false for that fixture).
//
// types resolves each extend field's Extendee (a leading-dot fully-qualified
// proto name) to its Elixir module name, and each field's own type, via the
// same TypeRegistry every ordinary field uses.
func RenderMessageExtension(msg *descriptorpb.DescriptorProto, modName string, syntax string, includeDocs bool, types *TypeRegistry) (string, bool, error) {
	fields := msg.GetExtension()
	if len(fields) == 0 {
		return "", false, nil
	}

	text, err := renderExtensionModule(fields, modName, syntax, includeDocs, types)
	if err != nil {
		return "", false, err
	}
	return text, true, nil
}

// RenderFileExtension renders the top-level, file-merged `<Base>.PbExtension`
// module (the file-level case) from a group of top-level extend fields
// collected across one or more files
// sharing the same computed base Elixir module name (see generator.go's
// second pass in generateFiles). Returns ("", false) when fields is empty,
// so callers can skip emitting the module entirely (mirrors the rule to never
// emit an extension module with zero extend lines, applied here as
// "don't produce the output" rather than erroring).
//
// Unlike RenderMessageExtension, this module's "use Protobuf" line has
// NEITHER full_name: NOR syntax: - confirmed identically across four golden
// variants (package_prefix, grpc, grpc_proto_source, transform_module).
// Presumably because this module can merge extend blocks from multiple files
// that might disagree on syntax, so no single unambiguous syntax value is
// ever emitted.
//
// This module's @moduledoc follows the same three-way branch
// RenderMessage/RenderEnum use, but this synthesized module never has its
// own single SourceCodeInfo location to extract a doc comment from (it can
// merge extend blocks from multiple files) - in practice docComment is
// always "" here, so under includeDocs=true it always lands in the "no
// @moduledoc line at all" branch. Confirmed: package_prefix's variant
// (include_docs=true) has no @moduledoc line; grpc_proto_source's and
// transform_module's variants (no include_docs) both show @moduledoc false.
func RenderFileExtension(fields []*descriptorpb.FieldDescriptorProto, modName string, includeDocs bool, types *TypeRegistry) (string, bool, error) {
	if len(fields) == 0 {
		return "", false, nil
	}

	var b strings.Builder

	fmt.Fprintf(&b, "defmodule %s do\n", modName)

	if !includeDocs {
		b.WriteString("  @moduledoc false\n\n")
	}

	b.WriteString(RenderUseProtobuf(2, []Option{
		{Key: "protoc_gen_elixir_version", Value: protocGenElixirVersion},
	}))
	b.WriteString("\n")

	lines, err := renderExtendLines(fields, types)
	if err != nil {
		return "", false, err
	}
	b.WriteString("\n")
	b.WriteString(lines)

	b.WriteString("\nend")

	return b.String(), true, nil
}

// renderExtensionModule renders a message-embedded PbExtension module body -
// shared by RenderMessageExtension. Unlike RenderFileExtension, this always
// includes syntax: (never full_name:). includeDocs gates "@moduledoc false"
// - see RenderMessageExtension's doc comment.
func renderExtensionModule(fields []*descriptorpb.FieldDescriptorProto, modName string, syntax string, includeDocs bool, types *TypeRegistry) (string, error) {
	var b strings.Builder

	fmt.Fprintf(&b, "defmodule %s do\n", modName)

	if !includeDocs {
		b.WriteString("  @moduledoc false\n\n")
	}

	syntaxAtom := Atom("proto2")
	if syntax != "" {
		syntaxAtom = Atom(syntax)
	}

	b.WriteString(RenderUseProtobuf(2, []Option{
		{Key: "protoc_gen_elixir_version", Value: protocGenElixirVersion},
		{Key: "syntax", Value: syntaxAtom},
	}))
	b.WriteString("\n")

	lines, err := renderExtendLines(fields, types)
	if err != nil {
		return "", err
	}
	b.WriteString("\n")
	b.WriteString(lines)

	b.WriteString("\nend")

	return b.String(), nil
}

// renderExtendLines renders one `extend <Extendee>, :name, N, ...` line per
// field, joined with blank lines between every line (unlike joinFieldLines'
// ordinary-field packing, every extend line in the corpus is separated by a
// blank line - see My.Test.ReplyExtensions.PbExtension and
// My.Test.PbExtension, which both blank-line-separate every extend, even
// consecutive single-line ones).
func renderExtendLines(fields []*descriptorpb.FieldDescriptorProto, types *TypeRegistry) (string, error) {
	lines := make([]string, len(fields))
	for i, field := range fields {
		line, err := renderExtendLine(field, types)
		if err != nil {
			return "", err
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n\n"), nil
}

// renderExtendLine renders a single `extend <ResolvedExtendeeModule>, :name,
// N, ...` line. Extend fields are ordinary FieldDescriptorProto values with
// an extra Extendee string set; they never carry OneofIndex or map-ness in
// practice, so reusing needsTypeResolution/renderFieldTypeValue/fieldOptions
// with an empty OneofContext{} is safe and correct - no changes to those
// helpers are needed. The wrap-threshold/line-join machinery
// (renderFieldCall) is reused verbatim, even though no fixture in the
// corpus actually triggers a wrap for an extend line (every extend line
// evidenced is well under fieldCallLineThreshold).
func renderExtendLine(field *descriptorpb.FieldDescriptorProto, types *TypeRegistry) (string, error) {
	if err := ValidateProtoName(field.GetName()); err != nil {
		return "", err
	}

	extendeeMod, ok := types.Resolve(field.GetExtendee())
	if !ok {
		return "", fmt.Errorf("field %q: unresolved extendee reference %q", field.GetName(), field.GetExtendee())
	}

	head := fmt.Sprintf("%s, :%s, %d", extendeeMod, field.GetName(), field.GetNumber())

	var opts []fieldOption

	opts = append(opts, fieldOption{labelKeyword(field.GetLabel()), "true"})

	typeValue, err := renderFieldTypeValue(field, types)
	if err != nil {
		return "", err
	}
	opts = append(opts, fieldOption{"type", typeValue})

	opts = append(opts, fieldOptions(field, OneofContext{})...)

	return renderExtendCall(head, opts), nil
}

// renderExtendCall mirrors renderFieldCall's single-line/wrapped decision,
// but with a leading "extend " keyword instead of "field " (an extend line
// starts with the resolved extendee module rather than a bare field name).
func renderExtendCall(head string, opts []fieldOption) string {
	rendered := make([]string, len(opts))
	for i, opt := range opts {
		rendered[i] = opt.Key + ": " + opt.Value
	}
	singleLine := "  extend " + head + ", " + strings.Join(rendered, ", ")

	if len(singleLine) <= fieldCallLineThreshold {
		return singleLine
	}

	optPad := strings.Repeat(" ", 4)
	optLines := make([]string, len(rendered))
	for i, r := range rendered {
		optLines[i] = optPad + r
	}
	return "  extend " + head + ",\n" + strings.Join(optLines, ",\n")
}
