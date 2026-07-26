package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestVersion(t *testing.T) {
	t.Parallel()

	stdout, stderr, exitCode := testRunProtocGenElixir(t, nil, "--version")
	assert.Equal(t, "", stderr.String())
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "protoc-gen-elixir")
	assert.Contains(t, stdout.String(), "commit:")
	assert.Contains(t, stdout.String(), "built:")
}

func TestHelp(t *testing.T) {
	t.Parallel()

	stdout, stderr, exitCode := testRunProtocGenElixir(t, nil, "--help")
	assert.Equal(t, "", stderr.String())
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "Flags:")
	assert.Contains(t, stdout.String(), "--version")
	assert.Contains(t, stdout.String(), "plugins")
	assert.Contains(t, stdout.String(), "package_prefix")
	assert.Contains(t, stdout.String(), "gen_proto_source")
}

func TestUnexpectedArgs(t *testing.T) {
	t.Parallel()

	stdout, stderr, exitCode := testRunProtocGenElixir(t, nil, "bogus")
	assert.Equal(t, "", stdout.String())
	assert.Contains(t, stderr.String(), "Flags:")
	assert.Equal(t, 1, exitCode)
}

func TestGenerateRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("no parameter", func(t *testing.T) {
		t.Parallel()

		resp := testGenerate(t, &pluginpb.CodeGeneratorRequest{})
		require.NotNil(t, resp)
		assert.Empty(t, resp.GetError())
		assert.Equal(t, uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL), resp.GetSupportedFeatures())
		assert.Empty(t, resp.GetFile())
	})

	t.Run("valid parameter", func(t *testing.T) {
		t.Parallel()

		resp := testGenerate(t, &pluginpb.CodeGeneratorRequest{
			Parameter: proto.String("plugins=grpc,include_docs=true"),
		})
		require.NotNil(t, resp)
		assert.Empty(t, resp.GetError())
	})

	t.Run("invalid parameter surfaces as CodeGeneratorResponse error", func(t *testing.T) {
		t.Parallel()

		resp := testGenerate(t, &pluginpb.CodeGeneratorRequest{
			Parameter: proto.String("package_prefix="),
		})
		require.NotNil(t, resp)
		assert.Equal(t, "package_prefix can't be empty", resp.GetError())
	})
}

func testGenerate(t *testing.T, req *pluginpb.CodeGeneratorRequest) *pluginpb.CodeGeneratorResponse {
	t.Helper()

	inputBytes, err := proto.Marshal(req)
	require.NoError(t, err)

	stdout, stderr, exitCode := testRunProtocGenElixir(t, bytes.NewReader(inputBytes))
	require.Equal(t, 0, exitCode, "stderr: %s", stderr.String())
	assert.Equal(t, "", stderr.String())

	var resp pluginpb.CodeGeneratorResponse
	require.NoError(t, proto.Unmarshal(stdout.Bytes(), &resp))
	return &resp
}

func testRunProtocGenElixir(t *testing.T, stdin io.Reader, args ...string) (stdout, stderr *bytes.Buffer, exitCode int) {
	t.Helper()

	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	args = append([]string{"run", "."}, args...)

	cmd := exec.Command("go", args...)
	cmd.Env = os.Environ()
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	_ = cmd.Run() // Don't use require.NoError since we want to capture exit codes
	exitCode = cmd.ProcessState.ExitCode()
	return stdout, stderr, exitCode
}
