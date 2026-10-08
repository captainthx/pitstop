package agentassets_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"pitstop/internal/agentassets"
	"pitstop/internal/config"
)

func TestBundleCompleteAndValid(t *testing.T) {
	expected := []string{
		"base", "node-npm", "node-pnpm", "node-yarn-classic", "node-yarn-berry",
		"go", "python-pip", "python-uv", "java-maven", "java-gradle", "dotnet",
		"rust", "php-composer", "ruby-bundler",
	}
	if got := agentassets.PresetNames(); !reflect.DeepEqual(got, expected) {
		t.Fatalf("preset names = %v, want %v", got, expected)
	}

	files := agentassets.Files()
	if !sort.StringsAreSorted(files) {
		t.Fatal("bundle files are not sorted")
	}
	seen := make(map[string]bool, len(files))
	for _, path := range files {
		if seen[path] || !validBundlePath(path) {
			t.Fatalf("invalid or duplicate bundle path %q", path)
		}
		seen[path] = true
		data, err := agentassets.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.HasSuffix(path, ".json") && path != "manifest.json" {
			if _, err := config.Parse(data); err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
		}
	}

	var manifest struct {
		Owner         string `json:"owner"`
		BundleVersion string `json:"bundleVersion"`
		Files         []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	manifestBytes, err := agentassets.ReadFile("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if manifest.Owner != "pitstop" || manifest.BundleVersion != agentassets.BundleVersion {
		t.Fatalf("manifest identity = %q/%q", manifest.Owner, manifest.BundleVersion)
	}
	wantManifest := append([]string(nil), files...)
	wantManifest = removePath(wantManifest, "manifest.json")
	gotManifest := make([]string, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		gotManifest = append(gotManifest, file.Path)
		data, err := agentassets.ReadFile(file.Path)
		if err != nil {
			t.Fatalf("manifest file %s: %v", file.Path, err)
		}
		hash := sha256.Sum256(data)
		if file.SHA256 != hex.EncodeToString(hash[:]) {
			t.Fatalf("manifest hash mismatch for %s", file.Path)
		}
	}
	if !reflect.DeepEqual(gotManifest, wantManifest) {
		t.Fatalf("manifest files = %v, want %v", gotManifest, wantManifest)
	}

	for _, path := range []string{"SKILL.md", "references/config-reference.md", "references/workflow-mapping.md", "references/examples.md"} {
		data, err := agentassets.ReadFile(path)
		if err != nil {
			t.Fatalf("read linked document %s: %v", path, err)
		}
		for _, link := range markdownLinks(string(data)) {
			if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "#") {
				continue
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), link))
			if !validBundlePath(resolved) {
				t.Fatalf("%s links outside bundle: %s", path, link)
			}
			if _, err := agentassets.ReadFile(resolved); err != nil {
				t.Fatalf("%s links to %s: %v", path, link, err)
			}
		}
	}

	root := filepath.Join("..", "..", "examples")
	for _, pair := range []struct{ root, preset string }{{"docker-react.json", "node-npm"}, {"docker-go.json", "go"}} {
		rootBytes, err := os.ReadFile(filepath.Join(root, pair.root))
		if err != nil {
			t.Fatal(err)
		}
		rootConfig, err := config.Parse(rootBytes)
		if err != nil {
			t.Fatalf("parse root example %s: %v", pair.root, err)
		}
		templateBytes, err := agentassets.Preset(pair.preset)
		if err != nil {
			t.Fatal(err)
		}
		templateConfig, err := config.Parse(templateBytes)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(rootConfig, templateConfig) {
			t.Fatalf("root example %s differs from preset %s", pair.root, pair.preset)
		}
	}

	for _, name := range expected {
		if _, err := agentassets.Preset(name); err != nil {
			t.Fatalf("preset %s: %v", name, err)
		}
	}

	for _, path := range []string{
		"assets/source/scripts/check.sh",
		"assets/source/local-action/action.yml",
		"assets/source/manifests/python-pyproject.toml",
		"assets/images/node-go-tools/Dockerfile",
		"assets/workflows/multi-test.yml",
		"assets/workflows/multi-release.yml",
		"assets/workflows/python-pytest.yml",
		"assets/workflows/java-maven.yml",
		"assets/workflows/rust-cargo.yml",
		"assets/mappings/python-pytest.json",
		"assets/mappings/java-maven.json",
		"assets/mappings/rust-cargo.json",
	} {
		if _, err := agentassets.ReadFile(path); err != nil {
			t.Fatalf("required source fixture %s: %v", path, err)
		}
	}
	assertAssetContains(t, "assets/source/scripts/check.sh", "npm run check")
	assertAssetContains(t, "assets/source/local-action/action.yml", "npm run check")
	assertAssetContains(t, "assets/source/manifests/python-pyproject.toml", "pytest")
}

func assertAssetContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := agentassets.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s does not contain %q", path, want)
	}
}

func validBundlePath(path string) bool {
	return path != "" && !strings.HasPrefix(path, "/") && !strings.Contains(path, "..")
}

func removePath(paths []string, target string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != target {
			result = append(result, path)
		}
	}
	return result
}

func markdownLinks(s string) []string {
	var links []string
	for {
		start := strings.Index(s, "](")
		if start < 0 {
			return links
		}
		s = s[start+2:]
		end := strings.IndexByte(s, ')')
		if end < 0 {
			return links
		}
		links = append(links, s[:end])
		s = s[end+1:]
	}
}
