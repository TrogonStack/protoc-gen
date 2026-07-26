package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// buildDescriptorSet runs the real protoc binary against testdata/proto to
// produce a FileDescriptorSet with source_code_info attached, exercising
// the same path a real `protoc --elixir_out=...` invocation would take.
func buildDescriptorSet(t *testing.T, protoFile string) *descriptorpb.FileDescriptorSet {
	t.Helper()
	return buildDescriptorSetMulti(t, protoFile)
}

// buildDescriptorSetMulti is buildDescriptorSet for a request spanning
// multiple entry-point proto files (e.g. test.proto + service.proto, which
// share a package and are conventionally generated together).
func buildDescriptorSetMulti(t *testing.T, protoFiles ...string) *descriptorpb.FileDescriptorSet {
	t.Helper()

	protocPath, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc not found on PATH")
	}

	protoDir := testdataProtoDir(t)
	outPath := filepath.Join(t.TempDir(), "descriptor_set.pb")

	args := []string{
		"-I", protoDir,
		"--include_source_info",
		"--include_imports",
		"--descriptor_set_out=" + outPath,
	}
	for _, f := range protoFiles {
		args = append(args, filepath.Join(protoDir, f))
	}

	cmd := exec.Command(protocPath, args...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "protoc output: %s", output)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)

	var fds descriptorpb.FileDescriptorSet
	require.NoError(t, proto.Unmarshal(data, &fds))
	return &fds
}

func TestMacroUnderscore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{"Foo.Bar.MyMessage", "foo/bar/my_message"},
		{"HTTPServer", "http_server"},
		{"SomeAPIThing", "some_api_thing"},
		{"V2Message", "v2_message"},
		{"ABC", "abc"},
		{"A", "a"},
		{"Foo_Bar", "foo__bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, macroUnderscore(tt.name))
		})
	}
}

func TestGenerateEnumsIntegration(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSet(t, "test.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"test.proto"},
		Parameter:      proto.String("include_docs=true,package_prefix=my"),
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())
	// test.proto now produces two files: test.pb.ex (this test's subject)
	// and my/test/pb_extension.pb.ex (test.proto's top-level extend block,
	// Phase 5 scope - see TestGenerateFileLevelExtensionIntegration).
	require.Len(t, resp.GetFile(), 2)

	file, ok := findGeneratedFile(resp.GetFile(), "my/test/test.pb.ex")
	require.True(t, ok, "expected my/test/test.pb.ex among generated files")

	content := file.GetContent()

	hatType := `defmodule My.Test.HatType do
  @moduledoc """
  This enum represents different kinds of hats.
  """

  use Protobuf,
    enum: true,
    full_name: "test.HatType",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :FEDORA, 1
  field :FEZ, 2
end`

	days := `defmodule My.Test.Days do
  @moduledoc """
  This enum represents days of the week.
  """

  use Protobuf,
    enum: true,
    full_name: "test.Days",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :MONDAY, 1
  field :TUESDAY, 2
  field :LUNDI, 1
end`

	mapEnum := `defmodule My.Test.MapEnum do
  use Protobuf,
    enum: true,
    full_name: "test.MapEnum",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :HELLO, 0
  field :WORLD, 2
end`

	require.Contains(t, content, hatType)
	require.Contains(t, content, days)
	require.Contains(t, content, mapEnum)

	hatTypeIdx := strings.Index(content, hatType)
	daysIdx := strings.Index(content, days)
	mapEnumIdx := strings.Index(content, mapEnum)

	assert.Less(t, hatTypeIdx, daysIdx, "HatType must render before Days")
	assert.Less(t, daysIdx, mapEnumIdx, "Days must render before MapEnum")
}

