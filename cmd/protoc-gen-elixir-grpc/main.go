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
	httpTranscodeFlag       = "http_transcode"
	codecsFlag              = "codecs"
	compressorsFlag         = "compressors"

	usage = "\n\nFlags:\n  -h, --help\tPrint this help and exit.\n      --version\tPrint the version and exit.\n      --handler_module_prefix\tCustom Elixir module prefix for handler modules instead of protobuf package.\n      --http_transcode\tEnable HTTP transcoding support (adds http_transcode: true to use GRPC.Server).\n      --codecs\tComma-separated list of codec modules (e.g., 'GRPC.Codec.Proto,GRPC.Codec.WebText,GRPC.Codec.JSON').\n      --compressors\tComma-separated list of compressor modules (e.g., 'GRPC.Compressor.Gzip')."
)

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

		generateElixirFile(resp, protoFile, *packagePrefix, *handlerModulePrefix, *httpTranscode, codecsList, compressorsList)
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

func generateElixirFile(resp *pluginpb.CodeGeneratorResponse, file *descriptorpb.FileDescriptorProto, packagePrefix, handlerModulePrefix string, httpTranscode bool, codecs []string, compressors []string) {
	if len(file.Service) == 0 {
		return
	}

	fileName := generateFilePath(file, packagePrefix)

	var content strings.Builder
	content.WriteString("# Code generated by protoc-gen-elixir-grpc. DO NOT EDIT.\n")
	content.WriteString("#\n")
	content.WriteString("# Source: " + file.GetName() + "\n")
	content.WriteString("\n")

	for _, service := range file.Service {
		generateServiceModule(&content, file, service, handlerModulePrefix, httpTranscode, codecs, compressors)
		content.WriteString("\n")
	}

	resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String(fileName),
		Content: proto.String(content.String()),
	})
}

func generateServiceModule(content *strings.Builder, file *descriptorpb.FileDescriptorProto, service *descriptorpb.ServiceDescriptorProto, handlerModulePrefix string, httpTranscode bool, codecs []string, compressors []string) {
	serverModuleName := generateServerModuleName(file, service)
	serviceModuleName := generateServiceModuleName(file, service)

	content.WriteString("defmodule " + serverModuleName + " do\n")
	content.WriteString("  use GRPC.Server,\n")
	content.WriteString("    service: " + serviceModuleName)
	if httpTranscode {
		content.WriteString(",\n    http_transcode: true")
	}
	if len(codecs) > 0 {
		content.WriteString(",\n    codecs: [")
		for i, codec := range codecs {
			if i > 0 {
				content.WriteString(", ")
			}
			content.WriteString(codec)
		}
		content.WriteString("]")
	}
	if len(compressors) > 0 {
		content.WriteString(",\n    compressors: [")
		for i, compressor := range compressors {
			if i > 0 {
				content.WriteString(", ")
			}
			content.WriteString(compressor)
		}
		content.WriteString("]")
	}
	content.WriteString("\n\n")

	for _, method := range service.Method {
		generateMethodDelegate(content, file, service, method, handlerModulePrefix)
	}

	content.WriteString("end")
}

func generateMethodDelegate(content *strings.Builder, file *descriptorpb.FileDescriptorProto, service *descriptorpb.ServiceDescriptorProto, method *descriptorpb.MethodDescriptorProto, handlerModulePrefix string) {
	methodName := toSnakeCase(method.GetName())
	handlerModuleName := generateHandlerModuleName(file, service, method, handlerModulePrefix)

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

	content.WriteString("  defdelegate " + signature + ", to: " + handlerModuleName + ", as: :handle_message\n")
}

func generateServiceModuleName(file *descriptorpb.FileDescriptorProto, service *descriptorpb.ServiceDescriptorProto) string {
	serviceName := service.GetName()
	pkg := file.GetPackage()

	if pkg == "" {
		return toPascalCase(serviceName) + ".Service"
	}

	parts := strings.Split(pkg, ".")
	var elixirParts []string
	for _, part := range parts {
		elixirParts = append(elixirParts, toPascalCase(part))
	}

	elixirParts = append(elixirParts, toPascalCase(serviceName), "Service")

	return strings.Join(elixirParts, ".")
}

func generateServerModuleName(file *descriptorpb.FileDescriptorProto, service *descriptorpb.ServiceDescriptorProto) string {
	serviceName := service.GetName()
	pkg := file.GetPackage()

	if pkg == "" {
		return toPascalCase(serviceName) + "." + serverSuffix
	}

	parts := strings.Split(pkg, ".")
	var elixirParts []string
	for _, part := range parts {
		elixirParts = append(elixirParts, toPascalCase(part))
	}

	elixirParts = append(elixirParts, toPascalCase(serviceName), serverSuffix)

	return strings.Join(elixirParts, ".")
}

func generateHandlerModuleName(file *descriptorpb.FileDescriptorProto, service *descriptorpb.ServiceDescriptorProto, method *descriptorpb.MethodDescriptorProto, handlerModulePrefix string) string {
	serviceName := service.GetName()
	methodName := method.GetName()
	pkg := file.GetPackage()

	if handlerModulePrefix != "" {
		if pkg == "" {
			return fmt.Sprintf("%s.%s.Server.%sHandler", handlerModulePrefix, toPascalCase(serviceName), toPascalCase(methodName))
		}

		parts := strings.Split(pkg, ".")
		var packageParts []string
		for _, part := range parts {
			packageParts = append(packageParts, toPascalCase(part))
		}

		return fmt.Sprintf("%s.%s.%s.Server.%sHandler", handlerModulePrefix, strings.Join(packageParts, "."), toPascalCase(serviceName), toPascalCase(methodName))
	}

	if pkg == "" {
		return fmt.Sprintf("%s.Server.%sHandler", toPascalCase(serviceName), toPascalCase(methodName))
	}

	parts := strings.Split(pkg, ".")
	var elixirParts []string
	for _, part := range parts {
		elixirParts = append(elixirParts, toPascalCase(part))
	}

	elixirParts = append(elixirParts, toPascalCase(serviceName), "Server", toPascalCase(methodName)+"Handler")

	return strings.Join(elixirParts, ".")
}

func generateFilePath(file *descriptorpb.FileDescriptorProto, packagePrefix string) string {
	pkg := file.GetPackage()
	fileName := file.GetName()

	var pathParts []string

	if packagePrefix != "" {
		pathParts = append(pathParts, packagePrefix)
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

	return strings.Join(pathParts, "/") + "/" + baseFileName + ".server.pb" + filenameSuffix
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
	return strings.ToUpper(s[:1]) + s[1:]
}
