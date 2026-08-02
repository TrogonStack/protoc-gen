package main

import (
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// generateFiles walks the requested proto files and renders one
// CodeGeneratorResponse_File per file that contains at least one
// generatable top-level module: a top-level enum, a message (at any nesting
// depth), or - when plugins=grpc is set - a service. A file with nothing
// generatable is skipped entirely rather than producing an empty file (this
// is what makes service.proto alone, without plugins=grpc, produce no output
// at all - it defines only a service and nothing else).
//
// A second pass, after the per-file loop, produces the file-level merged
// PbExtension modules: every file being generated this run contributes its own top-level
// extend fields (file.GetExtension()) to a group keyed by that file's
// computed base Elixir module name, and each non-empty group becomes one
// additional CodeGeneratorResponse_File. This is architecturally independent
// of any single input file's own rendered content - see
// renderFileExtensionGroups.
func generateFiles(req *pluginpb.CodeGeneratorRequest, params *Params) ([]*pluginpb.CodeGeneratorResponse_File, error) {
	filesByName := make(map[string]*descriptorpb.FileDescriptorProto, len(req.GetProtoFile()))
	for _, file := range req.GetProtoFile() {
		filesByName[file.GetName()] = file
	}

	types := NewTypeRegistry(req.GetProtoFile(), buildFileToGenerateSet(req.GetFileToGenerate()), params.PackagePrefix)

	var out []*pluginpb.CodeGeneratorResponse_File
	var generated []*descriptorpb.FileDescriptorProto

	for _, name := range req.GetFileToGenerate() {
		file, ok := filesByName[name]
		if !ok {
			continue
		}
		generated = append(generated, file)

		hasServices := params.HasPlugin(PluginGRPC) && len(file.GetService()) > 0
		if len(file.GetEnumType()) == 0 && len(file.GetMessageType()) == 0 && !hasServices {
			continue
		}

		modules, err := renderFileModules(file, params, types)
		if err != nil {
			return nil, err
		}

		if params.OneFilePerModule {
			for _, m := range modules {
				out = append(out, &pluginpb.CodeGeneratorResponse_File{
					Name:    proto.String(modulePath(m.modName)),
					Content: proto.String(m.text + "\n"),
				})
			}
			continue
		}

		texts := make([]string, len(modules))
		for i, m := range modules {
			texts[i] = m.text
		}

		out = append(out, &pluginpb.CodeGeneratorResponse_File{
			Name:    proto.String(outputPath(file, params)),
			Content: proto.String(strings.Join(texts, "\n\n") + "\n"),
		})
	}

	extFiles, err := renderFileExtensionGroups(generated, params, types)
	if err != nil {
		return nil, err
	}
	out = append(out, extFiles...)

	return out, nil
}

// renderFileExtensionGroups implements module-ordering tier 5's file-level
// case: groups every generated file's top-level extend fields
// (file.GetExtension()) by that file's computed base Elixir module name
// (ModName, the same helper used everywhere else - more robust than grouping
// by raw proto package string alone, since two files nominally in the same
// package could theoretically disagree via elixirpb.file.module_prefix; no
// fixture exercises that edge case, so this grouping choice is a reasonable,
// non-fixture-proven inference), concatenating each contributing file's
// extend fields in file-processing order, and rendering one
// CodeGeneratorResponse_File per non-empty group via RenderFileExtension.
//
// Only one file in the current corpus (test.proto) has top-level extend
// blocks - its sibling service.proto declares none - so true cross-file
// merging (two different files contributing to the SAME group) isn't
// directly exercised by any fixture; the grouping mechanism is implemented
// generally per the rule that file-level extensions sharing a package are
// merged into one module, but that specific multi-file-into-one-group path
// is unproven.
func renderFileExtensionGroups(files []*descriptorpb.FileDescriptorProto, params *Params, types *TypeRegistry) ([]*pluginpb.CodeGeneratorResponse_File, error) {
	var order []string
	groups := make(map[string][]*descriptorpb.FieldDescriptorProto)

	for _, file := range files {
		baseModName := ModName(file, params.PackagePrefix, nil)
		if _, ok := groups[baseModName]; !ok {
			order = append(order, baseModName)
		}
		groups[baseModName] = append(groups[baseModName], file.GetExtension()...)
	}

	var out []*pluginpb.CodeGeneratorResponse_File

	for _, baseModName := range order {
		fields := groups[baseModName]
		if len(fields) == 0 {
			continue
		}

		modName := qualifyModName(baseModName, "PbExtension")
		content, ok, err := RenderFileExtension(fields, modName, params.IncludeDocs, types)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		out = append(out, &pluginpb.CodeGeneratorResponse_File{
			Name:    proto.String(modulePath(modName)),
			Content: proto.String(content + "\n"),
		})
	}

	return out, nil
}

// renderFileModules renders a file's modules in the order derived and
// verified from testdata/golden/package_prefix/my/test/test.pb.ex (this
// derivation is more precise than a literal reading of the original
// "Module Ordering Within a File" table):
//
//  1. File-level enums, in declaration order.
//  2. ALL nested enums, at any depth, from ANY message, in depth-first
//     pre-order: walk top-level messages in declaration order, recursing
//     into each message's NestedType (in declaration order) before
//     advancing to the next top-level message, collecting every EnumType
//     encountered along the way. This entire section renders before any
//     message bodies.
//  3. Message bodies, depth-first POST-order per top-level message (in file
//     declaration order): nested messages render before their parent.
//     Nested enums are never rendered again here.
//  4. Message-embedded PbExtension submodules (RenderMessageExtension),
//     DEFERRED and rendered as one contiguous block after every tier-3
//     message body (not inline alongside their own parent message's normal
//     declaration-order position) - collected via a recursive walk in
//     declaration order among themselves, at any nesting depth, across all
//     top-level messages. Verified BY POSITION against
//     testdata/golden/package_prefix/my/test/test.pb.ex: ReplyExtensions is
//     declared before OtherReplyExtensions/OldReply/Communique/Options/
//     MapInput, yet My.Test.ReplyExtensions.PbExtension is the LAST module
//     in the file, after My.Test.MapInput. Only one message-embedded extend
//     block exists in the corpus, so multi-instance relative ordering among
//     several such submodules is a reasonable, non-fixture-proven inference,
//     not independently verified. This tier is placed before tier 5
//     (services) per the original tier table ("5. Extension modules" listed
//     after "4. Services" in an earlier draft numbering; this codebase's own
//     tiers are renumbered here since deferred extension submodules turned
//     out to interleave before services, not after), but no fixture contains
//     both a service AND a message-embedded extend in the same file, so the
//     exact service-vs-this-tier relative order is inferred from that
//     original table, not independently proven.
//  5. Services (gated on plugins=grpc), in file declaration order, each as
//     one module pairing its ".Service" and ".Stub" defmodules - rendered
//     after every message body, verified against
//     testdata/golden/grpc/test/service.pb.ex and
//     testdata/golden/grpc_proto_source/test/service.pb.ex.
//
// The file-level merged PbExtension module is NOT rendered here at all - see
// generateFiles' second pass (renderFileExtensionGroups), which produces it
// as an entirely separate output file.
func renderFileModules(file *descriptorpb.FileDescriptorProto, params *Params, types *TypeRegistry) ([]renderedModule, error) {
	baseModName := ModName(file, params.PackagePrefix, nil)
	syntax := file.GetSyntax()

	ctx := &fileRenderContext{
		file:      file,
		params:    params,
		types:     types,
		syntax:    syntax,
		baseMod:   baseModName,
		basePkg:   file.GetPackage(),
		protoPath: file.GetName(),
	}

	var rendered []renderedModule

	for i, enum := range file.GetEnumType() {
		m, err := ctx.renderEnum(enum, ctx.baseMod, ctx.basePkg, []int32{5, int32(i)})
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, m)
	}

	for i, msg := range file.GetMessageType() {
		nestedEnums, err := ctx.renderNestedEnumsPreOrder(msg, ctx.baseMod, ctx.basePkg, []int32{4, int32(i)})
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, nestedEnums...)
	}

	for i, msg := range file.GetMessageType() {
		msgModules, err := ctx.renderMessageSubtreePostOrder(msg, ctx.baseMod, ctx.basePkg, []int32{4, int32(i)})
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, msgModules...)
	}

	for _, msg := range file.GetMessageType() {
		extModules, err := ctx.renderMessageExtensionsDeferred(msg, ctx.baseMod, ctx.syntax)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, extModules...)
	}

	if params.HasPlugin(PluginGRPC) {
		for i, svc := range file.GetService() {
			svcModules, err := ctx.renderService(svc, []int32{6, int32(i)})
			if err != nil {
				return nil, err
			}
			rendered = append(rendered, svcModules...)
		}
	}

	return rendered, nil
}

