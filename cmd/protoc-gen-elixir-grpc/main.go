// protoc-gen-elixir-grpc is a plugin for the Protobuf compiler that generates
// Elixir gRPC server modules with defdelegate patterns. To use it, build this program and make
// it available on your PATH as protoc-gen-elixir-grpc.
//
// The 'elixir-grpc' suffix becomes part of the arguments for the Protobuf
// compiler. To generate server modules using protoc:
//
//	protoc --elixir_out=lib --elixir-grpc_out=lib path/to/file.proto
//
// With [buf], your buf.gen.yaml will look like this:
//
//	version: v2
//	plugins:
//	  - local: protoc-gen-elixir
//	    out: lib
//	  - local: protoc-gen-elixir-grpc
//	    out: lib
//
// If you use buf's top-level clean: true, set out to a generated-only
// directory. Buf deletes each plugin out directory before invoking the plugin.
// Do not point out at a directory that also contains hand-written handlers.
//
// This generates server module definitions for the Protobuf services
// defined by file.proto. If file.proto defines the Greeter service, the
// invocations above will write output to:
//
//	lib/greeter.pb.ex
//	lib/greeter.server.pb.ex
//
// [buf]: https://buf.build
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

var (
	// These variables are set by ldflags during build time
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

const (
	filenameSuffix          = ".ex"
	serverSuffix            = "Server"
	defaultPackagePrefix    = ""
	packagePrefixFlag       = "package_prefix"
	handlerModulePrefixFlag = "handler_module_prefix"
	serverModulePrefixFlag  = "server_module_prefix"
	httpTranscodeFlag       = "http_transcode"
	codecsFlag              = "codecs"
	compressorsFlag         = "compressors"

	usage = "\n\nFlags:\n  -h, --help\tPrint this help and exit.\n      --version\tPrint the version and exit.\n      --handler_module_prefix\tCustom Elixir module prefix for handler modules instead of protobuf package.\n      --server_module_prefix\tCustom Elixir module prefix for server modules instead of protobuf package.\n      --http_transcode\tEnable HTTP transcoding support (adds http_transcode: true to use GRPC.Server).\n      --codecs\tComma-separated list of codec modules (e.g., 'GRPC.Codec.Proto,GRPC.Codec.WebText,GRPC.Codec.JSON').\n      --compressors\tComma-separated list of compressor modules (e.g., 'GRPC.Compressor.Gzip')."
)

// GenerateOptions contains configuration options for generating Elixir gRPC files
type GenerateOptions struct {
	PackagePrefix       string
	HandlerModulePrefix string
	ServerModulePrefix  string
	HTTPTranscode       bool
	Codecs              []string
	Compressors         []string
}

func parsePluginParameters(paramStr string, flagSet *flag.FlagSet) error {
	if paramStr == "" {
		return nil
	}

	// Parse key=value pairs, handling values that may contain commas
	params := parseKeyValuePairs(paramStr)
	for key, value := range params {
		if err := flagSet.Set(key, value); err != nil {
			return err
		}
	}

	return nil
}

// parseKeyValuePairs parses a comma-separated list of key=value pairs.
// It handles the special case where a value may contain commas (e.g., codecs=A,B,C).
// The parser works by splitting on commas, then checking if each segment is a valid key=value pair.
// If not, it's assumed to be a continuation of the previous value.
func parseKeyValuePairs(paramStr string) map[string]string {
	result := make(map[string]string)

	segments := strings.Split(paramStr, ",")
	var currentKey string
	var currentValue strings.Builder

	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		// Check if this segment contains an '=' sign
		if idx := strings.Index(segment, "="); idx > 0 {
			// This is a new key=value pair
			// Save the previous key-value if exists
			if currentKey != "" {
				result[currentKey] = currentValue.String()
			}

			// Start new key-value
			currentKey = strings.TrimSpace(segment[:idx])
			valueStart := strings.TrimSpace(segment[idx+1:])
			currentValue.Reset()
			currentValue.WriteString(valueStart)
		} else {
			// This is a continuation of the current value
			if currentKey != "" {
				currentValue.WriteString(",")
				currentValue.WriteString(segment)
			}
		}
	}

	// Save the final key-value pair if it exists
	if currentKey != "" {
		result[currentKey] = currentValue.String()
	}

	return result
}

