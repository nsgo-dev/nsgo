package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// A module can shadow a standard-library package at its root or beneath it.
func initModuleCollision(ctx context.Context, module string) (string, error) {
	first, _, _ := strings.Cut(module, "/")
	if strings.Contains(first, ".") {
		return "", nil
	}
	args := []string{"list", "-find", "-e", "-f", "{{if .Standard}}{{.ImportPath}}{{end}}", "--", "std"}
	// Listing a literal candidate too detects packages that std omits on this
	// host (such as syscall/js). -e allows an absent candidate.
	if module != "all" && module != "cmd" && module != "std" &&
		fs.ValidPath(module) && !strings.ContainsAny(module, `\:`) && !strings.Contains(module, "...") {
		args = append(args, module)
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	// The caller's module, workspace and build flags must not affect the list.
	cmd.Env = append(os.Environ(), "GO111MODULE=off", "GOENV=off", "GOFLAGS=", "GOTOOLCHAIN=local", "GOWORK=off")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) != 0 {
			return "", fmt.Errorf("init: list Go standard library packages: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("init: list Go standard library packages: %w", err)
	}
	for path := range strings.FieldsSeq(string(out)) {
		if suffix, ok := strings.CutPrefix(path, module); ok && (suffix == "" || suffix[0] == '/') {
			return path, nil
		}
	}
	return "", nil
}

func initModuleTemplate(ctx context.Context, dir, kind string, data initData) error {
	module, version, hasVersion := strings.Cut(kind, "@")
	domain, _, _ := strings.Cut(module, "/")
	if !strings.Contains(domain, ".") || strings.HasPrefix(module, "-") || hasVersion && version == "" {
		return fmt.Errorf("init: no template %q; use %s or a module path with an optional @version", kind, strings.Join(templates, ", "))
	}
	if !hasVersion {
		version = "latest"
	}
	explicitModule := data.Module != ""
	if !explicitModule {
		data.Module = data.Slug
		standard, err := initModuleCollision(ctx, data.Module)
		if err != nil {
			return err
		}
		if standard != "" {
			data.Module = "example.com/" + data.Slug
		}
	}
	moduleDir, err := downloadInitModule(ctx, module+"@"+version)
	if err != nil {
		return err
	}
	src, err := os.OpenRoot(filepath.Join(moduleDir, "template"))
	if err != nil {
		return fmt.Errorf("init: module %s has no readable template directory: %w", module, err)
	}
	defer src.Close()

	files, err := moduleTemplateFiles(src.FS(), data, explicitModule)
	if err != nil {
		return fmt.Errorf("init: template %s: %w", kind, err)
	}
	if err := writeModuleTemplate(dir, src, files); err != nil {
		return err
	}
	detail("next: go tool nsgo dev %s", rel(dir))
	return nil
}

// Use an isolated directory so a caller's module, workspace and replace
// directives cannot influence the download or receive its checksums.
func downloadInitModule(ctx context.Context, query string) (string, error) {
	dir, err := os.MkdirTemp("", "nsgo-init-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	env := append(os.Environ(), "GO111MODULE=on", "GOWORK=off", "GOTOOLCHAIN=local", "GONOPROXY=none")
	goCommand := func(args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir, cmd.Env = dir, env
		return cmd
	}
	proxy, err := goCommand("env", "GOPROXY").Output()
	if err != nil {
		return "", fmt.Errorf("init: read Go module proxy: %w", err)
	}
	env = append(env, "GOPROXY="+initModuleProxy(strings.TrimSpace(string(proxy))))

	out, err := goCommand("mod", "download", "-json", query).Output()
	var result struct{ Dir, Error string }
	decodeErr := json.Unmarshal(out, &result)
	if err != nil {
		if result.Error != "" {
			return "", fmt.Errorf("init: download template %s: %s", query, result.Error)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) != 0 {
			return "", fmt.Errorf("init: download template %s: %s", query, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("init: download template %s: %w", query, err)
	}
	if decodeErr != nil {
		return "", fmt.Errorf("init: read download result for %s: %w", query, decodeErr)
	}
	if result.Dir == "" {
		return "", fmt.Errorf("init: download template %s returned no module directory", query)
	}
	return result.Dir, nil
}

func initModuleProxy(proxies string) string {
	start := 0
	for {
		end := strings.IndexAny(proxies[start:], ",|")
		if end < 0 {
			end = len(proxies) - start
		}
		value := strings.TrimSpace(proxies[start : start+end])
		if value == "direct" {
			return proxies[:start] + "off"
		}
		if start+end == len(proxies) {
			return proxies
		}
		start += end + 1
	}
}

type moduleTemplateFile struct {
	name, source string
	dir          bool
	content      []byte
}

// Read and render every template before opening any destination file.
func moduleTemplateFiles(src fs.FS, data initData, explicitModule bool) ([]moduleTemplateFile, error) {
	var files []moduleTemplateFile
	seen := make(map[string]bool)
	err := fs.WalkDir(src, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || name == "." {
			return err
		}
		file := moduleTemplateFile{source: name, dir: entry.IsDir()}
		target := name
		if !file.dir {
			if !entry.Type().IsRegular() {
				return fmt.Errorf("%s is not a regular file", name)
			}
			var marked bool
			target, marked = strings.CutSuffix(name, ".tmpl")
			if marked || target == "nsgo.yaml" || target == "main.go" {
				var err error
				file.content, err = renderInitTemplate(src, name, data)
				if err != nil {
					return err
				}
				file.source = ""
			}
		}
		if _, exists := seen[target]; target == "." || exists {
			return fmt.Errorf("%s conflicts with another template entry", target)
		}
		file.name, err = filepath.Localize(target)
		if err != nil {
			return fmt.Errorf("invalid output path %q: %w", target, err)
		}
		files = append(files, file)
		seen[target] = !file.dir
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"nsgo.yaml", "main.go"} {
		if !seen[name] {
			return nil, fmt.Errorf("missing %s", name)
		}
	}
	hasModuleFile, hasModuleEntry := seen["go.mod"]
	if hasModuleEntry && !hasModuleFile {
		return nil, fmt.Errorf("go.mod is a directory")
	}
	if explicitModule && !hasModuleEntry {
		var content bytes.Buffer
		if err := initMod.Execute(&content, data); err != nil {
			return nil, err
		}
		files = append(files, moduleTemplateFile{name: "go.mod", content: content.Bytes()})
	}
	return files, nil
}

func renderInitTemplate(src fs.FS, name string, data initData) ([]byte, error) {
	content, err := fs.ReadFile(src, name)
	if err != nil {
		return nil, err
	}
	t, err := template.New(name).Option("missingkey=error").Parse(string(content))
	if err != nil {
		return nil, err
	}
	var rendered bytes.Buffer
	if err := t.Execute(&rendered, data); err != nil {
		return nil, err
	}
	return rendered.Bytes(), nil
}

func writeModuleTemplate(dir string, src *os.Root, files []moduleTemplateFile) (err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dst, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer dst.Close()

	for _, file := range files {
		info, err := dst.Lstat(file.name)
		switch {
		case err == nil && !(file.dir && info.IsDir()):
			return fmt.Errorf("init: %s exists", rel(filepath.Join(dir, file.name)))
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			return err
		}
	}
	var created []string
	defer func() {
		if err != nil {
			for i := len(created) - 1; i >= 0; i-- {
				err = errors.Join(err, dst.Remove(created[i]))
			}
		}
	}()
	for _, file := range files {
		if file.dir {
			if err := dst.Mkdir(file.name, 0o755); err != nil {
				if info, statErr := dst.Lstat(file.name); !errors.Is(err, fs.ErrExist) || statErr != nil || !info.IsDir() {
					return err
				}
				continue
			}
		} else {
			if err := copyModuleTemplateFile(src, dst, file); err != nil {
				return err
			}
		}
		created = append(created, file.name)
	}
	for _, file := range files {
		if !file.dir {
			done("create %s", rel(filepath.Join(dir, file.name)))
		}
	}
	return nil
}

func copyModuleTemplateFile(src, dst *os.Root, file moduleTemplateFile) error {
	var content io.Reader = bytes.NewReader(file.content)
	if file.source != "" {
		f, err := src.Open(file.source)
		if err != nil {
			return err
		}
		defer f.Close()
		content = f
	}
	f, err := dst.OpenFile(file.name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, content)
	if err := errors.Join(copyErr, f.Close()); err != nil {
		return errors.Join(err, dst.Remove(file.name))
	}
	return nil
}