// renderedModule pairs a rendered Elixir module's text with its own
// fully-qualified module name, so one_file_per_module=true can derive each
// module's own output path (via modulePath) independently of the file it
// came from.
type renderedModule struct {
	modName string
	text    string
}

// fileRenderContext carries the per-file constants renderFileModules'
// recursive helpers need, so those helpers don't need long parameter lists
// repeated at every nesting level.
type fileRenderContext struct {
	file      *descriptorpb.FileDescriptorProto
	params    *Params
	types     *TypeRegistry
	syntax    string
	baseMod   string
	basePkg   string
	protoPath string
}

// renderEnum renders a single enum module given its already-qualified parent
// module name / full name and its SourceCodeInfo path.
func (ctx *fileRenderContext) renderEnum(enum *descriptorpb.EnumDescriptorProto, parentMod, parentFullName string, path []int32) (renderedModule, error) {
	modName := qualifyModName(parentMod, CamelizeEach(enum.GetName()))
	fullName := qualifyFullName(parentFullName, enum.GetName())

	var docComment string
	if ctx.params.IncludeDocs {
		docComment = ExtractDocComment(ctx.file.GetSourceCodeInfo(), path)
	}

	text, err := RenderEnum(enum, modName, fullName, ctx.syntax, ctx.params.IncludeDocs, docComment, ctx.params.GenDescriptors)
	if err != nil {
		return renderedModule{}, err
	}
	return renderedModule{modName: modName, text: text}, nil
}

