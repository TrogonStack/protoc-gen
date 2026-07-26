package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/types/descriptorpb"
)

// descriptorLineThreshold matches useProtobufLineThreshold/fieldCallLineThreshold:
// all three are governed by the same Code.format_string!/2 default line
// length (98 chars). Every struct literal in the two golden fixtures
// (testdata/golden/gen_descriptors/test/custom_options.pb.ex) is multi-line
// because the guaranteed __unknown_fields__/__protobuf__ trailer alone
// trivially exceeds 98 chars once nested under any nonzero indent - so the
// true single-line boundary case (a struct literal that legitimately fits on
// one line) is UNEVIDENCED by the corpus. The wrap decision below still
// implements the same fits-on-one-line-else-wrap rule as everywhere else in
// this codebase, on the reasonable assumption that Code.format_string!/2
// applies its normal rule uniformly rather than special-casing descriptor
// bodies.
const descriptorLineThreshold = useProtobufLineThreshold

// elixirTerm is a node in the small Elixir-value IR used to pretty-print
// descriptor struct literals. Each term decides for itself whether it fits on
// one line at the given indent, or must be broken across multiple lines -
// mirroring RenderUseProtobuf/renderFieldCall's own single-line/wrapped
// decision, generalized to arbitrarily nested struct/list literals.
type elixirTerm interface {
	// render returns the term's Elixir source text. indent is the column (in
	// spaces) the term's OWN content lines start at - i.e. the indent already
	// used by whatever "key: " prefix precedes this term on its opening line.
	// widthBudget is how many characters remain on the current line before
	// the threshold is exceeded (used only to decide single-line-fits;
	// nested terms recompute their own budget once they commit to a new
	// line).
	render(indent int) string
}

// termLiteral is a pre-rendered, always-single-line Elixir value: atoms,
// booleans, strings, integers, nil. It never wraps on its own - only
// termStruct/termList decide whether to place literals one-per-line.
type termLiteral string

func (t termLiteral) render(int) string { return string(t) }

func atomTerm(name string) elixirTerm { return termLiteral(":" + name) }
func boolTerm(v bool) elixirTerm {
	if v {
		return termLiteral("true")
	}
	return termLiteral("false")
}
func nilTerm() elixirTerm { return termLiteral("nil") }
func stringTerm(s string) elixirTerm {
	return termLiteral(fmt.Sprintf("%q", s))
}
func intTerm(v int64) elixirTerm   { return termLiteral(strconv.FormatInt(v, 10)) }
func uintTerm(v uint64) elixirTerm { return termLiteral(strconv.FormatUint(v, 10)) }
func rawTerm(s string) elixirTerm  { return termLiteral(s) }

// field is a single "key: value" pair inside a termStruct, in the order it
// must be emitted (ascending proto field number per this file's hardcoded
// tables, NOT alphabetical or Go-struct-tag order - see the per-type builder
// functions below).
type field struct {
	Key   string
	Value elixirTerm
}

// termStruct renders a "%Google.Protobuf.<Type>{...}" struct literal. Fields
// render in Fields' given
// order, one "key: value" pair each, trailing-comma-joined, never a trailing
// comma after the last field.
type termStruct struct {
	TypeName string
	Fields   []field
}

func (t termStruct) render(indent int) string {
	head := "%" + t.TypeName + "{"

	// Elixir struct-literal single-line syntax is "%Type{k: v, k2: v2}" - no
	// space after "{" or before "}".
	single := head + joinFieldsSingleLine(t.Fields, indent+len(head)) + "}"

	if fitsOneLine(indent, single) {
		return single
	}

	inner := indent + 2
	pad := strings.Repeat(" ", inner)
	closePad := strings.Repeat(" ", indent)

	lines := make([]string, len(t.Fields))
	for i, f := range t.Fields {
		lines[i] = pad + f.Key + ": " + f.Value.render(inner)
	}

	return head + "\n" + strings.Join(lines, ",\n") + "\n" + closePad + "}"
}

func joinFieldsSingleLine(fields []field, indent int) string {
	rendered := make([]string, len(fields))
	for i, f := range fields {
		rendered[i] = f.Key + ": " + f.Value.render(indent)
	}
	return strings.Join(rendered, ", ")
}

// termList renders a "[...]" list literal (repeated descriptor fields).
// Empty lists always render as the literal "[]", matching every empty
// repeated field in the golden fixtures (value: [], reserved_range: [],
// etc.).
type termList struct {
	Elements []elixirTerm
}

func (t termList) render(indent int) string {
	if len(t.Elements) == 0 {
		return "[]"
	}

	single := "[" + joinElementsSingleLine(t.Elements, indent+1) + "]"
	if fitsOneLine(indent, single) {
		return single
	}

	inner := indent + 2
	pad := strings.Repeat(" ", inner)
	closePad := strings.Repeat(" ", indent)

	lines := make([]string, len(t.Elements))
	for i, el := range t.Elements {
		lines[i] = pad + el.render(inner)
	}

	return "[\n" + strings.Join(lines, ",\n") + "\n" + closePad + "]"
}

func joinElementsSingleLine(elements []elixirTerm, indent int) string {
	rendered := make([]string, len(elements))
	for i, el := range elements {
		rendered[i] = el.render(indent)
	}
	return strings.Join(rendered, ", ")
}

// termTuple renders a fixed-arity "{a, b, c}" tuple literal, used for
// __unknown_fields__ entries ("{field_number, wire_type, value}"). Unlike
// termStruct/termList, tuples in the evidenced fixtures are always short
// enough to stay single-line, so no multi-line form is implemented - see
// decodeUnknownFields' doc comment.
type termTuple struct {
	Elements []elixirTerm
}

func (t termTuple) render(indent int) string {
	return "{" + joinElementsSingleLine(t.Elements, indent+1) + "}"
}

// fitsOneLine reports whether a term's single-line rendering, placed at the
// given indent, stays within descriptorLineThreshold - mirroring
// RenderUseProtobuf/renderFieldCall's own "pad + content" width check.
func fitsOneLine(indent int, singleLine string) bool {
	return indent+len(singleLine) <= descriptorLineThreshold
}

// ---------------------------------------------------------------------------
// Unknown-field wire decoding
// ---------------------------------------------------------------------------

