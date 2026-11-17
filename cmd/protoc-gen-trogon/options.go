package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type language string
type feature string

const (
	rustLanguage language = "rust"
	uuidFeature  feature  = "uuid"
)

type configuration struct {
	language    language
	features    []feature
	runtimePath string
}

func parseConfiguration(parameter string) (configuration, error) {
	values := make(map[string]string)
	if parameter != "" {
		for _, option := range strings.Split(parameter, ",") {
			if option == "" {
				return configuration{}, fmt.Errorf("empty option")
			}
			key, value, ok := strings.Cut(option, "=")
			if !ok || value == "" {
				return configuration{}, fmt.Errorf("option %q requires a nonempty value", key)
			}
			switch key {
			case "language", "features", "rs_runtime_path":
			default:
				return configuration{}, fmt.Errorf("unknown option %q", key)
			}
			if _, exists := values[key]; exists {
				return configuration{}, fmt.Errorf("duplicate option %q", key)
			}
			values[key] = value
		}
	}
	if values["language"] == "" {
		return configuration{}, fmt.Errorf("language is required; supported language: rust")
	}
	selectedLanguage := language(values["language"])
	if selectedLanguage != rustLanguage {
		return configuration{}, fmt.Errorf("unsupported language %q; supported language: rust", selectedLanguage)
	}
	if values["features"] == "" {
		return configuration{}, fmt.Errorf("features is required; supported feature: uuid")
	}
	var selectedFeatures []feature
	seen := make(map[feature]bool)
	for _, value := range strings.Split(values["features"], "+") {
		selected := feature(value)
		if selected != uuidFeature {
			return configuration{}, fmt.Errorf("unsupported feature %q for language %q; supported feature: uuid", selected, selectedLanguage)
		}
		if seen[selected] {
			return configuration{}, fmt.Errorf("duplicate feature %q", selected)
		}
		seen[selected] = true
		selectedFeatures = append(selectedFeatures, selected)
	}
	runtimePath := defaultRuntimePath
	if value, present := values["rs_runtime_path"]; present {
		runtimePath = value
	}
	if !validRuntimePath(runtimePath) {
		return configuration{}, fmt.Errorf("rs_runtime_path must be an absolute Rust path of identifiers")
	}
	return configuration{language: selectedLanguage, features: selectedFeatures, runtimePath: runtimePath}, nil
}

func validRuntimePath(path string) bool {
	if !strings.HasPrefix(path, "::") {
		return false
	}
	for _, segment := range strings.Split(strings.TrimPrefix(path, "::"), "::") {
		if !validField(segment) || rustIdent(segment) != segment {
			return false
		}
	}
	return true
}

func (config configuration) generateFile(file protoreflect.FileDescriptor, files *protoregistry.Files) (string, error) {
	if config.language != rustLanguage {
		return "", fmt.Errorf("unsupported language %q", config.language)
	}
	var output strings.Builder
	for _, selected := range config.features {
		switch selected {
		case uuidFeature:
			source, err := generateFile(file, files, config.runtimePath)
			if err != nil {
				return "", err
			}
			output.WriteString(source)
		default:
			return "", fmt.Errorf("unsupported feature %q", selected)
		}
	}
	return output.String(), nil
}