// renderService renders a single service's paired .Service/.Stub modules
// given the file's base module name / base package and the service's own
// SourceCodeInfo path (e.g. [6, i] for file-level service i). The pair is
// returned as ONE renderedModule keyed by the bare service module name (no
// .Service/.Stub suffix), matching the escript, which emits both defmodules
// into a single file derived from that bare name.
func (ctx *fileRenderContext) renderService(svc *descriptorpb.ServiceDescriptorProto, path []int32) ([]renderedModule, error) {
	modName := qualifyModName(ctx.baseMod, CamelizeEach(svc.GetName()))
	serviceModName := modName + ".Service"
	stubModName := modName + ".Stub"
	fullName := qualifyFullName(ctx.basePkg, svc.GetName())

	var docComment string
	if ctx.params.IncludeDocs {
		docComment = ExtractDocComment(ctx.file.GetSourceCodeInfo(), path)
	}

	var protoSource string
	if ctx.params.GenProtoSource {
		protoSource = ctx.protoPath
	}

	serviceText, stubText, err := RenderServiceModules(svc, serviceModName, stubModName, fullName, ctx.params.IncludeDocs, docComment, ctx.params.GenDescriptors, protoSource, ctx.types)
	if err != nil {
		return nil, err
	}

	return []renderedModule{
		{modName: modName, text: serviceText + "\n\n" + stubText},
	}, nil
}

// renderNestedEnumsPreOrder implements module-ordering section 2's
// depth-first pre-order enum walk for a single top-level message's subtree:
// it does NOT render msg's own EnumType inline with msg - it returns the
// flat, ordered list of every nested enum's rendered text found anywhere in
// msg's NestedType tree (including msg's own direct EnumType), regardless of
// msg's own IsRenderable eligibility.
//
// path is msg's own SourceCodeInfo path (e.g. [4, i] for a top-level message
// or [4, i, 3, k] for a nested message k levels down).
func (ctx *fileRenderContext) renderNestedEnumsPreOrder(msg *descriptorpb.DescriptorProto, parentMod, parentFullName string, path []int32) ([]renderedModule, error) {
	modName := qualifyModName(parentMod, CamelizeEach(msg.GetName()))
	fullName := qualifyFullName(parentFullName, msg.GetName())

	var out []renderedModule

	for i, enum := range msg.GetEnumType() {
		enumPath := append(slices.Clone(path), 4, int32(i))
		m, err := ctx.renderEnum(enum, modName, fullName, enumPath)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}

	for i, nested := range msg.GetNestedType() {
		nestedPath := append(slices.Clone(path), 3, int32(i))
		nestedModules, err := ctx.renderNestedEnumsPreOrder(nested, modName, fullName, nestedPath)
		if err != nil {
			return nil, err
		}
		out = append(out, nestedModules...)
	}

	return out, nil
}

