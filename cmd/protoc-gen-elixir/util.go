package main

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/TrogonStack/protoc-gen/cmd/protoc-gen-elixir/internal/elixirpb"
)

// protocGenElixirVersion mirrors Util.version/0 at the pinned escript HEAD
// (mix.exs @version "0.17.0", unreleased). Embedded into every generated
// module's protoc_gen_elixir_version: option.
const protocGenElixirVersion = "0.17.0"

var protoNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateProtoName mirrors Util.validate_proto_name!/1: any name that
// doesn't match [A-Za-z_][A-Za-z0-9_]* is rejected. Unlike the escript,
// which raises, this returns a plain error so callers can surface it as a
// CodeGeneratorResponse.error instead of panicking.
func ValidateProtoName(name string) error {
	if !protoNamePattern.MatchString(name) {
		return fmt.Errorf("invalid name: %q", name)
	}
	return nil
}

// CamelizeEach mirrors Macro.camelize/1 applied independently to each
// dot-separated segment of name: split each segment on "_", capitalize each
// part, then join segments back together with ".". Underscore-splitting
// never crosses a dot boundary, e.g. "foo_bar.ab_cd" -> "FooBar.AbCd".
func CamelizeEach(name string) string {
	if name == "" {
		return ""
	}

	segments := strings.Split(name, ".")
	for i, segment := range segments {
		segments[i] = camelizeSegment(segment)
	}
	return strings.Join(segments, ".")
}

func camelizeSegment(segment string) string {
	parts := strings.Split(segment, "_")
	for i, part := range parts {
		parts[i] = capitalize(part)
	}
	return strings.Join(parts, "")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ModName mirrors Protobuf.Protoc.Generator.Util.mod_name/2: computes the
// Elixir module name prefix for types defined in file, then appends
// nestedNames (outer to inner), camelizing each independently.
//
// Precedence for the prefix (elixirpb.file.module_prefix beats
// package_prefix, which beats package alone) matches the escript's
// mod_name/2, which checks the file-level extension before falling back to
// context-supplied naming.
func ModName(file *descriptorpb.FileDescriptorProto, packagePrefix *string, nestedNames []string) string {
	prefix := modNamePrefix(file, packagePrefix)

	segments := make([]string, 0, 1+len(nestedNames))
	if prefix != "" {
		segments = append(segments, prefix)
	}
	segments = append(segments, nestedNames...)

	return strings.Join(segments, ".")
}

func modNamePrefix(file *descriptorpb.FileDescriptorProto, packagePrefix *string) string {
	if modulePrefix, ok := elixirModulePrefix(file); ok {
		return CamelizeEach(modulePrefix)
	}

	pkg := file.GetPackage()

	if packagePrefix != nil {
		if pkg == "" {
			return CamelizeEach(*packagePrefix)
		}
		return CamelizeEach(*packagePrefix + "." + pkg)
	}

	if pkg == "" {
		return ""
	}
	return CamelizeEach(pkg)
}

func elixirModulePrefix(file *descriptorpb.FileDescriptorProto) (string, bool) {
	opts := file.GetOptions()
	if opts == nil {
		return "", false
	}

	if !proto.HasExtension(opts, elixirpb.E_File) {
		return "", false
	}

	fileOpts, ok := proto.GetExtension(opts, elixirpb.E_File).(*elixirpb.FileOptions)
	if !ok || fileOpts == nil {
		return "", false
	}

	prefix := fileOpts.GetModulePrefix()
	if prefix == "" {
		return "", false
	}
	return prefix, true
}

// Atom wraps a string that must render as an unquoted Elixir atom (:foo)
// rather than a quoted string literal ("foo"), so RenderOptions can tell the
// two apart without callers pre-formatting values.
type Atom string

// Option is a single key/value pair for a `use Protobuf, ...` option list.
// Value must be a bool, string, or Atom.
type Option struct {
	Key   string
	Value any
}

// RenderOptionValue renders a single option value per its Go type: bool ->
// unquoted true/false, Atom -> :value, string -> Go %q-quoted.
func RenderOptionValue(value any) string {
	switch v := value.(type) {
	case bool:
		if v {
			return "true"
		}
		return "false"
	case Atom:
		return ":" + string(v)
	case string:
		return fmt.Sprintf("%q", v)
	default:
		panic(fmt.Sprintf("unsupported option value type %T", value))
	}
}

// RenderOptionsBody sorts opts alphabetically by key and renders a
// comma-joined "key: value, key2: value2" body, without a leading comma or
// the "use Protobuf" prefix. Call sites are responsible for prepending
// "use Protobuf, " and applying the line-wrap threshold.
func RenderOptionsBody(opts []Option) string {
	sorted := sortedOptions(opts)

	rendered := make([]string, len(sorted))
	for i, opt := range sorted {
		rendered[i] = opt.Key + ": " + RenderOptionValue(opt.Value)
	}
	return strings.Join(rendered, ", ")
}

func sortedOptions(opts []Option) []Option {
	sorted := make([]Option, len(opts))
	copy(sorted, opts)
	slices.SortFunc(sorted, func(a, b Option) int {
		return cmp.Compare(a.Key, b.Key)
	})
	return sorted
}

// useProtobufLineThreshold is the escript's default Code.format_string!/2
// line length (98 chars), used to decide whether "use Protobuf, <opts>" fits
// on one line or must be wrapped one option per line.
const useProtobufLineThreshold = 98

// RenderUseProtobuf renders a full "use Protobuf, ..." block at the given
// indent (in spaces), choosing single-line or wrapped form based on whether
// the single-line rendering fits within useProtobufLineThreshold.
func RenderUseProtobuf(indent int, opts []Option) string {
	pad := strings.Repeat(" ", indent)
	body := RenderOptionsBody(opts)
	singleLine := pad + "use Protobuf, " + body

	if len(singleLine) <= useProtobufLineThreshold {
		return singleLine
	}

	sorted := sortedOptions(opts)

	optionPad := strings.Repeat(" ", indent+2)
	optionLines := make([]string, len(sorted))
	for i, opt := range sorted {
		optionLines[i] = optionPad + opt.Key + ": " + RenderOptionValue(opt.Value)
	}

	return pad + "use Protobuf,\n" + strings.Join(optionLines, ",\n")
}

// groupIntegerDigits mirrors mix format's numeric-literal normalization for
// integer literals: groups of 3 digits from the right, joined with "_" (e.g.
// 2147483647 -> "2_147_483_647"). Evidenced by
// testdata/golden/package_prefix/my/test/test.pb.ex's `OldReply` extension
// range, whose descriptor End is the literal 2147483647 (INT32_MAX, not the
// ordinary "extensions ... to max" sentinel - see RenderMessage's extension-
// range doc comment). Scoped strictly to extension-range rendering: this is
// NOT applied to renderDefaultValue's existing int case, since no fixture in
// the corpus has a default value large enough to trigger digit-grouping and
// that behavior is already independently verified against its own fixtures.
// The same grouping would likely apply there too if a fixture ever exercised
// it, but that's an untested claim, so renderDefaultValue is left untouched.
func groupIntegerDigits(n int32) string {
	s := strconv.Itoa(int(n))

	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}

	var groups []string
	for len(s) > 3 {
		groups = append([]string{s[len(s)-3:]}, groups...)
		s = s[:len(s)-3]
	}
	groups = append([]string{s}, groups...)

	result := strings.Join(groups, "_")
	if neg {
		result = "-" + result
	}
	return result
}