func parseCodecs(codecsStr string) []string {
	if codecsStr == "" {
		return nil
	}

	codecs := strings.Split(codecsStr, ",")
	var result []string
	for _, codec := range codecs {
		codec = strings.TrimSpace(codec)
		if codec != "" {
			result = append(result, codec)
		}
	}

	return result
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("protoc-gen-elixir-grpc %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}
	if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		if _, err := fmt.Fprintln(os.Stdout, usage); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if len(os.Args) != 1 {
		if _, err := fmt.Fprintln(os.Stderr, usage); err != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}

	var flagSet flag.FlagSet
	packagePrefix := flagSet.String(
		packagePrefixFlag,
		defaultPackagePrefix,
		"Generate files with a package prefix.",
	)
	handlerModulePrefix := flagSet.String(
		handlerModulePrefixFlag,
		"",
		"Custom Elixir module prefix for handler modules instead of protobuf package (e.g., 'MyApp.Handlers').",
	)
	serverModulePrefix := flagSet.String(
		serverModulePrefixFlag,
		"",
		"Custom Elixir module prefix for server modules instead of protobuf package (e.g., 'MyApp.Servers').",
	)
	httpTranscode := flagSet.Bool(
		httpTranscodeFlag,
		false,
		"Enable HTTP transcoding support (adds http_transcode: true to use GRPC.Server).",
	)
	codecs := flagSet.String(
		codecsFlag,
		"",
		"Comma-separated list of codec modules (e.g., 'GRPC.Codec.Proto,GRPC.Codec.WebText,GRPC.Codec.JSON').",
	)
	compressors := flagSet.String(
		compressorsFlag,
		"",
		"Comma-separated list of compressor modules (e.g., 'GRPC.Compressor.Gzip').",
	)

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %v\n", err)
		os.Exit(1)
	}

	var req pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(input, &req); err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal request: %v\n", err)
		os.Exit(1)
	}

	if req.Parameter != nil {
		if err := parsePluginParameters(*req.Parameter, &flagSet); err != nil {
			resp := &pluginpb.CodeGeneratorResponse{
				Error: proto.String(fmt.Sprintf("failed to parse parameters: %v", err)),
			}
			output, marshalErr := proto.Marshal(resp)
			if marshalErr != nil {
				fmt.Fprintf(os.Stderr, "failed to marshal error response: %v\n", marshalErr)
				os.Exit(1)
			}
			if _, writeErr := os.Stdout.Write(output); writeErr != nil {
				fmt.Fprintf(os.Stderr, "failed to write error response: %v\n", writeErr)
				os.Exit(1)
			}
			return
		}
	}

	resp := &pluginpb.CodeGeneratorResponse{
		SupportedFeatures: proto.Uint64(uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)),
	}

	codecsList := parseCodecs(*codecs)
	compressorsList := parseCodecs(*compressors)

	opts := GenerateOptions{
		PackagePrefix:       *packagePrefix,
		HandlerModulePrefix: *handlerModulePrefix,
		ServerModulePrefix:  *serverModulePrefix,
		HTTPTranscode:       *httpTranscode,
		Codecs:              codecsList,
		Compressors:         compressorsList,
	}

	for _, fileName := range req.FileToGenerate {
		var protoFile *descriptorpb.FileDescriptorProto
		for _, file := range req.ProtoFile {
			if file.GetName() == fileName {
				protoFile = file
				break
			}
		}
		if protoFile == nil {
			continue
		}

		if len(protoFile.Service) == 0 {
			continue
		}

		generateElixirFile(resp, protoFile, opts)
	}

	output, err := proto.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal response: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stdout.Write(output); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write output: %v\n", err)
		os.Exit(1)
	}
}

