// Package agentassets provides the canonical embedded config-agent bundle.
package agentassets

import (
	"embed"
	"errors"
	"io/fs"
)

const BundleVersion = "1"

var ErrNotFound = errors.New("agent asset not found")

//go:embed bundle
var bundle embed.FS

var presetNames = []string{
	"base", "node-npm", "node-pnpm", "node-yarn-classic", "node-yarn-berry",
	"go", "python-pip", "python-uv", "java-maven", "java-gradle", "dotnet",
	"rust", "php-composer", "ruby-bundler",
}

var bundleFiles = []string{
	"SKILL.md",
	"assets/examples/base.json",
	"assets/examples/custom-image.json",
	"assets/examples/dotnet.json",
	"assets/examples/go.json",
	"assets/examples/java-gradle.json",
	"assets/examples/java-maven.json",
	"assets/examples/mixed-language.json",
	"assets/examples/node-npm.json",
	"assets/examples/node-pnpm.json",
	"assets/examples/node-yarn-berry.json",
	"assets/examples/node-yarn-classic.json",
	"assets/examples/php-composer.json",
	"assets/examples/python-pip.json",
	"assets/examples/python-uv.json",
	"assets/examples/ruby-bundler.json",
	"assets/examples/rust.json",
	"assets/mappings/local-composite.json",
	"assets/mappings/monorepo.json",
	"assets/mappings/multi-workflow.json",
	"assets/mappings/no-workflow.json",
	"assets/mappings/nonstandard-names.json",
	"assets/mappings/partial-features.json",
	"assets/mappings/shared-script.json",
	"assets/workflows/local-composite.yml",
	"assets/workflows/monorepo.yml",
	"assets/workflows/multi-workflow.yml",
	"assets/workflows/nonstandard-names.yml",
	"assets/workflows/partial-features.yml",
	"assets/workflows/shared-script.yml",
	"manifest.json",
	"references/config-reference.md",
	"references/examples.md",
	"references/workflow-mapping.md",
}

func Preset(name string) ([]byte, error) {
	for _, candidate := range presetNames {
		if candidate == name {
			return ReadFile("assets/examples/" + name + ".json")
		}
	}
	return nil, ErrNotFound
}

func PresetNames() []string { return append([]string(nil), presetNames...) }

func Files() []string { return append([]string(nil), bundleFiles...) }

func ReadFile(path string) ([]byte, error) {
	if !fs.ValidPath(path) || !allowedPath(path) {
		return nil, ErrNotFound
	}
	data, err := fs.ReadFile(bundle, "bundle/"+path)
	if err != nil {
		return nil, ErrNotFound
	}
	return data, nil
}

func allowedPath(path string) bool {
	for _, candidate := range bundleFiles {
		if candidate == path {
			return true
		}
	}
	return false
}
