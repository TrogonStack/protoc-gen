package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
)

// fieldOption is a single key/value pair in a `field :name, N, ...` call.
//
// Field options are NOT emitted alphabetically. The doc comment here
// previously claimed Protobuf.Protoc.Generator.Message hardcodes the order
// json_name, optional, repeated, map, type, default, enum, oneof, packed,
// deprecated - that description does NOT match the golden fixtures and has
// been corrected. The actual left-to-right rendered order, verified against
// testdata/golden/package_prefix/my/test/test.pb.ex, is:
//
//	[label] (optional/required/repeated, or omitted for proto3
//	  non-repeated, or omitted entirely for proto3_optional),
//	type,
//	[json_name],
//	[default],
//	[enum: true],
//	[oneof: N],
//	[packed],
//	[deprecated],
//	[map: true]
//
// Evidence: `hat` has default before enum; `today` has enum before oneof;
// `temp_c` has json_name before oneof; `name_mapping`/int32_map etc. have
// map after json_name and type (NOT before type, contradicting the old doc
// comment). No fixture combines map with default/enum/oneof/packed/
// deprecated in the same field, so map's exact position relative to those
// specifically is unproven by the corpus - it's implemented as
// trailing-most (after deprecated) as the best-evidenced hypothesis.
//
// "label"/"type" are rendered directly in RenderField's head string rather
// than as fieldOption values; fieldOptions builds the remaining applicable
// keys in the order above.
type fieldOption struct {
	Key   string
	Value string
}

// typeAtom maps a FieldDescriptorProto_Type to its Elixir type atom name
// (without the leading ":"). For
// TYPE_MESSAGE and TYPE_ENUM fields, callers do not use this atom directly -
// see RenderField, which substitutes the cross-referenced module name
// instead (TYPE_GROUP is the one exception: it keeps the bare :group atom
// even though the descriptor's TypeName points at a real nested message).
func typeAtom(t descriptorpb.FieldDescriptorProto_Type) string {
	switch t {
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return "double"
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return "float"
	case descriptorpb.FieldDescriptorProto_TYPE_INT64:
		return "int64"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		return "uint64"
	case descriptorpb.FieldDescriptorProto_TYPE_INT32:
		return "int32"
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		return "fixed64"
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		return "fixed32"
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "bool"
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return "string"
	case descriptorpb.FieldDescriptorProto_TYPE_GROUP:
		return "group"
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		return "message"
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "bytes"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		return "uint32"
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		return "enum"
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		return "sfixed32"
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return "sfixed64"
	case descriptorpb.FieldDescriptorProto_TYPE_SINT32:
		return "sint32"
	case descriptorpb.FieldDescriptorProto_TYPE_SINT64:
		return "sint64"
	default:
		panic(fmt.Sprintf("unsupported field type %v", t))
	}
}

// needsTypeResolution reports whether t is a field type whose rendered
// `type:` value must be substituted with a cross-referenced Elixir module
// name (TYPE_MESSAGE, TYPE_ENUM), as opposed to a bare type atom. TYPE_GROUP
// is deliberately excluded even though its TypeName also points at a real
// message: group fields always render the bare :group atom (groups have no
// special structure).
func needsTypeResolution(t descriptorpb.FieldDescriptorProto_Type) bool {
	switch t {
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
		descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		return true
	default:
		return false
	}
}

// intDefaultTypes are the FieldDescriptorProto_Type values whose
// default_value is rendered as a decimal integer literal (Integer.parse/1 in
// the escript).
var intDefaultTypes = map[descriptorpb.FieldDescriptorProto_Type]bool{
	descriptorpb.FieldDescriptorProto_TYPE_INT64:    true,
	descriptorpb.FieldDescriptorProto_TYPE_UINT64:   true,
	descriptorpb.FieldDescriptorProto_TYPE_INT32:    true,
	descriptorpb.FieldDescriptorProto_TYPE_FIXED64:  true,
	descriptorpb.FieldDescriptorProto_TYPE_FIXED32:  true,
	descriptorpb.FieldDescriptorProto_TYPE_UINT32:   true,
	descriptorpb.FieldDescriptorProto_TYPE_SFIXED32: true,
	descriptorpb.FieldDescriptorProto_TYPE_SFIXED64: true,
	descriptorpb.FieldDescriptorProto_TYPE_SINT32:   true,
	descriptorpb.FieldDescriptorProto_TYPE_SINT64:   true,
}

