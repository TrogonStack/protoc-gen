package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
)

// RenderEnum mirrors priv/templates/enum.ex.eex + Protobuf.Protoc.Generator.Enum
// at the pinned escript HEAD, rendering a single top-level enum module.
//
// genDescriptors, when true, emits a `def descriptor do ... end` function
// right after `use Protobuf` (blank-line separated on both sides), rendering
// the enum's own EnumDescriptorProto as an Elixir struct literal - see
// descriptor.go. Placement here is EVIDENCED byte-for-byte against
// testdata/golden/gen_descriptors/test/custom_options.pb.ex.
func RenderEnum(
	enum *descriptorpb.EnumDescriptorProto,
	modName string,
	fullName string,
	syntax string,
	includeDocs bool,
	docComment string,
	genDescriptors bool,
) (string, error) {
	if err := ValidateProtoName(enum.GetName()); err != nil {
		return "", err
	}

	var b strings.Builder

	fmt.Fprintf(&b, "defmodule %s do\n", modName)

	switch {
	case !includeDocs:
		b.WriteString("  @moduledoc false\n\n")
	case docComment != "":
		fmt.Fprintf(&b, "  @moduledoc \"\"\"\n%s\n  \"\"\"\n\n", indentDocComment(docComment, 2))
	}

	syntaxAtom := Atom("proto2")
	if syntax != "" {
		syntaxAtom = Atom(syntax)
	}

	b.WriteString(RenderUseProtobuf(2, []Option{
		{Key: "enum", Value: true},
		{Key: "full_name", Value: fullName},
		{Key: "protoc_gen_elixir_version", Value: protocGenElixirVersion},
		{Key: "syntax", Value: syntaxAtom},
	}))
	b.WriteString("\n\n")

	if genDescriptors {
		b.WriteString(RenderEnumDescriptor(enum))
		b.WriteString("\n\n")
	}

	for _, value := range enum.GetValue() {
		if err := ValidateProtoName(value.GetName()); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "  field :%s, %d\n", value.GetName(), value.GetNumber())
	}

	b.WriteString("end")

	return b.String(), nil
}

// indentDocComment prefixes each non-blank line of comment with indent
// spaces, matching Code.format_string!/2's treatment of @moduledoc heredoc
// bodies: blank lines stay truly empty, never gaining trailing whitespace.
func indentDocComment(comment string, indent int) string {
	pad := strings.Repeat(" ", indent)
	lines := strings.Split(comment, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}
