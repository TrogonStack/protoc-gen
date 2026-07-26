package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseParams(t *testing.T) {
	t.Parallel()

	t.Run("empty parameter string", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("")
		require.NoError(t, err)
		assert.Equal(t, &Params{}, params)
	})

	t.Run("unknown keys are silently ignored", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("foo=bar,baz")
		require.NoError(t, err)
		assert.Equal(t, &Params{}, params)
	})

	t.Run("plugins", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("plugins=grpc")
		require.NoError(t, err)
		assert.Equal(t, []Plugin{PluginGRPC}, params.Plugins)
		assert.True(t, params.HasPlugin(PluginGRPC))
		assert.False(t, params.HasPlugin("other"))
	})

	t.Run("plugins joined by +", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("plugins=grpc+other")
		require.NoError(t, err)
		assert.Equal(t, []Plugin{PluginGRPC, "other"}, params.Plugins)
	})

	t.Run("gen_descriptors requires literal true", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("gen_descriptors=true")
		require.NoError(t, err)
		assert.True(t, params.GenDescriptors)

		_, err = ParseParams("gen_descriptors=false")
		assert.EqualError(t, err, `invalid value for gen_descriptors option, expected "true", got: "false"`)
	})

	t.Run("one_file_per_module requires literal true", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("one_file_per_module=true")
		require.NoError(t, err)
		assert.True(t, params.OneFilePerModule)

		_, err = ParseParams("one_file_per_module=1")
		assert.EqualError(t, err, `invalid value for one_file_per_module option, expected "true", got: "1"`)
	})

	t.Run("include_docs requires literal true", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("include_docs=true")
		require.NoError(t, err)
		assert.True(t, params.IncludeDocs)

		_, err = ParseParams("include_docs=yes")
		assert.EqualError(t, err, `invalid value for include_docs option, expected "true", got: "yes"`)
	})

	t.Run("package_prefix", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("package_prefix=MyApp")
		require.NoError(t, err)
		require.NotNil(t, params.PackagePrefix)
		assert.Equal(t, "MyApp", *params.PackagePrefix)
	})

	t.Run("package_prefix cannot be empty", func(t *testing.T) {
		t.Parallel()

		_, err := ParseParams("package_prefix=")
		assert.EqualError(t, err, "package_prefix can't be empty")
	})

	t.Run("transform_module", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("transform_module=My.App.Transform")
		require.NoError(t, err)
		require.NotNil(t, params.TransformModule)
		assert.Equal(t, "My.App.Transform", *params.TransformModule)
	})

	// Unlike package_prefix, an empty transform_module is NOT an error in the
	// pinned v0.16.0 escript: Module.concat([""]) never raises. Verified
	// directly against elixir-protobuf/protobuf@6379e87 (v0.16.0 release
	// commit) lib/protobuf/protoc/cli.ex.
	t.Run("transform_module empty value does not error", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("transform_module=")
		require.NoError(t, err)
		require.NotNil(t, params.TransformModule)
		assert.Equal(t, "", *params.TransformModule)
	})

	t.Run("gen_proto_source requires literal true", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("gen_proto_source=true")
		require.NoError(t, err)
		assert.True(t, params.GenProtoSource)

		_, err = ParseParams("gen_proto_source=false")
		assert.EqualError(t, err, `invalid value for gen_proto_source option, expected "true", got: "false"`)
	})

	t.Run("combined parameters", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("plugins=grpc,gen_descriptors=true,package_prefix=MyApp,include_docs=true")
		require.NoError(t, err)
		assert.Equal(t, []Plugin{PluginGRPC}, params.Plugins)
		assert.True(t, params.GenDescriptors)
		assert.True(t, params.IncludeDocs)
		require.NotNil(t, params.PackagePrefix)
		assert.Equal(t, "MyApp", *params.PackagePrefix)
	})

	t.Run("last value wins when a key repeats", func(t *testing.T) {
		t.Parallel()

		params, err := ParseParams("plugins=grpc,plugins=other")
		require.NoError(t, err)
		assert.Equal(t, []Plugin{"other"}, params.Plugins)
	})
}