// TestGenerateFlatMessagesIntegration exercises the full generate() pipeline
// against test.proto, which mixes messages within Phase 1/2's flat-scalar
// scope (Options, OtherReplyExtensions) with messages well outside it. As of
// Phase 5, extension ranges and message-embedded extend blocks no longer
// exclude a message from rendering (see message.go's removed IsRenderable) -
// so Reply, OtherBase, OldReply, and ReplyExtensions all now render too.
func TestGenerateFlatMessagesIntegration(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSet(t, "test.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"test.proto"},
		Parameter:      proto.String("package_prefix=my"),
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())
	// test.proto now produces two files: test.pb.ex and
	// my/test/pb_extension.pb.ex (its top-level extend block).
	require.Len(t, resp.GetFile(), 2)

	file, ok := findGeneratedFile(resp.GetFile(), "my/test/test.pb.ex")
	require.True(t, ok, "expected my/test/test.pb.ex among generated files")
	content := file.GetContent()

	options := `defmodule My.Test.Options do
  @moduledoc false

  use Protobuf, full_name: "test.Options", protoc_gen_elixir_version: "0.17.0", syntax: :proto2

  field :opt1, 1, optional: true, type: :string, deprecated: true
end`

	otherReplyExtensions := `defmodule My.Test.OtherReplyExtensions do
  @moduledoc false

  use Protobuf,
    full_name: "test.OtherReplyExtensions",
    protoc_gen_elixir_version: "0.17.0",
    syntax: :proto2

  field :key, 1, optional: true, type: :int32
end`

	require.Contains(t, content, options)
	require.Contains(t, content, otherReplyExtensions)

	// Messages carrying extension ranges / message-embedded extend blocks
	// (Phase 5 scope) now render too - previously silently absent.
	assert.Contains(t, content, "defmodule My.Test.Reply do")
	assert.Contains(t, content, "defmodule My.Test.OtherBase do")
	assert.Contains(t, content, "defmodule My.Test.OldReply do")
	assert.Contains(t, content, "defmodule My.Test.ReplyExtensions do")
	assert.Contains(t, content, "defmodule My.Test.Reply.Entry do")

	// Request, Communique, and MapInput carry no extension ranges of their
	// own, so Phase 3 renders them even though their parent file also
	// contains messages with extension constructs.
	assert.Contains(t, content, "defmodule My.Test.Request do")
	assert.Contains(t, content, "defmodule My.Test.Communique do")
	assert.Contains(t, content, "defmodule My.Test.MapInput do")

	// Enums must still render before messages within the same file.
	hatTypeIdx := strings.Index(content, "defmodule My.Test.HatType do")
	optionsIdx := strings.Index(content, "defmodule My.Test.Options do")
	require.GreaterOrEqual(t, hatTypeIdx, 0)
	require.GreaterOrEqual(t, optionsIdx, 0)
	assert.Less(t, hatTypeIdx, optionsIdx, "enums must render before messages")

	otherReplyExtensionsIdx := strings.Index(content, "defmodule My.Test.OtherReplyExtensions do")
	assert.Less(t, otherReplyExtensionsIdx, optionsIdx, "OtherReplyExtensions must render before Options (declaration order)")
}