func generateElixirFile(resp *pluginpb.CodeGeneratorResponse, file *descriptorpb.FileDescriptorProto, opts GenerateOptions) {
	if len(file.Service) == 0 {
		return
	}

	fileName := generateFilePath(file, opts)

	var content strings.Builder
	content.WriteString("# Code generated by protoc-gen-elixir-grpc. DO NOT EDIT.\n")
	content.WriteString("#\n")
	content.WriteString("# Source: " + file.GetName() + "\n")
	content.WriteString("\n")

	for _, service := range file.Service {
		generateServiceModule(&content, file, service, opts)
		content.WriteString("\n")
	}

	resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String(fileName),
		Content: proto.String(content.String()),
	})
}

func generateServiceModule(content *strings.Builder, file *descriptorpb.FileDescriptorProto, service *descriptorpb.ServiceDescriptorProto, opts GenerateOptions) {
	serverModuleName := generateServerModuleName(file, service, opts)
	serviceModuleName := generateServiceModuleName(file, service)

	content.WriteString("defmodule " + serverModuleName + " do\n")
	content.WriteString("  use GRPC.Server,\n")
	content.WriteString("    service: " + serviceModuleName)
	if opts.HTTPTranscode {
		content.WriteString(",\n    http_transcode: true")
	}
	if len(opts.Codecs) > 0 {
		content.WriteString(",\n    codecs: [")
		for i, codec := range opts.Codecs {
			if i > 0 {
				content.WriteString(", ")
			}
			content.WriteString(codec)
		}
		content.WriteString("]")
	}
	if len(opts.Compressors) > 0 {
		content.WriteString(",\n    compressors: [")
		for i, compressor := range opts.Compressors {
			if i > 0 {
				content.WriteString(", ")
			}
			content.WriteString(compressor)
		}
		content.WriteString("]")
	}
	content.WriteString("\n\n")

	// Sort methods alphabetically to ensure deterministic output. This prevents
	// spurious diffs when method definitions are reordered in the proto file.
	sort.Slice(service.Method, func(i, j int) bool {
		return service.Method[i].GetName() < service.Method[j].GetName()
	})

	lastMethodIndex := len(service.Method) - 1
	for i, method := range service.Method {
		generateMethodDelegate(content, file, service, method, opts)
		// Add blank line between delegates, but not after the last one
		if i < lastMethodIndex {
			content.WriteString("\n")
		}
	}

	content.WriteString("end")
}

func generateMethodDelegate(content *strings.Builder, file *descriptorpb.FileDescriptorProto, service *descriptorpb.ServiceDescriptorProto, method *descriptorpb.MethodDescriptorProto, opts GenerateOptions) {
	methodName := toSnakeCase(method.GetName())
	handlerModuleName := generateHandlerModuleName(file, service, method, opts)

	isStreamingClient := method.GetClientStreaming()
	isStreamingServer := method.GetServerStreaming()

	var signature string
	switch {
	case isStreamingClient && isStreamingServer:
		signature = fmt.Sprintf("%s(request_stream, response_stream)", methodName)
	case isStreamingClient && !isStreamingServer:
		signature = fmt.Sprintf("%s(request_stream, stream)", methodName)
	case !isStreamingClient && isStreamingServer:
		signature = fmt.Sprintf("%s(request, response_stream)", methodName)
	default:
		signature = fmt.Sprintf("%s(request, stream)", methodName)
	}

	content.WriteString("  defdelegate " + signature + ",\n")
	content.WriteString("    to: " + handlerModuleName + ",\n")
	content.WriteString("    as: :handle_message\n")
}

func packageToPascalParts(pkg string) []string {
	if pkg == "" {
		return []string{}
	}
	parts := strings.Split(pkg, ".")
	result := make([]string, len(parts))
	for i, part := range parts {
		result[i] = toPascalCase(part)
	}
	return result
}