// unknownFieldEntry is one decoded "{field_number, wire_type, value}" tuple
// found in a message's raw unrecognized wire bytes (protoreflect.RawFields),
// in stream order. Evidenced by both golden fixtures: custom_options.proto's
// two custom extensions (my_custom_option = 50005 on EnumValueOptions,
// lowercase_name = 51300 on MessageOptions) are NOT in Go's global extension
// registry (they're declared in the test proto itself, never registered with
// protoc-gen-go), so protoc-parsed EnumValueOptions/MessageOptions values
// carry them as raw unknown bytes on the Go side too - exactly mirroring how
// the Elixir escript's own decoder sees them (also as unrecognized options,
// since Elixir never loaded a compiled extension module for them either).
type unknownFieldEntry struct {
	Number   int32
	WireType int32
	Value    elixirTerm
}

// decodeUnknownFields walks raw (a message's ProtoReflect().GetUnknown())
// and decodes it into an ordered list of (field_number, wire_type, value)
// tuples. Only wire type 2
// (length-delimited) is fixture-evidenced (both {50005, 2, "hello"} and
// {51300, 2, "message_with_custom_options"} are length-delimited string
// values) - wire types 0 (varint) and 1/5 (fixed64/fixed32) are best-effort,
// UNEVIDENCED extrapolations, called out at their own render sites below.
func decodeUnknownFields(raw []byte) []unknownFieldEntry {
	var entries []unknownFieldEntry

	for len(raw) > 0 {
		num, typ, n := protowire.ConsumeTag(raw)
		if n < 0 {
			break
		}
		raw = raw[n:]

		value, valueLen := decodeUnknownFieldValue(typ, raw)
		if valueLen < 0 {
			break
		}
		raw = raw[valueLen:]

		entries = append(entries, unknownFieldEntry{
			Number:   int32(num),
			WireType: int32(typ),
			Value:    value,
		})
	}

	return entries
}

// decodeUnknownFieldValue decodes a single field value per its wire type,
// returning the rendered Elixir term and the number of bytes consumed (-1 on
// malformed input, so the caller can stop rather than loop forever).
func decodeUnknownFieldValue(typ protowire.Type, raw []byte) (elixirTerm, int) {
	switch typ {
	case protowire.VarintType:
		v, n := protowire.ConsumeVarint(raw)
		if n < 0 {
			return nil, -1
		}
		// UNEVIDENCED: no fixture exercises a varint-typed unknown field: both
		// corpus examples are wire type 2 (length-delimited strings). Rendered
		// as a plain decimal integer, matching how every other integer
		// literal in this codebase is emitted (renderDefaultValue's int
		// branch, groupIntegerDigits' extension-range branch).
		return uintTerm(v), n
	case protowire.Fixed32Type:
		v, n := protowire.ConsumeFixed32(raw)
		if n < 0 {
			return nil, -1
		}
		// UNEVIDENCED, best-effort: rendered as a plain decimal integer of the
		// raw 32-bit pattern. No fixture exercises fixed32-typed unknown
		// fields (float/fixed32 semantics can't be recovered from the raw
		// wire bytes alone without knowing the original proto type).
		return uintTerm(uint64(v)), n
	case protowire.Fixed64Type:
		v, n := protowire.ConsumeFixed64(raw)
		if n < 0 {
			return nil, -1
		}
		// UNEVIDENCED, best-effort: see Fixed32Type above.
		return uintTerm(v), n
	case protowire.BytesType:
		v, n := protowire.ConsumeBytes(raw)
		if n < 0 {
			return nil, -1
		}
		return decodeLengthDelimitedTerm(v), n
	default:
		return nil, -1
	}
}

// decodeLengthDelimitedTerm renders a wire-type-2 field's raw bytes as an
// Elixir string literal when the bytes are printable per Elixir's
// String.printable?/1 semantics (evidenced by both fixtures: {50005, 2,
// "hello"} and {51300, 2, "message_with_custom_options"}), else as a
// "<<byte, byte, ...>>" binary literal - the non-printable branch is
// UNEVIDENCED by the corpus (no fixture attaches a non-string-typed unknown
// extension), implemented as the best-effort fallback Elixir's own inspect/1
// would produce for an unprintable binary.
func decodeLengthDelimitedTerm(raw []byte) elixirTerm {
	if isElixirPrintable(raw) {
		return stringTerm(string(raw))
	}

	parts := make([]string, len(raw))
	for i, b := range raw {
		parts[i] = strconv.Itoa(int(b))
	}
	return rawTerm("<<" + strings.Join(parts, ", ") + ">>")
}

// elixirPrintableEscapes is the set of extra ASCII control characters
// String.printable?/1 accepts alongside Unicode's own
// printable/graphic/whitespace categories: \n \r \t \v \b \f \e \d \a (per
// mirroring Elixir's own definition in String.Unicode/String.Chars).
var elixirPrintableEscapes = map[rune]bool{
	'\n':   true, // \n
	'\r':   true, // \r
	'\t':   true, // \t
	'\v':   true, // \v
	'\b':   true, // \b
	'\f':   true, // \f
	'\x1b': true, // \e
	'\x7f': true, // \d (DEL)
	'\a':   true, // \a
}

// isElixirPrintable reports whether raw is valid UTF-8 and every codepoint is
// either printable/graphic or one of elixirPrintableEscapes, per
// String.printable?/1's semantics. Unicode "printable" here is approximated as
// "printable OR graphic" per unicode.IsPrint/unicode.IsGraphic, which is the
// closest Go stdlib equivalent to Elixir's own definition.
func isElixirPrintable(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}

	s := string(raw)
	for _, r := range s {
		if elixirPrintableEscapes[r] {
			continue
		}
		if !unicode.IsPrint(r) && !unicode.IsGraphic(r) {
			return false
		}
	}
	return true
}

// unknownFieldsTerm builds the "__unknown_fields__: [...]" list term from raw
// wire bytes, shared by every descriptor-struct builder below.
func unknownFieldsTerm(raw []byte) elixirTerm {
	entries := decodeUnknownFields(raw)
	elements := make([]elixirTerm, len(entries))
	for i, e := range entries {
		elements[i] = termTuple{Elements: []elixirTerm{
			intTerm(int64(e.Number)),
			intTerm(int64(e.WireType)),
			e.Value,
		}}
	}
	return termList{Elements: elements}
}

