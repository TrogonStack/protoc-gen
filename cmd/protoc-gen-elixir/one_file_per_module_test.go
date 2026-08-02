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

// TestOneFilePerModuleWithGRPC guards the escript-matching file layout: the
// .Service and .Stub defmodules for a given service are emitted together in
// ONE file, named from the bare service module name (no .Service/.Stub
// suffix) via Macro.underscore. There's no golden fixture combining
// one_file_per_module=true with plugins=grpc (gen_goldens.sh never generates
// one), so this asserts against testdata/golden/grpc/test/service.pb.ex -
// the proven-correct rendering of the same test.proto/service.proto pair -
// which is exactly the expected content here since that fixture already
// pairs both defmodules in one blob.
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
end
`

	file, ok := findGeneratedFile(resp.GetFile(), "test/test_service.pb.ex")
	require.True(t, ok, "expected test/test_service.pb.ex among generated files")
	assert.Equal(t, want, file.GetContent())
}
