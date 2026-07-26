package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

// TestOneFilePerModuleIntegration mirrors testdata/gen_goldens.sh's
// `gen one_file_per_module "one_file_per_module=true,include_docs=true" test.proto service.proto`
// invocation (no plugins=grpc, so service.proto's service produces no
// output) and asserts BYTE-IDENTICAL content for every file under
// testdata/golden/one_file_per_module/test/, walking the golden directory
// tree rather than hardcoding each path so newly added fixtures are picked
// up automatically.
func TestOneFilePerModuleIntegration(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSetMulti(t, "test.proto", "service.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"test.proto", "service.proto"},
		Parameter:      proto.String("one_file_per_module=true,include_docs=true"),
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())

	goldenRoot := filepath.Join("testdata", "golden", "one_file_per_module")

	var goldenPaths []string
	err := filepath.WalkDir(goldenRoot, func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(goldenRoot, path)
		require.NoError(t, err)
		goldenPaths = append(goldenPaths, filepath.ToSlash(rel))
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, goldenPaths)

	for _, rel := range goldenPaths {
		t.Run(rel, func(t *testing.T) {
			file, ok := findGeneratedFile(resp.GetFile(), rel)
			require.True(t, ok, "expected %s among generated files", rel)

			want, err := os.ReadFile(filepath.Join(goldenRoot, filepath.FromSlash(rel)))
			require.NoError(t, err)
			assert.Equal(t, string(want), file.GetContent())
		})
	}

	assert.Len(t, resp.GetFile(), len(goldenPaths), "generated file count should match golden fixture count exactly")
}

// TestOneFilePerModuleWithGRPC guards against a regression where the
// .Service/.Stub pair was split apart via strings.Cut(text, "\n\n") on
// RenderService's combined output: the .Service module body itself contains
// internal blank lines (e.g. after a multi-line @moduledoc, before "use
// GRPC.Service") that appear before the true Service/Stub boundary, so
// cutting on the first "\n\n" silently truncated the .Service file and
// prepended garbage to the .Stub file. There's no golden fixture combining
// one_file_per_module=true with plugins=grpc (gen_goldens.sh never generates
// one), so this asserts against testdata/golden/grpc/test/service.pb.ex -
// the proven-correct single-file rendering of the same test.proto/
// service.proto pair - split at its own known-correct module boundary.
func TestOneFilePerModuleWithGRPC(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSetMulti(t, "test.proto", "service.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"service.proto"},
		Parameter:      proto.String("one_file_per_module=true,plugins=grpc,include_docs=true"),
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())

	wantService := `defmodule Test.TestService.Service do
  @moduledoc """
  An example test service that has
  a test method. It expects a Request
  and returns a Reply.
  """

  use GRPC.Service, name: "test.TestService", protoc_gen_elixir_version: "0.17.0"

  rpc :test, Test.Request, Test.Reply
end
`
	wantStub := `defmodule Test.TestService.Stub do
  use GRPC.Stub, service: Test.TestService.Service
end
`

	serviceFile, ok := findGeneratedFile(resp.GetFile(), "test/test_service/service.pb.ex")
	require.True(t, ok, "expected test/test_service/service.pb.ex among generated files")
	assert.Equal(t, wantService, serviceFile.GetContent())

	stubFile, ok := findGeneratedFile(resp.GetFile(), "test/test_service/stub.pb.ex")
	require.True(t, ok, "expected test/test_service/stub.pb.ex among generated files")
	assert.Equal(t, wantStub, stubFile.GetContent())
}