// trailerFields appends the universal "__unknown_fields__: [...]" then
// "__protobuf__: true" pair, always in that order, always last, on every
// descriptor struct rendered (rule 3). unknown is the owning message's own raw unknown bytes (via
// ProtoReflect().GetUnknown()).
func trailerFields(fields []field, unknown []byte) []field {
	fields = append(fields, field{"__unknown_fields__", unknownFieldsTerm(unknown)})
	fields = append(fields, field{"__protobuf__", boolTerm(true)})
	return fields
}

// pbExtensionsField is "__pb_extensions__: %{}", inserted immediately before
// __unknown_fields__ on "Options"-suffixed descriptor types only (rule 4) -
// MessageOptions, FieldOptions, OneofOptions, EnumOptions, EnumValueOptions,
// ServiceOptions, MethodOptions, ExtensionRangeOptions. Always an empty map
// literal: no fixture in the corpus attaches an actual (Go-registry-known)
// extension value to any Options message, only unknown-bytes ones, so
// __pb_extensions__ is never non-empty in the evidenced corpus either.
func pbExtensionsField() field {
	return field{"__pb_extensions__", rawTerm("%{}")}
}

// ---------------------------------------------------------------------------
// Per-type struct builders (hardcoded, ascending proto field number order)
// ---------------------------------------------------------------------------
//
// Every builder below is a plain Go function, not reflection-driven: each
// hardcodes its own field list, in ascending proto field-number order. This
// is deliberate: some fields (Visibility) must be excluded regardless of
// what the linked descriptorpb package happens to carry, which a generic
// struct-tag walk would get wrong the moment upstream descriptor.proto
// gains a new field.
//
// EVIDENCED builders (byte-for-byte proven against
// testdata/golden/gen_descriptors/test/custom_options.pb.ex): DescriptorProto,
// EnumDescriptorProto, EnumValueDescriptorProto, MessageOptions,
// EnumValueOptions.
//
// Every other builder (FieldDescriptorProto, OneofDescriptorProto,
// ServiceDescriptorProto, MethodDescriptorProto, the ExtensionRange/
// ReservedRange family, FieldOptions, OneofOptions, EnumOptions,
// ServiceOptions, MethodOptions, ExtensionRangeOptions, UninterpretedOption)
// is UNVERIFIED against any golden fixture or differential run - implemented
// per the current public descriptorpb schema, ascending field-number order,
// on the same "best-evidenced hypothesis" basis this codebase already uses
// elsewhere (e.g. util.go's groupIntegerDigits, message.go's map-position
// comment).