var floatDefaultTypes = map[descriptorpb.FieldDescriptorProto_Type]bool{
	descriptorpb.FieldDescriptorProto_TYPE_DOUBLE: true,
	descriptorpb.FieldDescriptorProto_TYPE_FLOAT:  true,
}

// elixirFloatPattern matches the exact grammar accepted by Elixir's
// Float.parse/1: optional sign, digit+, optional "." digit+, optional
// (e|E) optional-sign digit+. Notably narrower than Go's strconv.ParseFloat,
// which also accepts "inf"/"nan"/hex-float syntax that Elixir's Float.parse/1
// rejects (verified empirically: Float.parse("inf") == :error). The escript
// requires a full-string match ({float, ""} in cast_default_value/2's case),
// so this must anchor both ends.
var elixirFloatPattern = regexp.MustCompile(`^[+-]?[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// renderDefaultValue mirrors Message.cast_default_value/2 + add_default_value_to_opts:
// integer types parse as decimal literals, float/double types parse as
// floats but fall back to the raw string for non-numeric values like "inf"
// or "nan", bool parses "true"/"false" literally, string/bytes are emitted
// verbatim as Elixir string literals, and enum defaults become bare atoms.
// The returned bool reports whether a default should be emitted at all
// (mirrors default_value in [nil, ""] short-circuiting to no-op).
func renderDefaultValue(t descriptorpb.FieldDescriptorProto_Type, defaultValue string) (string, bool) {
	if defaultValue == "" {
		return "", false
	}

	switch {
	case t == descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		switch defaultValue {
		case "true":
			return "true", true
		case "false":
			return "false", true
		default:
			return "", false
		}
	case t == descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		return ":" + defaultValue, true
	case t == descriptorpb.FieldDescriptorProto_TYPE_STRING || t == descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return fmt.Sprintf("%q", defaultValue), true
	case intDefaultTypes[t]:
		// NOTE: unlike extension-range rendering's `extensions [...]` line
		// (see RenderMessage), this does NOT apply Elixir's underscore
		// digit-grouping (e.g. 2_147_483_647) to the emitted literal. No
		// fixture in the corpus has a default value large enough to trigger
		// mix format's grouping behavior, so it's left unimplemented here
		// rather than guessed at - see groupIntegerDigits in util.go, which
		// is scoped strictly to extension ranges for this reason.
		if _, err := strconv.ParseInt(defaultValue, 10, 64); err == nil {
			return defaultValue, true
		}
		// Unsigned 64-bit values (e.g. uint64 defaults above math.MaxInt64)
		// don't fit ParseInt but are still valid decimal literals.
		if _, err := strconv.ParseUint(defaultValue, 10, 64); err == nil {
			return defaultValue, true
		}
		return "", false
	case floatDefaultTypes[t]:
		if elixirFloatPattern.MatchString(defaultValue) {
			return defaultValue, true
		}
		// Non-numeric float defaults (e.g. "inf", "-inf", "nan") are emitted
		// as a quoted string literal, matching Float.parse/1's :error
		// fallback in cast_default_value/2 (inspect/1 quotes the raw value).
		return fmt.Sprintf("%q", defaultValue), true
	default:
		return "", false
	}
}

// labelKeyword mirrors Message.label_name/1: maps a FieldDescriptorProto_Label
// to the Elixir option key used for proto2/repeated labels ("optional",
// "required", "repeated"). Proto3 non-repeated fields never reach the
// call site that uses this (see RenderField).
func labelKeyword(label descriptorpb.FieldDescriptorProto_Label) string {
	switch label {
	case descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL:
		return "optional"
	case descriptorpb.FieldDescriptorProto_LABEL_REQUIRED:
		return "required"
	case descriptorpb.FieldDescriptorProto_LABEL_REPEATED:
		return "repeated"
	default:
		panic(fmt.Sprintf("unsupported field label %v", label))
	}
}

// OneofContext carries the information RenderField needs about a message's
// oneof_decl list to decide whether a field's OneofIndex should produce a
// trailing `oneof: N` field-option: which raw 0-based indices refer to REAL
// oneofs (name not starting with "_" - synthetic proto3-optional oneofs are
// excluded from this set entirely).
type OneofContext struct {
	realOneofIndex map[int32]bool
}

// NewOneofContext builds an OneofContext from a message's oneof_decl list.
func NewOneofContext(oneofDecls []*descriptorpb.OneofDescriptorProto) OneofContext {
	real := make(map[int32]bool, len(oneofDecls))
	for i, decl := range oneofDecls {
		if !strings.HasPrefix(decl.GetName(), "_") {
			real[int32(i)] = true
		}
	}
	return OneofContext{realOneofIndex: real}
}

// isReal reports whether oneofIndex refers to a real (non-synthetic) oneof.
func (c OneofContext) isReal(oneofIndex int32) bool {
	return c.realOneofIndex[oneofIndex]
}

// RenderField renders a single "field :name, N, ..." call body (without the
// leading "field " keyword or trailing newline). syntax is the file's syntax string ("proto2",
// "proto3", or "" which behaves like proto2 for label purposes since only
// proto3 suppresses the label key on non-repeated fields).
//
// types resolves TYPE_MESSAGE/TYPE_ENUM field types (and map-entry value
// types) to their Elixir module name. oneofCtx identifies which OneofIndex
// values refer to real (non-synthetic) oneofs.
func RenderField(field *descriptorpb.FieldDescriptorProto, syntax string, types *TypeRegistry, oneofCtx OneofContext) (string, error) {
	if err := ValidateProtoName(field.GetName()); err != nil {
		return "", err
	}

	head := fmt.Sprintf(":%s, %d", field.GetName(), field.GetNumber())

	var opts []fieldOption

	switch {
	case field.GetProto3Optional():
		opts = append(opts, fieldOption{"proto3_optional", "true"})
	case syntax != "proto3" || field.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED:
		opts = append(opts, fieldOption{labelKeyword(field.GetLabel()), "true"})
	}

	typeValue, err := renderFieldTypeValue(field, types)
	if err != nil {
		return "", err
	}
	opts = append(opts, fieldOption{"type", typeValue})

	opts = append(opts, fieldOptions(field, oneofCtx)...)

	if types.IsMapField(field) {
		opts = append(opts, fieldOption{"map", "true"})
	}

	return renderFieldCall(head, opts), nil
}

// renderFieldTypeValue computes the "type:" value for field: a bare scalar
// atom (:int32, :group, ...), or - for TYPE_MESSAGE/TYPE_ENUM fields - the
// referenced type's Elixir module name (unquoted, no leading colon),
// resolved via types. Map fields resolve to their synthesized entry
// message's module name, same as any other TYPE_MESSAGE field - the map-ness
// only affects the trailing `map: true` option (see TypeRegistry.IsMapField).
func renderFieldTypeValue(field *descriptorpb.FieldDescriptorProto, types *TypeRegistry) (string, error) {
	if !needsTypeResolution(field.GetType()) {
		return ":" + typeAtom(field.GetType()), nil
	}

	modName, ok := types.Resolve(field.GetTypeName())
	if !ok {
		return "", fmt.Errorf("field %q: unresolved type reference %q", field.GetName(), field.GetTypeName())
	}
	return modName, nil
}

// fieldOptions computes the ordered (per the hardcoded weight order
// documented on fieldOption) trailing options for a field, i.e. everything
// after "type: <value>" in the field call: json_name, default, enum, oneof,
// packed, deprecated, map. json_name/default/enum/oneof/packed/map are each
// gated on their own explicit presence; deprecated is gated on FieldOptions
// being non-nil at all (see the comment at its call site below for the
// evidence - it's the one exception to "presence-gated" among this list).
func fieldOptions(field *descriptorpb.FieldDescriptorProto, oneofCtx OneofContext) []fieldOption {
	var opts []fieldOption

	if field.JsonName != nil && field.GetJsonName() != field.GetName() {
		opts = append(opts, fieldOption{"json_name", fmt.Sprintf("%q", field.GetJsonName())})
	}

	if defaultStr, ok := renderDefaultValue(field.GetType(), field.GetDefaultValue()); ok {
		opts = append(opts, fieldOption{"default", defaultStr})
	}

	if field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
		opts = append(opts, fieldOption{"enum", "true"})
	}

	if field.OneofIndex != nil && !field.GetProto3Optional() && oneofCtx.isReal(field.GetOneofIndex()) {
		opts = append(opts, fieldOption{"oneof", strconv.FormatInt(int64(field.GetOneofIndex()), 10)})
	}

	if fieldOpts := field.GetOptions(); fieldOpts != nil {
		if fieldOpts.Packed != nil {
			opts = append(opts, fieldOption{"packed", strconv.FormatBool(fieldOpts.GetPacked())})
		}
		// Unlike packed (gated on its own explicit presence), deprecated is
		// emitted whenever FieldOptions is non-nil at all, using its
		// effective (default-including) value - NOT gated on
		// fieldOpts.Deprecated's own explicit presence. Evidenced by
		// testdata/golden/package_prefix/my/test/test.pb.ex's
		// Reply.compact_keys: the real descriptor
		// (testdata/proto/test.proto's `[packed = true]`) only sets
		// options.packed, yet the golden fixture still renders
		// `deprecated: false` for that field. Corroborated in the other
		// direction by Options.opt1 ([deprecated = true], no packed set at
		// all in the descriptor): its rendered field has no `packed` key,
		// confirming packed keeps its own independent gate.
		opts = append(opts, fieldOption{"deprecated", strconv.FormatBool(fieldOpts.GetDeprecated())})
	}

	return opts
}

// fieldCallLineThreshold matches useProtobufLineThreshold: both are governed
// by the same Code.format_string!/2 default line length (98 chars).
const fieldCallLineThreshold = useProtobufLineThreshold

// renderFieldCall joins head (the ":name, N" prefix, without a leading
// "field " keyword) with opts - which includes the label/type pseudo-options
// as its first one or two entries, per RenderField - choosing single-line or
// wrapped form based on whether "  field <head>, <opts>" fits within
// fieldCallLineThreshold. Mirrors RenderUseProtobuf's wrap decision: when
// wrapped, EVERY option (including label/type) becomes its own indented
// line, matching the golden compact_keys example where "repeated: true" and
// "type: :int32" both move to their own lines alongside json_name/packed/deprecated.
func renderFieldCall(head string, opts []fieldOption) string {
	if len(opts) == 0 {
		return "  field " + head
	}

	rendered := make([]string, len(opts))
	for i, opt := range opts {
		rendered[i] = opt.Key + ": " + opt.Value
	}
	singleLine := "  field " + head + ", " + strings.Join(rendered, ", ")

	if len(singleLine) <= fieldCallLineThreshold {
		return singleLine
	}

	optPad := strings.Repeat(" ", 4)
	optLines := make([]string, len(rendered))
	for i, r := range rendered {
		optLines[i] = optPad + r
	}
	return "  field " + head + ",\n" + strings.Join(optLines, ",\n")
}

// joinFieldLines joins already-rendered "  field ..." lines the way the
// golden fixture does: consecutive single-line field calls are packed with
// no blank line between them, but a blank line is inserted on either side of
// any wrapped (multi-line) field call - matching Request's name_mapping /
// msg_mapping map fields and Reply's compact_keys, which each get a blank
// line separating them from their single-line neighbors.
func joinFieldLines(lines []string) string {
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			if strings.Contains(lines[i-1], "\n") || strings.Contains(line, "\n") {
				b.WriteString("\n\n")
			} else {
				b.WriteString("\n")
			}
		}
		b.WriteString(line)
	}
	return b.String()
}

// RenderMessage mirrors priv/templates/message.ex.eex + Protobuf.Protoc.Generator.Message
// at the pinned escript HEAD, rendering a single message module's body:
// oneof declarations (if any real oneofs are present), field declarations,
// then an `extensions [...]` line (if msg has any extension ranges). It
// renders only the message's own body - nested messages/enums, and any
// message-embedded PbExtension submodule (see extension.go's
// RenderMessageExtension), are rendered as separate, independent module
// strings by the caller (see generator.go's recursive walk), never inlined
// here.
//
// types resolves message/enum-typed field types and map-field detection.
// genDescriptors, when true, emits a `def descriptor do ... end` function
// rendering the message's own DescriptorProto as an Elixir struct literal -
// see descriptor.go. Placement (first body section, right after `use
// Protobuf`, before oneof/field/extension-range sections) is UNEVIDENCED: the
// only gen_descriptors=true message fixture (MessageWithCustomOptions, in
// testdata/golden/gen_descriptors/test/custom_options.pb.ex) has no fields,
// oneofs, or extension ranges to be ordered relative to, so this matches the
// enum case (which IS evidenced) as the best available hypothesis rather than
// an independently proven placement. protoSource is the originating .proto path, embedded as a
// proto_source: use-option when non-empty (gen_proto_source=true) - fully
// specified now, so implemented here rather than deferred.
//
// transformModule, when non-nil (transform_module=... was set), emits a
// trailing `def transform_module(), do: <value>` body section - the LAST
// section, after fields and before any `extensions [...]` line. Verified
// byte-for-byte against testdata/golden/transform_module/test/test.pb.ex:
// Test.Reply (fields + extension ranges) shows fields, blank line, `def
// transform_module()`, blank line, `extensions [...]`; Test.ReplyExtensions
// (no fields, no extension ranges - a message body that would otherwise be
// completely empty) shows it as the ONLY body section, directly after `use
// Protobuf,`; Test.Request.NameMappingEntry (a synthetic map-entry message)
// gets it too, after its key/value fields. The gate is `!= nil` rather than
// `!= ""` (unlike protoSource's `!= ""` gate) because transform_module=""
// is a valid, non-rejected explicit value per the empty-value handling rule
// - a nil pointer (flag not passed at all) must still emit
// nothing.
func RenderMessage(
	msg *descriptorpb.DescriptorProto,
	modName string,
	fullName string,
	syntax string,
	includeDocs bool,
	docComment string,
	genDescriptors bool,
	protoSource string,
	transformModule *string,
	types *TypeRegistry,
) (string, error) {
	if err := ValidateProtoName(msg.GetName()); err != nil {
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

	useOpts := []Option{
		{Key: "full_name", Value: fullName},
		{Key: "protoc_gen_elixir_version", Value: protocGenElixirVersion},
		{Key: "syntax", Value: syntaxAtom},
	}
	if opts := msg.GetOptions(); opts != nil {
		if opts.GetMapEntry() {
			useOpts = append(useOpts, Option{Key: "map", Value: true})
		}
		if opts.GetDeprecated() {
			useOpts = append(useOpts, Option{Key: "deprecated", Value: true})
		}
	}
	if protoSource != "" {
		useOpts = append(useOpts, Option{Key: "proto_source", Value: protoSource})
	}

	b.WriteString(RenderUseProtobuf(2, useOpts))

	oneofCtx := NewOneofContext(msg.GetOneofDecl())

	var bodySections []string

	if genDescriptors {
		bodySections = append(bodySections, RenderMessageDescriptor(msg))
	}

	if oneofLines := renderOneofDecls(msg.GetOneofDecl()); oneofLines != "" {
		bodySections = append(bodySections, oneofLines)
	}

	fields := msg.GetField()
	if len(fields) > 0 {
		renderedFields := make([]string, len(fields))
		for i, field := range fields {
			rendered, err := RenderField(field, syntax, types, oneofCtx)
			if err != nil {
				return "", err
			}
			renderedFields[i] = rendered
		}
		bodySections = append(bodySections, joinFieldLines(renderedFields))
	}

	if transformModule != nil {
		bodySections = append(bodySections, renderTransformModule(*transformModule))
	}

	if extRanges := msg.GetExtensionRange(); len(extRanges) > 0 {
		bodySections = append(bodySections, renderExtensionRanges(extRanges))
	}

	if len(bodySections) > 0 {
		b.WriteString("\n\n")
		b.WriteString(strings.Join(bodySections, "\n\n"))
	}

	b.WriteString("\nend")

	return b.String(), nil
}

// renderOneofDecls renders `oneof :name, N` lines (one per REAL oneof, name
// not starting with "_"), in oneof_decl order, joined by a single newline
// (no blank lines between them - the blank-line separation from the
// following field declarations is handled by the caller, RenderMessage,
// treating this as one body section). N is the RAW 0-based index in
// oneof_decl, even counting any synthetic "_"-prefixed oneofs that precede
// it positionally. Returns "" when there are no real oneofs at all
// (including when oneof_decl is empty or contains only synthetic entries).
func renderOneofDecls(oneofDecls []*descriptorpb.OneofDescriptorProto) string {
	var lines []string
	for i, decl := range oneofDecls {
		if strings.HasPrefix(decl.GetName(), "_") {
			continue
		}
		lines = append(lines, fmt.Sprintf("  oneof :%s, %d", decl.GetName(), i))
	}
	return strings.Join(lines, "\n")
}

// renderTransformModule renders the `def transform_module(), do: <value>`
// body-section line for a message module. value is emitted verbatim, unquoted (a bare Elixir module
// reference, e.g. My.App.Transform - NOT a string, NOT an atom with a
// leading colon), matching Module.concat([value]) rendered via inspect/1 for
// a non-empty module name.
//
// The empty-string case (transform_module="") is NOT rejected by param
// parsing (see params.go), but no fixture in the corpus exercises what
// Module.concat([""]) actually renders as via inspect/1 in this position -
// this simply emits the empty string verbatim (`do: `) so it doesn't crash,
// which is a reasonable but UNVERIFIED guess, not a fixture-proven
// rendering. Do not treat this branch as confirmed correct.
func renderTransformModule(value string) string {
	return "  def transform_module(), do: " + value
}

// extensionRangeMaxEnd is the descriptor End value ordinary "extensions ...
// to max;" declarations compile to (0x20000000 / 2^29). It renders as the
// Protobuf.Extension.max() sentinel rather than the raw integer. A message
// with option message_set_wire_format = true changes what "max" means at the
// protoc/descriptor level - its extension range's End is 2147483647
// (2^31 - 1, INT32_MAX) instead, which is NOT this sentinel value and
// therefore renders as a raw (underscore-grouped) integer literal. Evidenced
// by testdata/golden/package_prefix/my/test/test.pb.ex: Reply's `extensions
// 100 to max;` renders `{100, Protobuf.Extension.max()}`, while OldReply's
// `extensions 100 to max;` (with message_set_wire_format = true) renders
// `{100, 2_147_483_647}`.
const extensionRangeMaxEnd = 0x20000000

// renderExtensionRanges renders the `extensions [...]` body-section line for
// a message's ExtensionRange list, in declaration order. Each range's Start/End are used
// directly from the descriptor with no further adjustment (End is already
// protoc's own exclusive convention).
func renderExtensionRanges(ranges []*descriptorpb.DescriptorProto_ExtensionRange) string {
	tuples := make([]string, len(ranges))
	for i, r := range ranges {
		end := "Protobuf.Extension.max()"
		if r.GetEnd() != extensionRangeMaxEnd {
			end = groupIntegerDigits(r.GetEnd())
		}
		tuples[i] = fmt.Sprintf("{%d, %s}", r.GetStart(), end)
	}
	return "  extensions [" + strings.Join(tuples, ", ") + "]"
}
