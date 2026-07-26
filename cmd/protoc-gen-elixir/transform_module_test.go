package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

// TestGenerateTransformModuleIntegration is the byte-exact whole-fixture
// counterpart for Phase 8's transform_module=... support: generating
// test.proto alone (mirroring gen_goldens.sh's own
// `gen transform_module "transform_module=My.App.Transform" "$PROTO_DIR/test.proto"`
// invocation, which does NOT pass service.proto) must produce exactly two
// files - test.pb.ex and the file-level merged pb_extension.pb.ex - each
// byte-identical to testdata/golden/transform_module/test/*.pb.ex.
func TestGenerateTransformModuleIntegration(t *testing.T) {
	t.Parallel()

	fds := buildDescriptorSet(t, "test.proto")

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: []string{"test.proto"},
		Parameter:      proto.String("transform_module=My.App.Transform"),
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())
	require.Len(t, resp.GetFile(), 2)

	testFile, ok := findGeneratedFile(resp.GetFile(), "test/test.pb.ex")
	require.True(t, ok, "expected test/test.pb.ex among generated files")

	wantTest, err := os.ReadFile(filepath.Join("testdata", "golden", "transform_module", "test", "test.pb.ex"))
	require.NoError(t, err)
	assert.Equal(t, string(wantTest), testFile.GetContent())

	pbExtensionFile, ok := findGeneratedFile(resp.GetFile(), "test/pb_extension.pb.ex")
	require.True(t, ok, "expected test/pb_extension.pb.ex among generated files")

	wantPbExtension, err := os.ReadFile(filepath.Join("testdata", "golden", "transform_module", "test", "pb_extension.pb.ex"))
	require.NoError(t, err)
	assert.Equal(t, string(wantPbExtension), pbExtensionFile.GetContent())
}