func descriptorProtoTerm(msg *descriptorpb.DescriptorProto) elixirTerm {
	fieldTerms := make([]elixirTerm, len(msg.GetField()))
	for i, f := range msg.GetField() {
		fieldTerms[i] = fieldDescriptorProtoTerm(f)
	}

	nestedTerms := make([]elixirTerm, len(msg.GetNestedType()))
	for i, n := range msg.GetNestedType() {
		nestedTerms[i] = descriptorProtoTerm(n)
	}

	enumTerms := make([]elixirTerm, len(msg.GetEnumType()))
	for i, e := range msg.GetEnumType() {
		enumTerms[i] = enumDescriptorProtoTerm(e)
	}

	extRangeTerms := make([]elixirTerm, len(msg.GetExtensionRange()))
	for i, r := range msg.GetExtensionRange() {
		extRangeTerms[i] = extensionRangeTerm(r)
	}

	extTerms := make([]elixirTerm, len(msg.GetExtension()))
	for i, e := range msg.GetExtension() {
		extTerms[i] = fieldDescriptorProtoTerm(e)
	}

	oneofTerms := make([]elixirTerm, len(msg.GetOneofDecl()))
	for i, o := range msg.GetOneofDecl() {
		oneofTerms[i] = oneofDescriptorProtoTerm(o)
	}

	reservedRangeTerms := make([]elixirTerm, len(msg.GetReservedRange()))
	for i, r := range msg.GetReservedRange() {
		reservedRangeTerms[i] = reservedRangeTerm(r)
	}

	reservedNameTerms := make([]elixirTerm, len(msg.GetReservedName()))
	for i, n := range msg.GetReservedName() {
		reservedNameTerms[i] = stringTerm(n)
	}

	var optionsTerm = nilTerm()
	if opts := msg.GetOptions(); opts != nil {
		optionsTerm = messageOptionsTerm(opts)
	}

	fields := []field{
		{"name", stringTerm(msg.GetName())},
		{"field", termList{Elements: fieldTerms}},
		{"nested_type", termList{Elements: nestedTerms}},
		{"enum_type", termList{Elements: enumTerms}},
		{"extension_range", termList{Elements: extRangeTerms}},
		{"extension", termList{Elements: extTerms}},
		{"options", optionsTerm},
		{"oneof_decl", termList{Elements: oneofTerms}},
		{"reserved_range", termList{Elements: reservedRangeTerms}},
		{"reserved_name", termList{Elements: reservedNameTerms}},
		// visibility (field 11) is deliberately EXCLUDED - evidenced by
		// MessageWithCustomOptions rendering every other field explicitly
		// (including falsy/absent ones like map_entry: nil) yet never
		// emitting a visibility: key at all.
	}
	fields = trailerFields(fields, msg.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.DescriptorProto", Fields: fields}
}

func enumDescriptorProtoTerm(enum *descriptorpb.EnumDescriptorProto) elixirTerm {
	valueTerms := make([]elixirTerm, len(enum.GetValue()))
	for i, v := range enum.GetValue() {
		valueTerms[i] = enumValueDescriptorProtoTerm(v)
	}

	reservedRangeTerms := make([]elixirTerm, len(enum.GetReservedRange()))
	for i, r := range enum.GetReservedRange() {
		reservedRangeTerms[i] = enumReservedRangeTerm(r)
	}

	reservedNameTerms := make([]elixirTerm, len(enum.GetReservedName()))
	for i, n := range enum.GetReservedName() {
		reservedNameTerms[i] = stringTerm(n)
	}

	var optionsTerm = nilTerm()
	if opts := enum.GetOptions(); opts != nil {
		optionsTerm = enumOptionsTerm(opts)
	}

	fields := []field{
		{"name", stringTerm(enum.GetName())},
		{"value", termList{Elements: valueTerms}},
		{"options", optionsTerm},
		{"reserved_range", termList{Elements: reservedRangeTerms}},
		{"reserved_name", termList{Elements: reservedNameTerms}},
		// visibility (field 6) is deliberately EXCLUDED - see
		// descriptorProtoTerm's own comment; evidenced identically by
		// EnumWithCustomOptions.
	}
	fields = trailerFields(fields, enum.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.EnumDescriptorProto", Fields: fields}
}

func enumValueDescriptorProtoTerm(v *descriptorpb.EnumValueDescriptorProto) elixirTerm {
	var optionsTerm = nilTerm()
	if opts := v.GetOptions(); opts != nil {
		optionsTerm = enumValueOptionsTerm(opts)
	}

	fields := []field{
		{"name", stringTerm(v.GetName())},
		{"number", intTerm(int64(v.GetNumber()))},
		{"options", optionsTerm},
	}
	fields = trailerFields(fields, v.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.EnumValueDescriptorProto", Fields: fields}
}

// fieldDescriptorProtoTerm is UNVERIFIED against any golden fixture (no
// gen_descriptors=true fixture in the corpus has a message with any
// fields - MessageWithCustomOptions has none). Field order/keys follow
// FieldDescriptorProto's own proto field numbers; label/type render as
// uppercase Elixir atoms matching the real enum value names
// (FieldDescriptorProto.Label/.Type).
func fieldDescriptorProtoTerm(f *descriptorpb.FieldDescriptorProto) elixirTerm {
	var labelTerm = nilTerm()
	if f.Label != nil {
		labelTerm = atomTerm(fieldLabelAtomName(f.GetLabel()))
	}

	var typeTerm = nilTerm()
	if f.Type != nil {
		typeTerm = atomTerm(fieldTypeAtomName(f.GetType()))
	}

	var typeNameTerm = nilTerm()
	if f.TypeName != nil {
		typeNameTerm = stringTerm(f.GetTypeName())
	}

	var extendeeTerm = nilTerm()
	if f.Extendee != nil {
		extendeeTerm = stringTerm(f.GetExtendee())
	}

	var defaultTerm = nilTerm()
	if f.DefaultValue != nil {
		defaultTerm = stringTerm(f.GetDefaultValue())
	}

	var oneofIndexTerm = nilTerm()
	if f.OneofIndex != nil {
		oneofIndexTerm = intTerm(int64(f.GetOneofIndex()))
	}

	var jsonNameTerm = nilTerm()
	if f.JsonName != nil {
		jsonNameTerm = stringTerm(f.GetJsonName())
	}

	var optionsTerm = nilTerm()
	if opts := f.GetOptions(); opts != nil {
		optionsTerm = fieldOptionsTerm(opts)
	}

	var proto3OptionalTerm = nilTerm()
	if f.Proto3Optional != nil {
		proto3OptionalTerm = boolTerm(f.GetProto3Optional())
	}

	fields := []field{
		{"name", stringTerm(f.GetName())},
		{"extendee", extendeeTerm},
		{"number", intTerm(int64(f.GetNumber()))},
		{"label", labelTerm},
		{"type", typeTerm},
		{"type_name", typeNameTerm},
		{"default_value", defaultTerm},
		{"options", optionsTerm},
		{"oneof_index", oneofIndexTerm},
		{"json_name", jsonNameTerm},
		{"proto3_optional", proto3OptionalTerm},
	}
	fields = trailerFields(fields, f.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.FieldDescriptorProto", Fields: fields}
}

// fieldLabelAtomName maps a FieldDescriptorProto_Label to its uppercase
// Elixir atom name (e.g. :LABEL_OPTIONAL), matching
// google.protobuf.FieldDescriptorProto.Label's own enum value names -
// distinct from labelKeyword in message.go, which maps to a lowercase
// use-option key ("optional") for ordinary field rendering, not a descriptor
// struct literal value.
func fieldLabelAtomName(label descriptorpb.FieldDescriptorProto_Label) string {
	switch label {
	case descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL:
		return "LABEL_OPTIONAL"
	case descriptorpb.FieldDescriptorProto_LABEL_REQUIRED:
		return "LABEL_REQUIRED"
	case descriptorpb.FieldDescriptorProto_LABEL_REPEATED:
		return "LABEL_REPEATED"
	default:
		panic(fmt.Sprintf("unsupported field label %v", label))
	}
}

// fieldTypeAtomName maps a FieldDescriptorProto_Type to its uppercase Elixir
// atom name (e.g. :TYPE_STRING), matching
// google.protobuf.FieldDescriptorProto.Type's own enum value names - distinct
// from typeAtom in message.go, which maps to the lowercase `field` type
// value.
func fieldTypeAtomName(t descriptorpb.FieldDescriptorProto_Type) string {
	switch t {
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return "TYPE_DOUBLE"
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return "TYPE_FLOAT"
	case descriptorpb.FieldDescriptorProto_TYPE_INT64:
		return "TYPE_INT64"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		return "TYPE_UINT64"
	case descriptorpb.FieldDescriptorProto_TYPE_INT32:
		return "TYPE_INT32"
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		return "TYPE_FIXED64"
	case descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		return "TYPE_FIXED32"
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "TYPE_BOOL"
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return "TYPE_STRING"
	case descriptorpb.FieldDescriptorProto_TYPE_GROUP:
		return "TYPE_GROUP"
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		return "TYPE_MESSAGE"
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "TYPE_BYTES"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		return "TYPE_UINT32"
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		return "TYPE_ENUM"
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		return "TYPE_SFIXED32"
	case descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return "TYPE_SFIXED64"
	case descriptorpb.FieldDescriptorProto_TYPE_SINT32:
		return "TYPE_SINT32"
	case descriptorpb.FieldDescriptorProto_TYPE_SINT64:
		return "TYPE_SINT64"
	default:
		panic(fmt.Sprintf("unsupported field type %v", t))
	}
}

// oneofDescriptorProtoTerm is UNVERIFIED against any golden fixture (no
// message in the gen_descriptors corpus has a oneof).
func oneofDescriptorProtoTerm(o *descriptorpb.OneofDescriptorProto) elixirTerm {
	var optionsTerm = nilTerm()
	if opts := o.GetOptions(); opts != nil {
		optionsTerm = oneofOptionsTerm(opts)
	}

	fields := []field{
		{"name", stringTerm(o.GetName())},
		{"options", optionsTerm},
	}
	fields = trailerFields(fields, o.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.OneofDescriptorProto", Fields: fields}
}

// extensionRangeTerm is UNVERIFIED against any golden fixture (no
// gen_descriptors=true fixture message declares an extension range).
func extensionRangeTerm(r *descriptorpb.DescriptorProto_ExtensionRange) elixirTerm {
	var startTerm = nilTerm()
	if r.Start != nil {
		startTerm = intTerm(int64(r.GetStart()))
	}
	var endTerm = nilTerm()
	if r.End != nil {
		endTerm = intTerm(int64(r.GetEnd()))
	}

	var optionsTerm = nilTerm()
	if opts := r.GetOptions(); opts != nil {
		optionsTerm = extensionRangeOptionsTerm(opts)
	}

	fields := []field{
		{"start", startTerm},
		{"end", endTerm},
		{"options", optionsTerm},
	}
	fields = trailerFields(fields, r.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.DescriptorProto.ExtensionRange", Fields: fields}
}

// reservedRangeTerm is UNVERIFIED against any golden fixture.
func reservedRangeTerm(r *descriptorpb.DescriptorProto_ReservedRange) elixirTerm {
	var startTerm = nilTerm()
	if r.Start != nil {
		startTerm = intTerm(int64(r.GetStart()))
	}
	var endTerm = nilTerm()
	if r.End != nil {
		endTerm = intTerm(int64(r.GetEnd()))
	}

	fields := []field{
		{"start", startTerm},
		{"end", endTerm},
	}
	fields = trailerFields(fields, r.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.DescriptorProto.ReservedRange", Fields: fields}
}

// enumReservedRangeTerm is UNVERIFIED against any golden fixture.
func enumReservedRangeTerm(r *descriptorpb.EnumDescriptorProto_EnumReservedRange) elixirTerm {
	var startTerm = nilTerm()
	if r.Start != nil {
		startTerm = intTerm(int64(r.GetStart()))
	}
	var endTerm = nilTerm()
	if r.End != nil {
		endTerm = intTerm(int64(r.GetEnd()))
	}

	fields := []field{
		{"start", startTerm},
		{"end", endTerm},
	}
	fields = trailerFields(fields, r.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.EnumDescriptorProto.EnumReservedRange", Fields: fields}
}

// serviceDescriptorProtoTerm is ENTIRELY UNEVIDENCED - no fixture combines
// gen_descriptors=true with a service (see service.go's RenderService
// no-op comment and this file's ServiceModules wiring below).
func serviceDescriptorProtoTerm(svc *descriptorpb.ServiceDescriptorProto) elixirTerm {
	methodTerms := make([]elixirTerm, len(svc.GetMethod()))
	for i, m := range svc.GetMethod() {
		methodTerms[i] = methodDescriptorProtoTerm(m)
	}

	var optionsTerm = nilTerm()
	if opts := svc.GetOptions(); opts != nil {
		optionsTerm = serviceOptionsTerm(opts)
	}

	fields := []field{
		{"name", stringTerm(svc.GetName())},
		{"method", termList{Elements: methodTerms}},
		{"options", optionsTerm},
	}
	fields = trailerFields(fields, svc.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.ServiceDescriptorProto", Fields: fields}
}

// methodDescriptorProtoTerm is ENTIRELY UNEVIDENCED - see
// serviceDescriptorProtoTerm above.
func methodDescriptorProtoTerm(m *descriptorpb.MethodDescriptorProto) elixirTerm {
	var inputTerm = nilTerm()
	if m.InputType != nil {
		inputTerm = stringTerm(m.GetInputType())
	}
	var outputTerm = nilTerm()
	if m.OutputType != nil {
		outputTerm = stringTerm(m.GetOutputType())
	}

	var optionsTerm = nilTerm()
	if opts := m.GetOptions(); opts != nil {
		optionsTerm = methodOptionsTerm(opts)
	}

	fields := []field{
		{"name", stringTerm(m.GetName())},
		{"input_type", inputTerm},
		{"output_type", outputTerm},
		{"options", optionsTerm},
		{"client_streaming", boolTerm(m.GetClientStreaming())},
		{"server_streaming", boolTerm(m.GetServerStreaming())},
	}
	fields = trailerFields(fields, m.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.MethodDescriptorProto", Fields: fields}
}

// ---------------------------------------------------------------------------
// *Options builders - each gets __pb_extensions__ before the trailer (rule 4)
// ---------------------------------------------------------------------------

// messageOptionsTerm is EVIDENCED byte-for-byte against
// MessageWithCustomOptions in the golden fixture.
func messageOptionsTerm(opts *descriptorpb.MessageOptions) elixirTerm {
	var mapEntryTerm = nilTerm()
	if opts.MapEntry != nil {
		mapEntryTerm = boolTerm(opts.GetMapEntry())
	}
	// deprecated_legacy_json_field_conflicts is deprecated in
	// descriptor.proto itself, but still a real field number this rendering
	// table must cover (evidenced by MessageOptions in the golden fixture,
	// which renders it as an explicit nil).
	var depLegacyTerm = nilTerm()                       //nolint:staticcheck
	if opts.DeprecatedLegacyJsonFieldConflicts != nil { //nolint:staticcheck
		depLegacyTerm = boolTerm(opts.GetDeprecatedLegacyJsonFieldConflicts()) //nolint:staticcheck
	}

	uninterpTerms := make([]elixirTerm, len(opts.GetUninterpretedOption()))
	for i, u := range opts.GetUninterpretedOption() {
		uninterpTerms[i] = uninterpretedOptionTerm(u)
	}

	fields := []field{
		{"message_set_wire_format", boolTerm(opts.GetMessageSetWireFormat())},
		{"no_standard_descriptor_accessor", boolTerm(opts.GetNoStandardDescriptorAccessor())},
		{"deprecated", boolTerm(opts.GetDeprecated())},
		{"map_entry", mapEntryTerm},
		{"deprecated_legacy_json_field_conflicts", depLegacyTerm},
		{"features", nilTerm()}, // FeatureSet: no fixture sets it, always rendered nil (editions support out of scope).
		{"uninterpreted_option", termList{Elements: uninterpTerms}},
		pbExtensionsField(),
	}
	fields = trailerFields(fields, opts.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.MessageOptions", Fields: fields}
}

// fieldOptionsTerm has no golden fixture exercising these fields, but each
// one's presence-gating is grounded in the corresponding `field` declaration
// in elixir-protobuf/protobuf's own generated descriptor.pb.ex at the pinned
// commit: a field declared with a Protobuf `default:` renders its effective
// value even when unset (ctype, jstype, lazy, weak, unverified_lazy,
// debug_redact, deprecated), while a field declared with no `default:` stays
// nil unless explicitly set (packed, retention). targets/edition_defaults
// (19/20) are very-recent editions-era fields - plausibly ALSO absent from
// this pin (like Visibility was excluded above), but no fixture proves it
// either way, so they're included on a best-effort basis rather than
// guessed-excluded: excluding without evidence just substitutes one guess for
// another.
func fieldOptionsTerm(opts *descriptorpb.FieldOptions) elixirTerm {
	var packedTerm = nilTerm()
	if opts.Packed != nil {
		packedTerm = boolTerm(opts.GetPacked())
	}
	// weak is deprecated in descriptor.proto itself, but still a real field
	// number this rendering table must cover on the same best-effort basis as
	// the rest of FieldOptions (see this function's own doc comment).
	weakTerm := boolTerm(opts.GetWeak()) //nolint:staticcheck
	var retentionTerm = nilTerm()
	if opts.Retention != nil {
		retentionTerm = atomTerm(fieldOptionsRetentionAtomName(opts.GetRetention()))
	}

	targetTerms := make([]elixirTerm, len(opts.GetTargets()))
	for i, target := range opts.GetTargets() {
		targetTerms[i] = atomTerm(fieldOptionsTargetTypeAtomName(target))
	}

	// edition_defaults: no builder is provided for FieldOptions_EditionDefault
	// itself (deeper editions-era nesting, entirely out of the evidenced
	// corpus) - rendered as an empty list unless populated, which is the only
	// value ever observed for any *Options.uninterpreted_option-adjacent
	// repeated field with no test coverage.
	editionDefaultTerms := make([]elixirTerm, 0, len(opts.GetEditionDefaults()))

	// FeatureSupport: no fixture sets it, always rendered nil (editions support out of scope).
	featureSupportTerm := nilTerm()

	uninterpTerms := make([]elixirTerm, len(opts.GetUninterpretedOption()))
	for i, u := range opts.GetUninterpretedOption() {
		uninterpTerms[i] = uninterpretedOptionTerm(u)
	}

	fields := []field{
		{"ctype", atomTerm(fieldOptionsCTypeAtomName(opts.GetCtype()))},
		{"packed", packedTerm},
		{"deprecated", boolTerm(opts.GetDeprecated())},
		{"lazy", boolTerm(opts.GetLazy())},
		{"jstype", atomTerm(fieldOptionsJSTypeAtomName(opts.GetJstype()))},
		{"weak", weakTerm},
		{"unverified_lazy", boolTerm(opts.GetUnverifiedLazy())},
		{"debug_redact", boolTerm(opts.GetDebugRedact())},
		{"retention", retentionTerm},
		{"targets", termList{Elements: targetTerms}},
		{"edition_defaults", termList{Elements: editionDefaultTerms}},
		{"features", nilTerm()},
		{"feature_support", featureSupportTerm},
		{"uninterpreted_option", termList{Elements: uninterpTerms}},
		pbExtensionsField(),
	}
	fields = trailerFields(fields, opts.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.FieldOptions", Fields: fields}
}

func fieldOptionsCTypeAtomName(v descriptorpb.FieldOptions_CType) string {
	switch v {
	case descriptorpb.FieldOptions_STRING:
		return "STRING"
	case descriptorpb.FieldOptions_CORD:
		return "CORD"
	case descriptorpb.FieldOptions_STRING_PIECE:
		return "STRING_PIECE"
	default:
		panic(fmt.Sprintf("unsupported FieldOptions_CType %v", v))
	}
}

func fieldOptionsJSTypeAtomName(v descriptorpb.FieldOptions_JSType) string {
	switch v {
	case descriptorpb.FieldOptions_JS_NORMAL:
		return "JS_NORMAL"
	case descriptorpb.FieldOptions_JS_STRING:
		return "JS_STRING"
	case descriptorpb.FieldOptions_JS_NUMBER:
		return "JS_NUMBER"
	default:
		panic(fmt.Sprintf("unsupported FieldOptions_JSType %v", v))
	}
}

func fieldOptionsRetentionAtomName(v descriptorpb.FieldOptions_OptionRetention) string {
	switch v {
	case descriptorpb.FieldOptions_RETENTION_UNKNOWN:
		return "RETENTION_UNKNOWN"
	case descriptorpb.FieldOptions_RETENTION_RUNTIME:
		return "RETENTION_RUNTIME"
	case descriptorpb.FieldOptions_RETENTION_SOURCE:
		return "RETENTION_SOURCE"
	default:
		panic(fmt.Sprintf("unsupported FieldOptions_OptionRetention %v", v))
	}
}

func fieldOptionsTargetTypeAtomName(v descriptorpb.FieldOptions_OptionTargetType) string {
	switch v {
	case descriptorpb.FieldOptions_TARGET_TYPE_UNKNOWN:
		return "TARGET_TYPE_UNKNOWN"
	case descriptorpb.FieldOptions_TARGET_TYPE_FILE:
		return "TARGET_TYPE_FILE"
	case descriptorpb.FieldOptions_TARGET_TYPE_EXTENSION_RANGE:
		return "TARGET_TYPE_EXTENSION_RANGE"
	case descriptorpb.FieldOptions_TARGET_TYPE_MESSAGE:
		return "TARGET_TYPE_MESSAGE"
	case descriptorpb.FieldOptions_TARGET_TYPE_FIELD:
		return "TARGET_TYPE_FIELD"
	case descriptorpb.FieldOptions_TARGET_TYPE_ONEOF:
		return "TARGET_TYPE_ONEOF"
	case descriptorpb.FieldOptions_TARGET_TYPE_ENUM:
		return "TARGET_TYPE_ENUM"
	case descriptorpb.FieldOptions_TARGET_TYPE_ENUM_ENTRY:
		return "TARGET_TYPE_ENUM_ENTRY"
	case descriptorpb.FieldOptions_TARGET_TYPE_SERVICE:
		return "TARGET_TYPE_SERVICE"
	case descriptorpb.FieldOptions_TARGET_TYPE_METHOD:
		return "TARGET_TYPE_METHOD"
	default:
		panic(fmt.Sprintf("unsupported FieldOptions_OptionTargetType %v", v))
	}
}

// oneofOptionsTerm is UNVERIFIED against any golden fixture.
func oneofOptionsTerm(opts *descriptorpb.OneofOptions) elixirTerm {
	uninterpTerms := make([]elixirTerm, len(opts.GetUninterpretedOption()))
	for i, u := range opts.GetUninterpretedOption() {
		uninterpTerms[i] = uninterpretedOptionTerm(u)
	}

	fields := []field{
		{"features", nilTerm()},
		{"uninterpreted_option", termList{Elements: uninterpTerms}},
		pbExtensionsField(),
	}
	fields = trailerFields(fields, opts.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.OneofOptions", Fields: fields}
}

// enumOptionsTerm is UNVERIFIED against any golden fixture (no fixture enum
// carries EnumOptions - EnumWithCustomOptions's own options field is nil).
func enumOptionsTerm(opts *descriptorpb.EnumOptions) elixirTerm {
	var allowAliasTerm = nilTerm()
	if opts.AllowAlias != nil {
		allowAliasTerm = boolTerm(opts.GetAllowAlias())
	}
	// deprecated_legacy_json_field_conflicts is deprecated in
	// descriptor.proto itself, but still a real field number this rendering
	// table must cover (evidenced by MessageOptions in the golden fixture,
	// which renders it as an explicit nil).
	var depLegacyTerm = nilTerm()                       //nolint:staticcheck
	if opts.DeprecatedLegacyJsonFieldConflicts != nil { //nolint:staticcheck
		depLegacyTerm = boolTerm(opts.GetDeprecatedLegacyJsonFieldConflicts()) //nolint:staticcheck
	}

	uninterpTerms := make([]elixirTerm, len(opts.GetUninterpretedOption()))
	for i, u := range opts.GetUninterpretedOption() {
		uninterpTerms[i] = uninterpretedOptionTerm(u)
	}

	fields := []field{
		{"allow_alias", allowAliasTerm},
		{"deprecated", boolTerm(opts.GetDeprecated())},
		{"deprecated_legacy_json_field_conflicts", depLegacyTerm},
		{"features", nilTerm()},
		{"uninterpreted_option", termList{Elements: uninterpTerms}},
		pbExtensionsField(),
	}
	fields = trailerFields(fields, opts.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.EnumOptions", Fields: fields}
}

// enumValueOptionsTerm is EVIDENCED byte-for-byte against MY_ENUM_FOO's
// options in the golden fixture.
func enumValueOptionsTerm(opts *descriptorpb.EnumValueOptions) elixirTerm {
	// FeatureSupport: no fixture sets it, always rendered nil (editions support out of scope).
	featureSupportTerm := nilTerm()

	uninterpTerms := make([]elixirTerm, len(opts.GetUninterpretedOption()))
	for i, u := range opts.GetUninterpretedOption() {
		uninterpTerms[i] = uninterpretedOptionTerm(u)
	}

	fields := []field{
		{"deprecated", boolTerm(opts.GetDeprecated())},
		{"features", nilTerm()},
		{"debug_redact", boolTerm(opts.GetDebugRedact())},
		{"feature_support", featureSupportTerm},
		{"uninterpreted_option", termList{Elements: uninterpTerms}},
		pbExtensionsField(),
	}
	fields = trailerFields(fields, opts.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.EnumValueOptions", Fields: fields}
}

// serviceOptionsTerm is UNVERIFIED against any golden fixture.
func serviceOptionsTerm(opts *descriptorpb.ServiceOptions) elixirTerm {
	uninterpTerms := make([]elixirTerm, len(opts.GetUninterpretedOption()))
	for i, u := range opts.GetUninterpretedOption() {
		uninterpTerms[i] = uninterpretedOptionTerm(u)
	}

	fields := []field{
		{"deprecated", boolTerm(opts.GetDeprecated())},
		{"features", nilTerm()},
		{"uninterpreted_option", termList{Elements: uninterpTerms}},
		pbExtensionsField(),
	}
	fields = trailerFields(fields, opts.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.ServiceOptions", Fields: fields}
}

// methodOptionsTerm has no golden fixture exercising idempotency_level, but
// MethodOptions declares it with a Protobuf `default:` (like deprecated), so
// it renders its effective value even when unset - see fieldOptionsTerm's
// doc comment for the underlying rationale.
func methodOptionsTerm(opts *descriptorpb.MethodOptions) elixirTerm {
	uninterpTerms := make([]elixirTerm, len(opts.GetUninterpretedOption()))
	for i, u := range opts.GetUninterpretedOption() {
		uninterpTerms[i] = uninterpretedOptionTerm(u)
	}

	fields := []field{
		{"deprecated", boolTerm(opts.GetDeprecated())},
		{"idempotency_level", atomTerm(methodOptionsIdempotencyLevelAtomName(opts.GetIdempotencyLevel()))},
		{"features", nilTerm()},
		{"uninterpreted_option", termList{Elements: uninterpTerms}},
		pbExtensionsField(),
	}
	fields = trailerFields(fields, opts.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.MethodOptions", Fields: fields}
}

func methodOptionsIdempotencyLevelAtomName(v descriptorpb.MethodOptions_IdempotencyLevel) string {
	switch v {
	case descriptorpb.MethodOptions_IDEMPOTENCY_UNKNOWN:
		return "IDEMPOTENCY_UNKNOWN"
	case descriptorpb.MethodOptions_NO_SIDE_EFFECTS:
		return "NO_SIDE_EFFECTS"
	case descriptorpb.MethodOptions_IDEMPOTENT:
		return "IDEMPOTENT"
	default:
		panic(fmt.Sprintf("unsupported MethodOptions_IdempotencyLevel %v", v))
	}
}

// extensionRangeOptionsTerm is UNVERIFIED against any golden fixture.
// declaration is a recent, low-value field for this corpus; it renders as an
// always-empty list (see fieldOptionsTerm's edition_defaults comment - same
// rationale, no builder for the nested Declaration message since nothing in
// the corpus exercises it). verification declares a Protobuf `default:`
// (like FieldOptions.ctype/jstype), so it renders its effective value even
// when unset - see fieldOptionsTerm's doc comment for the underlying
// rationale.
func extensionRangeOptionsTerm(opts *descriptorpb.ExtensionRangeOptions) elixirTerm {
	declarationTerms := make([]elixirTerm, 0, len(opts.GetDeclaration()))

	uninterpTerms := make([]elixirTerm, len(opts.GetUninterpretedOption()))
	for i, u := range opts.GetUninterpretedOption() {
		uninterpTerms[i] = uninterpretedOptionTerm(u)
	}

	fields := []field{
		{"declaration", termList{Elements: declarationTerms}},
		{"verification", atomTerm(extensionRangeOptionsVerificationStateAtomName(opts.GetVerification()))},
		{"features", nilTerm()},
		{"uninterpreted_option", termList{Elements: uninterpTerms}},
		pbExtensionsField(),
	}
	fields = trailerFields(fields, opts.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.ExtensionRangeOptions", Fields: fields}
}

func extensionRangeOptionsVerificationStateAtomName(v descriptorpb.ExtensionRangeOptions_VerificationState) string {
	switch v {
	case descriptorpb.ExtensionRangeOptions_DECLARATION:
		return "DECLARATION"
	case descriptorpb.ExtensionRangeOptions_UNVERIFIED:
		return "UNVERIFIED"
	default:
		panic(fmt.Sprintf("unsupported ExtensionRangeOptions_VerificationState %v", v))
	}
}

// uninterpretedOptionTerm is UNVERIFIED against any golden fixture: protoc
// always resolves custom_options.proto's extension usages into known
// (albeit Go-registry-unknown) field numbers rather than leaving them as
// UninterpretedOption entries, so no fixture in the corpus ever populates
// uninterpreted_option with a non-empty list. Implemented per
// UninterpretedOption's own proto field numbers on the same best-evidenced
// basis as the rest of this UNVERIFIED section.
func uninterpretedOptionTerm(u *descriptorpb.UninterpretedOption) elixirTerm {
	nameTerms := make([]elixirTerm, len(u.GetName()))
	for i, n := range u.GetName() {
		nameTerms[i] = uninterpretedOptionNamePartTerm(n)
	}

	var identifierTerm = nilTerm()
	if u.IdentifierValue != nil {
		identifierTerm = stringTerm(u.GetIdentifierValue())
	}
	var posIntTerm = nilTerm()
	if u.PositiveIntValue != nil {
		posIntTerm = uintTerm(u.GetPositiveIntValue())
	}
	var negIntTerm = nilTerm()
	if u.NegativeIntValue != nil {
		negIntTerm = intTerm(u.GetNegativeIntValue())
	}
	var doubleTerm = nilTerm()
	if u.DoubleValue != nil {
		doubleTerm = rawTerm(strconv.FormatFloat(u.GetDoubleValue(), 'g', -1, 64))
	}
	var stringValTerm = nilTerm()
	if u.StringValue != nil {
		stringValTerm = decodeLengthDelimitedTerm(u.GetStringValue())
	}
	var aggregateTerm = nilTerm()
	if u.AggregateValue != nil {
		aggregateTerm = stringTerm(u.GetAggregateValue())
	}

	fields := []field{
		{"name", termList{Elements: nameTerms}},
		{"identifier_value", identifierTerm},
		{"positive_int_value", posIntTerm},
		{"negative_int_value", negIntTerm},
		{"double_value", doubleTerm},
		{"string_value", stringValTerm},
		{"aggregate_value", aggregateTerm},
	}
	fields = trailerFields(fields, u.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.UninterpretedOption", Fields: fields}
}

func uninterpretedOptionNamePartTerm(n *descriptorpb.UninterpretedOption_NamePart) elixirTerm {
	fields := []field{
		{"name_part", stringTerm(n.GetNamePart())},
		{"is_extension", boolTerm(n.GetIsExtension())},
	}
	fields = trailerFields(fields, n.ProtoReflect().GetUnknown())

	return termStruct{TypeName: "Google.Protobuf.UninterpretedOption.NamePart", Fields: fields}
}

// ---------------------------------------------------------------------------
// def descriptor do ... end emission
// ---------------------------------------------------------------------------

// renderDescriptorFunction wraps term in the "def descriptor do ... end"
// function body shared by enum/message/service modules: a literal "# credo:disable-for-next-line"
// comment line (2-space indent, matching the function body indent level)
// immediately before the struct literal (rule 6), indented at 4 spaces (2
// for "def descriptor do", 2 more for the body), blank-line separated from
// whatever precedes/follows it by the caller.
func renderDescriptorFunction(term elixirTerm) string {
	var b strings.Builder
	b.WriteString("  def descriptor do\n")
	b.WriteString("    # credo:disable-for-next-line\n")
	b.WriteString("    ")
	b.WriteString(term.render(4))
	b.WriteString("\n  end")
	return b.String()
}

// RenderEnumDescriptor renders the full "def descriptor do ... end" block for
// an enum module, per rule 7 (EVIDENCED: placed right after `use Protobuf`,
// before `field :NAME, N` lines, blank-line separated from both).
func RenderEnumDescriptor(enum *descriptorpb.EnumDescriptorProto) string {
	return renderDescriptorFunction(enumDescriptorProtoTerm(enum))
}

// RenderMessageDescriptor renders the full "def descriptor do ... end" block
// for a message module. Placement (right after `use Protobuf`, before
// oneof/field/extensions sections) is UNEVIDENCED - MessageWithCustomOptions,
// the only gen_descriptors=true message fixture, has no fields, oneofs,
// nested types, or extension ranges to be ordered relative to, so this
// relative order is inferred to match the enum case (rule 7) rather than
// independently proven.
func RenderMessageDescriptor(msg *descriptorpb.DescriptorProto) string {
	return renderDescriptorFunction(descriptorProtoTerm(msg))
}

// RenderServiceDescriptor renders the full "def descriptor do ... end" block
// for a service's .Service module. ENTIRELY UNEVIDENCED (rule 7): no fixture
// combines gen_descriptors=true with plugins=grpc. Placement is wired in
// service.go immediately after `def proto_source()` (when gen_proto_source is
// also set) or in that same slot right after `use GRPC.Service` otherwise,
// per the existing proto_source precedent.
func RenderServiceDescriptor(svc *descriptorpb.ServiceDescriptorProto) string {
	return renderDescriptorFunction(serviceDescriptorProtoTerm(svc))
}
