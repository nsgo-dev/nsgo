package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadYAML is load of a directory holding body as its nsgo.yaml.
func loadYAML(t *testing.T, body string) (*project, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return load(dir)
}

func TestUnknownKeyIsRefused(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "nested",
			body: "name: App\nandroid:\n  permisions: [RECORD_AUDIO]\n",
			want: "nsgo.yaml:3: android has no key permisions; the closest is permissions",
		},
		{
			name: "top level",
			body: "name: App\nlicences:\n  allow: [MIT]\n",
			want: "nsgo.yaml:2: the top level has no key licences; the closest is licenses",
		},
		{
			name: "two levels down",
			body: "name: App\nwindows:\n  store:\n    identiy: x\n",
			want: "nsgo.yaml:4: windows.store has no key identiy; the closest is identity",
		},
		{
			name: "in a sequence",
			body: "name: App\ncommands:\n  - name: tool\n    package: ./cmd/tool\n    unknown_thing: 1\n",
			want: "nsgo.yaml:5: commands[] has no key unknown_thing",
		},
		{
			name: "no near key",
			body: "name: App\nzzz: 1\n",
			want: "nsgo.yaml:2: the top level has no key zzz",
		},
		{
			name: "two unknown keys",
			body: "name: App\nlicences: {allow: [MIT]}\nandrodi: {permissions: [X]}\n",
			want: "nsgo.yaml:2: the top level has no key licences; the closest is licenses; 3: the top level has no key androdi; the closest is android",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadYAML(t, tt.body)
			if err == nil {
				t.Fatalf("a misspelt key loaded")
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("error = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFreeFormKeysStayFree checks that the parts nsgo.yaml documents as
// free-form keep taking keys of their own.
func TestFreeFormKeysStayFree(t *testing.T) {
	body := `name: App
window: {anything: 1}
tailwind:
  theme:
    extend:
      colors: {brand: "#123456"}
capabilities:
  anything: [open_url]
status_bar:
  foreground: dark
apple:
  info:
    NSMicrophoneUsageDescription: Anything
  privacy:
    accessed_apis:
      NSPrivacyAccessedAPICategoryFileTimestamp: [C617.1]
    collected_data:
      - {NSPrivacyCollectedDataType: Anything, extra: y}
macos:
  entitlements:
    com.example.anything: true
miniprogram:
  appid:
    wechat: wx123
dev:
  fps: {anything: 1}
build:
  fps: {anything: 1}
`
	if err := decodeProject([]byte(body), &project{}); err != nil {
		t.Errorf("a free-form key was refused: %v", err)
	}
}

// TestRepositoryNSGOYAMLsDecode checks that every nsgo.yaml in the
// repository still loads under strict decoding.
func TestRepositoryNSGOYAMLsDecode(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("no module at %s: %v", root, err)
	}
	files := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == ".git" || name == "node_modules" || name == "dist" {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != "nsgo.yaml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := decodeProject(data, &project{}); err != nil {
			t.Errorf("%s: %v", strings.TrimPrefix(path, root+string(filepath.Separator)), err)
		}
		files++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatal("no nsgo.yaml found")
	}
}

// TestInitTemplateDecodes checks that nsgo init's template loads.
func TestInitTemplateDecodes(t *testing.T) {
	var buf bytes.Buffer
	if err := initYAML.Execute(&buf, initData{"My App", "dev.example.myapp", "example.com/myapp", "my-app"}); err != nil {
		t.Fatal(err)
	}
	if err := decodeProject(buf.Bytes(), &project{}); err != nil {
		t.Fatalf("nsgo init's template: %v", err)
	}
}