// renderMessageSubtreePostOrder implements module-ordering section 3: it
// renders msg's nested messages first (depth-first, NestedType declaration
// order), then msg's own body. Every message now renders (Phase 5 lifted the
// prior extension-based exclusion - see message.go's former IsRenderable).
// Nested enums are never rendered here (handled entirely by
// renderNestedEnumsPreOrder); message-embedded PbExtension submodules are
// never rendered here either (handled entirely, deferred, by
// renderMessageExtensionsDeferred).
func (ctx *fileRenderContext) renderMessageSubtreePostOrder(msg *descriptorpb.DescriptorProto, parentMod, parentFullName string, path []int32) ([]renderedModule, error) {
	modName := qualifyModName(parentMod, CamelizeEach(msg.GetName()))
	fullName := qualifyFullName(parentFullName, msg.GetName())

	var out []renderedModule

	for i, nested := range msg.GetNestedType() {
		nestedPath := append(slices.Clone(path), 3, int32(i))
		nestedModules, err := ctx.renderMessageSubtreePostOrder(nested, modName, fullName, nestedPath)
		if err != nil {
			return nil, err
		}
		out = append(out, nestedModules...)
	}

	var docComment string
	if ctx.params.IncludeDocs {
		docComment = ExtractDocComment(ctx.file.GetSourceCodeInfo(), path)
	}

	var protoSource string
	if ctx.params.GenProtoSource {
		protoSource = ctx.protoPath
	}

	text, err := RenderMessage(msg, modName, fullName, ctx.syntax, ctx.params.IncludeDocs, docComment, ctx.params.GenDescriptors, protoSource, ctx.params.TransformModule, ctx.types)
	if err != nil {
		return nil, err
	}
	out = append(out, renderedModule{modName: modName, text: text})

	return out, nil
}

// renderMessageExtensionsDeferred implements module-ordering tier 3.5: it
// walks msg's own NestedType tree (any depth, declaration order) and msg
// itself, collecting each message's RenderMessageExtension output (its own
// embedded extend fields merged into one <Message>.PbExtension submodule) in
// declaration order among themselves. Per generator.go's tier-ordering doc
// comment, this is rendered by the caller as one contiguous block AFTER
// every tier-3 message body, not inline alongside msg's own position.
func (ctx *fileRenderContext) renderMessageExtensionsDeferred(msg *descriptorpb.DescriptorProto, parentMod, syntax string) ([]renderedModule, error) {
	modName := qualifyModName(parentMod, CamelizeEach(msg.GetName()))
	extModName := qualifyModName(modName, "PbExtension")

	var out []renderedModule

	if text, ok, err := RenderMessageExtension(msg, extModName, syntax, ctx.params.IncludeDocs, ctx.types); err != nil {
		return nil, err
	} else if ok {
		out = append(out, renderedModule{modName: extModName, text: text})
	}

	for _, nested := range msg.GetNestedType() {
		nestedModules, err := ctx.renderMessageExtensionsDeferred(nested, modName, syntax)
		if err != nil {
			return nil, err
		}
		out = append(out, nestedModules...)
	}

	return out, nil
}

func qualifyModName(baseModName, localModName string) string {
	if baseModName == "" {
		return localModName
	}
	return baseModName + "." + localModName
}