// TestGenerateNestedModuleOrderingIntegration verifies the module ordering
// rule against test.proto's full worked example: (1) all file-level enums
// in declaration order, (2) every nested enum at any depth from every
// top-level message, depth-first pre-order, one top-level message's tree
// fully before advancing to the next, rendered as one contiguous block
// before any message body, (3) message bodies in depth-first post-order per
// top-level message, in normal declaration order across top-level messages,
// (4) message-embedded PbExtension submodules, deferred to one contiguous
// block after every message body - see
// TestGenerateReplyExtensionsPbExtensionDeferredPosition for that specific
// assertion.
func TestGenerateNestedModuleOrderingIntegration(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSet(t, "test.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"test.proto"},
		Parameter:      proto.String("package_prefix=my"),
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())
	require.Len(t, resp.GetFile(), 2)

	file, ok := findGeneratedFile(resp.GetFile(), "my/test/test.pb.ex")
	require.True(t, ok, "expected my/test/test.pb.ex among generated files")
	content := file.GetContent()

	// The exact worked-example ordering, now including the extension-bearing
	// messages Reply, OtherBase, OldReply, and ReplyExtensions (Phase 5).
	wantOrder := []string{
		"defmodule My.Test.HatType do",
		"defmodule My.Test.Days do",
		"defmodule My.Test.MapEnum do",
		"defmodule My.Test.Request.Color do",
		"defmodule My.Test.Reply.Entry.Game do",
		"defmodule My.Test.Request.SomeGroup do",
		"defmodule My.Test.Request.NameMappingEntry do",
		"defmodule My.Test.Request.MsgMappingEntry do",
		"defmodule My.Test.Request do",
		"defmodule My.Test.Reply.Entry do",
		"defmodule My.Test.Reply do",
		"defmodule My.Test.OtherBase do",
		"defmodule My.Test.ReplyExtensions do",
		"defmodule My.Test.OtherReplyExtensions do",
		"defmodule My.Test.OldReply do",
		"defmodule My.Test.Communique.SomeGroup do",
		"defmodule My.Test.Communique.Delta do",
		"defmodule My.Test.Communique do",
		"defmodule My.Test.Options do",
		"defmodule My.Test.MapInput.Int32MapEntry do",
		"defmodule My.Test.MapInput.EnumMapEntry do",
		"defmodule My.Test.MapInput do",
		"defmodule My.Test.ReplyExtensions.PbExtension do",
	}

	var indices []int
	for _, marker := range wantOrder {
		idx := strings.Index(content, marker)
		require.GreaterOrEqualf(t, idx, 0, "expected to find %q in output", marker)
		indices = append(indices, idx)
	}

	for i := 1; i < len(indices); i++ {
		assert.Lessf(t, indices[i-1], indices[i],
			"expected %q to render before %q", wantOrder[i-1], wantOrder[i])
	}

	// All nested enums (from every top-level message, at any depth) must
	// render before any message body appears anywhere in the file - i.e.
	// the last nested enum module must precede the first message body
	// module. Request.NameMappingEntry/MsgMappingEntry are map-entry
	// messages (not enums), so they belong to the message-body post-order
	// pass and correctly render as Request's nested-message children,
	// immediately before Request's own body.
	lastNestedEnumIdx := strings.Index(content, "defmodule My.Test.Reply.Entry.Game do")
	firstMessageBodyIdx := strings.Index(content, "defmodule My.Test.Request do")
	assert.Less(t, lastNestedEnumIdx, firstMessageBodyIdx,
		"all nested enums must render before any top-level message body")
}

// TestGenerateReplyExtensionsPbExtensionDeferredPosition isolates the
// tier-4 (message-embedded PbExtension) deferred-position claim: even
// though ReplyExtensions is DECLARED before OtherReplyExtensions/OldReply/
// Communique/Options/MapInput in test.proto, its PbExtension submodule
// renders as the LAST module in the file, after MapInput - verified by
// position against
// testdata/golden/package_prefix/my/test/test.pb.ex:513-521.
func TestGenerateReplyExtensionsPbExtensionDeferredPosition(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSet(t, "test.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"test.proto"},
		Parameter:      proto.String("package_prefix=my"),
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())

	file, ok := findGeneratedFile(resp.GetFile(), "my/test/test.pb.ex")
	require.True(t, ok, "expected my/test/test.pb.ex among generated files")
	content := file.GetContent()

	replyExtensionsIdx := strings.Index(content, "defmodule My.Test.ReplyExtensions do")
	mapInputIdx := strings.Index(content, "defmodule My.Test.MapInput do")
	pbExtensionIdx := strings.Index(content, "defmodule My.Test.ReplyExtensions.PbExtension do")

	require.GreaterOrEqual(t, replyExtensionsIdx, 0)
	require.GreaterOrEqual(t, mapInputIdx, 0)
	require.GreaterOrEqual(t, pbExtensionIdx, 0)

	assert.Less(t, replyExtensionsIdx, mapInputIdx,
		"ReplyExtensions is declared before MapInput in test.proto")
	assert.Greater(t, pbExtensionIdx, mapInputIdx,
		"ReplyExtensions.PbExtension must render AFTER MapInput, deferred to end-of-file")

	assert.True(t, strings.HasSuffix(strings.TrimRight(content, "\n"), "end"),
		"ReplyExtensions.PbExtension must be the last module in the file")
}

