package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

// assertMatchesReference runs both the real elixir-protobuf/protobuf escript
// (pinned at the HEAD commit of elixir-protobuf/protobuf) and this native plugin
// against the same testdata/proto/ inputs and elixir_opt string, then asserts
// the two produce byte-identical output file sets.
//
// Requires the PROTOC_GEN_ELIXIR_ESCRIPT environment variable to point at a
// built reference escript (see testdata/gen_goldens.sh's header comment for
// how to build one); the test is skipped if unset, mirroring the existing
// protoc-not-on-PATH skip pattern in generator_test.go.
func assertMatchesReference(t *testing.T, elixirOpt string, protoFiles ...string) {
	t.Helper()

	escriptPath := os.Getenv("PROTOC_GEN_ELIXIR_ESCRIPT")
	if escriptPath == "" {
		t.Skip("PROTOC_GEN_ELIXIR_ESCRIPT not set")
	}

	want := runReferenceEscript(t, escriptPath, elixirOpt, protoFiles...)
	got := runNativePlugin(t, elixirOpt, protoFiles...)

	assert.Equal(t, want, got)
}

// runReferenceEscript invokes protoc with the real escript as the
// --elixir_out plugin and returns a map of output-relative-path -> content.
func runReferenceEscript(t *testing.T, escriptPath, elixirOpt string, protoFiles ...string) map[string]string {
	t.Helper()

	protocPath, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc not found on PATH")
	}

	protoDir := testdataProtoDir(t)
	outDir := t.TempDir()

	elixirOut := outDir
	if elixirOpt != "" {
		elixirOut = elixirOpt + ":" + outDir
	}

	args := []string{
		"-I", protoDir,
		"--plugin=protoc-gen-elixir=" + escriptPath,
		"--elixir_out=" + elixirOut,
	}
	for _, f := range protoFiles {
		args = append(args, filepath.Join(protoDir, f))
	}

	cmd := exec.Command(protocPath, args...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "protoc output: %s", output)

	return readGeneratedFiles(t, outDir)
}

// runNativePlugin builds a CodeGeneratorRequest from testdata/proto/ (via a
// real protoc --descriptor_set_out invocation, so cross-file imports resolve
// exactly as they would in production) and runs this package's own plugin
// binary against it, returning a map of response file Name -> Content.
func runNativePlugin(t *testing.T, elixirOpt string, protoFiles ...string) map[string]string {
	t.Helper()

	fds := buildDescriptorSetMulti(t, protoFiles...)

	req := &pluginpb.CodeGeneratorRequest{
		ProtoFile:      fds.GetFile(),
		FileToGenerate: protoFiles,
	}
	if elixirOpt != "" {
		req.Parameter = proto.String(elixirOpt)
	}

	resp := testGenerate(t, req)
	require.Empty(t, resp.GetError())

	out := make(map[string]string, len(resp.GetFile()))
	for _, f := range resp.GetFile() {
		out[f.GetName()] = f.GetContent()
	}
	return out
}

func readGeneratedFiles(t *testing.T, root string) map[string]string {
	t.Helper()

	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(content)
		return nil
	})
	require.NoError(t, err)
	return out
}

func testdataProtoDir(t *testing.T) string {
	t.Helper()

	repoRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	return filepath.Join(repoRoot, "cmd", "protoc-gen-elixir", "testdata", "proto")
}