func qualifyFullName(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// outputPath replicates the default-mode (one_file_per_module=false) output
// path algorithm from the default (one_file_per_module=false) output layout,
// including the deliberate path-doubling bug: whenever a package or
// package_prefix is set, the base module's underscored path is prepended a
// second time on top of the proto file's own directory layout. We replicate
// this byte-for-byte rather than fix it, per the upstream-confirmed decision
// to keep this quirk.
func outputPath(file *descriptorpb.FileDescriptorProto, params *Params) string {
	rootname := pathRootname(file.GetName())

	baseModName := ModName(file, params.PackagePrefix, nil)
	if baseModName == "" {
		return rootname + ".pb.ex"
	}

	return macroUnderscore(baseModName) + "/" + rootname + ".pb.ex"
}

func pathRootname(name string) string {
	ext := ".proto"
	return strings.TrimSuffix(name, ext)
}

// modulePath derives an output file path purely from a fully-qualified
// Elixir module name (e.g. "My.Test.PbExtension" -> "my/test/pb_extension.pb.ex"),
// unlike outputPath, which derives its filename from a single input proto
// file's own basename. Used for the file-level merged PbExtension module,
// which has no single originating .proto file of its own (see
// renderFileExtensionGroups), and for every module's own file in
// one_file_per_module=true mode (see generateFiles).
func modulePath(modName string) string {
	return macroUnderscore(modName) + ".pb.ex"
}

// macroUnderscore mirrors Macro.underscore/1: PascalCase segments become
// snake_case, dots become path separators. Segments are processed
// independently (matching Macro.underscore's own "." handling, which
// restarts the algorithm fresh after each dot), so splitting on "." first is
// equivalent to the real implementation.
func macroUnderscore(modName string) string {
	segments := strings.Split(modName, ".")
	for i, segment := range segments {
		segments[i] = underscoreSegment(segment)
	}
	return strings.Join(segments, "/")
}

// underscoreSegment mirrors Elixir's Macro.underscore/1 do_underscore/2
// clause-by-clause (lib/elixir/lib/macro.ex), which is NOT simply "insert _
// before every uppercase letter": consecutive uppercase runs (acronyms) are
// preserved as a block, and the boundary out of an acronym is decided by a
// two-character lookahead, not just the previous character. For example
// "SomeAPIThing" -> "some_api_thing" (no underscore between A/P/I) while
// "V2Message" -> "v2_message" (underscore after a digit).
func underscoreSegment(segment string) string {
	if segment == "" {
		return ""
	}

	runes := []rune(segment)
	var b strings.Builder
	b.WriteRune(toLowerASCII(runes[0]))
	prev := runes[0]

	for i := 1; i < len(runes); i++ {
		h := runes[i]

		if !isUpperASCII(h) {
			b.WriteRune(h)
			prev = h
			continue
		}

		if i+1 < len(runes) {
			t := runes[i+1]
			if !isUpperASCII(t) && !isDigitASCII(t) && t != '.' && t != '_' {
				b.WriteByte('_')
				b.WriteRune(toLowerASCII(h))
				b.WriteRune(toLowerASCII(t))
				prev = t
				i++
				continue
			}
		}

		if !isUpperASCII(prev) && prev != '_' {
			b.WriteByte('_')
		}
		b.WriteRune(toLowerASCII(h))
		prev = h
	}

	return b.String()
}

func isUpperASCII(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isDigitASCII(r rune) bool {
	return r >= '0' && r <= '9'
}

func toLowerASCII(r rune) rune {
	if isUpperASCII(r) {
		return r - 'A' + 'a'
	}
	return r
}

// generate implements the protoc plugin contract's code-generation step:
// parse parameters, render Phase 1 (top-level enum) output, and surface any
// error as a CodeGeneratorResponse.Error rather than panicking.
func generate(req *pluginpb.CodeGeneratorRequest) *pluginpb.CodeGeneratorResponse {
	params, err := ParseParams(req.GetParameter())
	if err != nil {
		return &pluginpb.CodeGeneratorResponse{
			Error: proto.String(err.Error()),
		}
	}

	files, err := generateFiles(req, params)
	if err != nil {
		return &pluginpb.CodeGeneratorResponse{
			Error: proto.String(err.Error()),
		}
	}

	return &pluginpb.CodeGeneratorResponse{
		SupportedFeatures: proto.Uint64(uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)),
		File:              files,
	}
}