// TestGenerateFileLevelExtensionIntegration is the byte-exact counterpart to
// TestGenerateEnumsIntegration/TestGenerateFlatMessagesIntegration's mention
// of my/test/pb_extension.pb.ex: test.proto's single top-level `extend
// Reply { tag = 103; donut = 106; }` block is rendered into its own
// synthetic file, grouped under the base module name (My.Test) shared by
// every file being generated in this request, matching
// testdata/golden/package_prefix/my/test/pb_extension.pb.ex byte-for-byte.
func TestGenerateFileLevelExtensionIntegration(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSet(t, "test.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"test.proto"},
		Parameter:      proto.String("include_docs=true,package_prefix=my"),
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())
	require.Len(t, resp.GetFile(), 2)

	file, ok := findGeneratedFile(resp.GetFile(), "my/test/pb_extension.pb.ex")
	require.True(t, ok, "expected my/test/pb_extension.pb.ex among generated files")

	want, err := os.ReadFile(filepath.Join("testdata", "golden", "package_prefix", "my", "test", "pb_extension.pb.ex"))
	require.NoError(t, err)

	assert.Equal(t, string(want), file.GetContent())
}

// TestGenerateFileLevelExtensionEmptyGroupIntegration confirms the inverse
// of TestGenerateFileLevelExtensionIntegration: service.proto defines no
// top-level extend block at all, so generating it together with test.proto
// must produce exactly one pb_extension.pb.ex (grouped under the shared
// My.Test base module name, from test.proto's own extend block alone) - not
// two, and not an empty one contributed by service.proto. This exercises
// renderFileExtensionGroups' empty-group skip (a base module name with zero
// merged extend fields across all its files must not synthesize a file).
func TestGenerateFileLevelExtensionEmptyGroupIntegration(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSetMulti(t, "test.proto", "service.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"test.proto", "service.proto"},
		Parameter:      proto.String("plugins=grpc,package_prefix=my"),
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())

	var pbExtensionFiles []string
	for _, f := range resp.GetFile() {
		if strings.HasSuffix(f.GetName(), "pb_extension.pb.ex") {
			pbExtensionFiles = append(pbExtensionFiles, f.GetName())
		}
	}
	assert.Equal(t, []string{"my/test/pb_extension.pb.ex"}, pbExtensionFiles,
		"service.proto contributes no top-level extend fields, so only test.proto's own pb_extension.pb.ex must be produced")
}

// TestGenerateNoPackageIntegration is the true whole-file differential test
// Phase 3 needed: no_package.proto's only message (NoPackageMessage) has a
// single map field and no extensions, so - unlike test.proto, which still
// has Phase 5 extension constructs out of scope - the entire generated file
// is within Phase 1+3's combined scope and can be asserted byte-for-byte
// against testdata/golden/no_package/no_package.pb.ex, no substring
// golden-picking required.
func TestGenerateNoPackageIntegration(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSet(t, "no_package.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"no_package.proto"},
		Parameter:      proto.String("include_docs=true"),
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())
	require.Len(t, resp.GetFile(), 1)

	wantPath := filepath.Join("testdata", "golden", "no_package", "no_package.pb.ex")
	want, err := os.ReadFile(wantPath)
	require.NoError(t, err)

	assert.Equal(t, string(want), resp.GetFile()[0].GetContent())
}

// generateTestServiceFiles is a shared helper for the plugins=grpc
// integration tests below: it builds a descriptor set from test.proto +
// service.proto (service.proto imports test.proto and cross-references its
// Request/Reply messages) and runs generate() with the given elixir_opt
// parameter string.
func generateTestServiceFiles(t *testing.T, elixirOpt string) *pluginpb.CodeGeneratorResponse {
	t.Helper()

	fds := buildDescriptorSetMulti(t, "test.proto", "service.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"test.proto", "service.proto"},
	}
	if elixirOpt != "" {
		req.Parameter = proto.String(elixirOpt)
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())
	return resp
}

func findGeneratedFile(files []*pluginpb.CodeGeneratorResponse_File, name string) (*pluginpb.CodeGeneratorResponse_File, bool) {
	for _, f := range files {
		if f.GetName() == name {
			return f, true
		}
	}
	return nil, false
}