func generateServiceModuleName(file *descriptorpb.FileDescriptorProto, service *descriptorpb.ServiceDescriptorProto) string {
	serviceName := service.GetName()
	pkg := file.GetPackage()

	if pkg == "" {
		return toPascalCase(serviceName) + ".Service"
	}

	parts := packageToPascalParts(pkg)
	parts = append(parts, toPascalCase(serviceName), "Service")

	return strings.Join(parts, ".")
}

func generateServerModuleName(file *descriptorpb.FileDescriptorProto, service *descriptorpb.ServiceDescriptorProto, opts GenerateOptions) string {
	serviceName := service.GetName()
	pkg := file.GetPackage()
	packageParts := packageToPascalParts(pkg)

	if opts.ServerModulePrefix != "" {
		if pkg == "" {
			return fmt.Sprintf("%s.%s.%s", opts.ServerModulePrefix, toPascalCase(serviceName), serverSuffix)
		}

		return fmt.Sprintf("%s.%s.%s.%s", opts.ServerModulePrefix, strings.Join(packageParts, "."), toPascalCase(serviceName), serverSuffix)
	}

	if pkg == "" {
		return toPascalCase(serviceName) + "." + serverSuffix
	}

	packageParts = append(packageParts, toPascalCase(serviceName), serverSuffix)
	return strings.Join(packageParts, ".")
}

func generateHandlerModuleName(file *descriptorpb.FileDescriptorProto, service *descriptorpb.ServiceDescriptorProto, method *descriptorpb.MethodDescriptorProto, opts GenerateOptions) string {
	serviceName := service.GetName()
	methodName := method.GetName()
	pkg := file.GetPackage()
	packageParts := packageToPascalParts(pkg)

	if opts.HandlerModulePrefix != "" {
		if pkg == "" {
			return fmt.Sprintf("%s.%s.Server.%sHandler", opts.HandlerModulePrefix, toPascalCase(serviceName), toPascalCase(methodName))
		}

		return fmt.Sprintf("%s.%s.%s.Server.%sHandler", opts.HandlerModulePrefix, strings.Join(packageParts, "."), toPascalCase(serviceName), toPascalCase(methodName))
	}

	if pkg == "" {
		return fmt.Sprintf("%s.Server.%sHandler", toPascalCase(serviceName), toPascalCase(methodName))
	}

	packageParts = append(packageParts, toPascalCase(serviceName), "Server", toPascalCase(methodName)+"Handler")

	return strings.Join(packageParts, ".")
}

func generateFilePath(file *descriptorpb.FileDescriptorProto, opts GenerateOptions) string {
	pkg := file.GetPackage()
	fileName := file.GetName()

	var pathParts []string

	if opts.PackagePrefix != "" {
		pathParts = append(pathParts, opts.PackagePrefix)
	}

	if pkg != "" {
		parts := strings.Split(pkg, ".")
		for _, part := range parts {
			pathParts = append(pathParts, strings.ToLower(part))
		}
	}

	baseFileName := filepath.Base(fileName)
	if idx := strings.LastIndex(baseFileName, "."); idx > 0 {
		baseFileName = baseFileName[:idx]
	}

	pathParts = append(pathParts, baseFileName+".server.pb"+filenameSuffix)

	return strings.Join(pathParts, "/")
}

func toSnakeCase(s string) string {
	re := regexp.MustCompile("([A-Z]+)([A-Z][a-z])")
	s = re.ReplaceAllString(s, "${1}_${2}")

	re = regexp.MustCompile("([a-z])([A-Z])")
	s = re.ReplaceAllString(s, "${1}_${2}")

	return strings.ToLower(s)
}

func toPascalCase(s string) string {
	if len(s) == 0 {
		return s
	}

	// Split by underscores and capitalize each part
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}

	return strings.Join(parts, "")
}