// ExtractDocComment mirrors Protobuf.Protoc.Generator.Util's doc-comment
// handling (comment.ex at the pinned escript HEAD): finds the
// SourceCodeInfo_Location matching path, combines leading_detached_comments
// + leading_comments + trailing_comments with blank-line separators,
// collapses runs of 3+ newlines, dedents, and trims.
func ExtractDocComment(sci *descriptorpb.SourceCodeInfo, path []int32) string {
	loc := findLocation(sci, path)
	if loc == nil {
		return ""
	}

	var parts []string

	if detached := loc.GetLeadingDetachedComments(); len(detached) > 0 {
		parts = append(parts, strings.Join(detached, "\n\n"))
	}
	if leading := loc.GetLeadingComments(); leading != "" {
		parts = append(parts, leading)
	}
	if trailing := loc.GetTrailingComments(); trailing != "" {
		parts = append(parts, trailing)
	}

	combined := strings.Join(parts, "\n\n")
	combined = collapseNewlines(combined)
	combined = dedent(combined)
	return strings.TrimSpace(combined)
}

func findLocation(sci *descriptorpb.SourceCodeInfo, path []int32) *descriptorpb.SourceCodeInfo_Location {
	if sci == nil {
		return nil
	}
	for _, loc := range sci.GetLocation() {
		if int32SliceEqual(loc.GetPath(), path) {
			return loc
		}
	}
	return nil
}

func int32SliceEqual(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var newlineRunPattern = regexp.MustCompile(`\n{3,}`)

func collapseNewlines(s string) string {
	return newlineRunPattern.ReplaceAllString(s, "\n")
}

func dedent(s string) string {
	if s == "" {
		return s
	}

	lines := strings.Split(s, "\n")

	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent <= 0 {
		return s
	}

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[i] = line[minIndent:]
	}
	return strings.Join(lines, "\n")
}