// TestGenerateServiceIncludeDocsIntegration mirrors testdata/gen_goldens.sh's
// `gen grpc "plugins=grpc,include_docs=true" test.proto service.proto`
// invocation and asserts test/service.pb.ex matches
// testdata/golden/grpc/test/service.pb.ex byte-for-byte. This is a stronger
// check than a substring assertion (justified here, unlike
// TestGenerateFlatMessagesIntegration's test.pb.ex checks, because
// service.pb.ex itself has no Phase 5 extension constructs - those all live
// in test.pb.ex and the Phase-5-only pb_extension.pb.ex file, neither of
// which this test inspects).
func TestGenerateServiceIncludeDocsIntegration(t *testing.T) {
	t.Parallel()

	resp := generateTestServiceFiles(t, "plugins=grpc,include_docs=true")

	file, ok := findGeneratedFile(resp.GetFile(), "test/service.pb.ex")
	require.True(t, ok, "expected test/service.pb.ex among generated files")

	want, err := os.ReadFile(filepath.Join("testdata", "golden", "grpc", "test", "service.pb.ex"))
	require.NoError(t, err)

	assert.Equal(t, string(want), file.GetContent())
}

// TestGenerateServiceProtoSourceIntegration mirrors testdata/gen_goldens.sh's
// `gen grpc_proto_source "plugins=grpc,gen_proto_source=true" test.proto
// service.proto` invocation (note: NOT include_docs=true this time) and
// asserts test/service.pb.ex matches
// testdata/golden/grpc_proto_source/test/service.pb.ex byte-for-byte - this
// is the fixture that exercises def proto_source() placement and the Stub
// module's @moduledoc false (include_docs defaults to false here).
//
// It also asserts test/test.pb.ex byte-for-byte against
// testdata/golden/grpc_proto_source/test/test.pb.ex, which is the only
// fixture in the corpus that exercises Test.ReplyExtensions.PbExtension
// (the message-embedded extension submodule) with include_docs=false -
// this is a regression check for RenderMessageExtension's includeDocs
// parameter: that module's "@moduledoc false" line (test.pb.ex line 615)
// was silently never emitted before that parameter was threaded through,
// and no other existing test asserted this file's content, so the bug went
// uncaught until direct fixture inspection.
func TestGenerateServiceProtoSourceIntegration(t *testing.T) {
	t.Parallel()

	resp := generateTestServiceFiles(t, "plugins=grpc,gen_proto_source=true")

	file, ok := findGeneratedFile(resp.GetFile(), "test/service.pb.ex")
	require.True(t, ok, "expected test/service.pb.ex among generated files")

	want, err := os.ReadFile(filepath.Join("testdata", "golden", "grpc_proto_source", "test", "service.pb.ex"))
	require.NoError(t, err)

	assert.Equal(t, string(want), file.GetContent())

	testFile, ok := findGeneratedFile(resp.GetFile(), "test/test.pb.ex")
	require.True(t, ok, "expected test/test.pb.ex among generated files")

	wantTest, err := os.ReadFile(filepath.Join("testdata", "golden", "grpc_proto_source", "test", "test.pb.ex"))
	require.NoError(t, err)

	assert.Equal(t, string(wantTest), testFile.GetContent())
}

// TestGenerateServiceWithoutGRPCPluginIntegration confirms services are
// silently absent (not erroring, not partially rendered) when plugins=grpc
// isn't requested - matching how every other ungated feature in this
// codebase degrades (e.g. IncludeDocs/GenProtoSource false is a silent
// no-op, never an error). test.proto still has enums/messages of its own,
// so test/test.pb.ex is still produced; only the service is missing.
func TestGenerateServiceWithoutGRPCPluginIntegration(t *testing.T) {
	t.Parallel()

	resp := generateTestServiceFiles(t, "")

	_, ok := findGeneratedFile(resp.GetFile(), "test/service.pb.ex")
	assert.False(t, ok, "service.pb.ex must not be generated without plugins=grpc")

	_, ok = findGeneratedFile(resp.GetFile(), "test/test.pb.ex")
	assert.True(t, ok, "test.pb.ex must still be generated (it has its own enums/messages)")
}

// TestGenerateServiceOnlyFileWithoutGRPCPluginIntegration confirms that
// service.proto ALONE (it defines nothing but a service - no enums, no
// messages) produces NO output file at all when plugins=grpc isn't
// requested, consistent with generateFiles' file-skip condition: a file
// with nothing generatable is skipped entirely rather than producing an
// empty file.
func TestGenerateServiceOnlyFileWithoutGRPCPluginIntegration(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSetMulti(t, "test.proto", "service.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"service.proto"},
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())
	assert.Empty(t, resp.GetFile(), "service.proto alone, without plugins=grpc, has nothing generatable and must be skipped entirely")
}
