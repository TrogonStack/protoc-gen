package main

import (
	"fmt"
	"slices"
	"strings"
)

// Plugin identifies a sub-plugin requested via plugins=a+b+c. It is a string
// underneath - the escript never validates sub-plugin names, so an unknown
// Plugin value is accepted and simply never matches HasPlugin - but named
// constants like PluginGRPC give known sub-plugins a proper, typo-resistant
// identifier instead of a bare string.
type Plugin string

// PluginGRPC is the only sub-plugin the escript currently acts on: it enables
// service module emission.
const PluginGRPC Plugin = "grpc"

// Params holds the plugin parameters accepted via --elixir_opt=key=value,...,
// mirroring the fields Protobuf.Protoc.Context accumulates from
// Protobuf.Protoc.CLI.parse_params/2 at the pinned escript HEAD.
type Params struct {
	Plugins          []Plugin
	GenDescriptors   bool
	PackagePrefix    *string
	TransformModule  *string
	OneFilePerModule bool
	IncludeDocs      bool
	GenProtoSource   bool
}

// HasPlugin reports whether the given sub-plugin (e.g. PluginGRPC) was
// requested via plugins=....
func (p *Params) HasPlugin(name Plugin) bool {
	return slices.Contains(p.Plugins, name)
}

// ParseParams parses the comma-separated key=value parameter string exactly
// as Protobuf.Protoc.CLI.parse_params/2 does: unknown keys are silently
// ignored, and gen_descriptors, one_file_per_module, and include_docs only
// accept the literal value "true" (any other value is an error).
//
// Unlike package_prefix, an empty transform_module value is NOT an error in
// the v0.16.0 escript (Module.concat([""]) never raises) even though it
// looks like it should mirror package_prefix's empty check. Don't add that
// check without re-verifying against the pinned escript version.
func ParseParams(paramStr string) (*Params, error) {
	params := &Params{}

	for segment := range strings.SplitSeq(paramStr, ",") {
		switch {
		case strings.HasPrefix(segment, "plugins="):
			value := strings.TrimPrefix(segment, "plugins=")
			params.Plugins = nil
			for name := range strings.SplitSeq(value, "+") {
				params.Plugins = append(params.Plugins, Plugin(name))
			}

		case strings.HasPrefix(segment, "gen_descriptors="):
			value := strings.TrimPrefix(segment, "gen_descriptors=")
			if value != "true" {
				return nil, fmt.Errorf("invalid value for gen_descriptors option, expected \"true\", got: %q", value)
			}
			params.GenDescriptors = true

		case strings.HasPrefix(segment, "package_prefix="):
			value := strings.TrimPrefix(segment, "package_prefix=")
			if value == "" {
				return nil, fmt.Errorf("package_prefix can't be empty")
			}
			params.PackagePrefix = &value

		case strings.HasPrefix(segment, "transform_module="):
			value := strings.TrimPrefix(segment, "transform_module=")
			params.TransformModule = &value

		case strings.HasPrefix(segment, "one_file_per_module="):
			value := strings.TrimPrefix(segment, "one_file_per_module=")
			if value != "true" {
				return nil, fmt.Errorf("invalid value for one_file_per_module option, expected \"true\", got: %q", value)
			}
			params.OneFilePerModule = true

		case strings.HasPrefix(segment, "include_docs="):
			value := strings.TrimPrefix(segment, "include_docs=")
			if value != "true" {
				return nil, fmt.Errorf("invalid value for include_docs option, expected \"true\", got: %q", value)
			}
			params.IncludeDocs = true

		case strings.HasPrefix(segment, "gen_proto_source="):
			value := strings.TrimPrefix(segment, "gen_proto_source=")
			if value != "true" {
				return nil, fmt.Errorf("invalid value for gen_proto_source option, expected \"true\", got: %q", value)
			}
			params.GenProtoSource = true
		}
	}

	return params, nil
}
