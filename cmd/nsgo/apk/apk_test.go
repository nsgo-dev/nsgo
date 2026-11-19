package apk

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	androidruntime "github.com/nsgo-dev/nsgo/internal/platform/android/runtime"
)

// TestWrite checks the archive structure and, when the Android SDK's
// apksigner is installed, that the signature verifies.
func TestWrite(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = Write(&buf, androidruntime.Manifest, Manifest{
		Package:     "dev.oneo.nsgo.test",
		Label:       "Test",
		VersionName: "1.0.0",
		VersionCode: 10000,
		Icon:        []byte("\x89PNG icon"),
	}, []File{
		{Name: "classes.dex", Data: []byte("dex")},
		{Name: "lib/arm64-v8a/libnsgo.so", Data: []byte("go")},
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	apk := buf.Bytes()

	// The archive the system reads: an end of directory record whose
	// central directory offset names the directory after the signing
	// block, and stored entries aligned for mapping.
	reader, err := zip.NewReader(bytes.NewReader(apk), int64(len(apk)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range reader.File {
		names[f.Name] = true
		if f.Method == zip.Store {
			offset, err := f.DataOffset()
			if err != nil {
				t.Fatal(err)
			}
			if offset%4 != 0 {
				t.Errorf("%s starts at %d, not a 4-byte boundary", f.Name, offset)
			}
		}
	}
	for _, want := range []string{"AndroidManifest.xml", "classes.dex", "lib/arm64-v8a/libnsgo.so", iconFile, "resources.arsc"} {
		if !names[want] {
			t.Errorf("no %s entry", want)
		}
	}

	eocd := bytes.LastIndex(apk, []byte{'P', 'K', 5, 6})
	if eocd < 0 {
		t.Fatal("no end of directory record")
	}
	cd := int(binary.LittleEndian.Uint32(apk[eocd+16:]))
	size := int(binary.LittleEndian.Uint32(apk[eocd+12:]))
	if cd+size != eocd {
		t.Fatalf("central directory %d+%d does not end at %d", cd, size, eocd)
	}
	if cd < len(blockMagic) || !bytes.HasSuffix(apk[:cd], []byte(blockMagic)) {
		t.Fatal("no signing block before the central directory")
	}

	apksigner, err := tool("apksigner")
	if err != nil {
		t.Skip("apksigner is not installed")
	}
	// The SDK's apksigner script runs the java on PATH, which on macOS is a stub that starts the JDK JAVA_HOME names, else the one java_home finds, and hangs with none.
	env := os.Environ()
	java, err := exec.LookPath("java")
	if jdk := jdkHome(); jdk != "" {
		java, err = filepath.Join(jdk, "bin", "java"), nil
		env = append(env, "JAVA_HOME="+jdk, "PATH="+filepath.Join(jdk, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	if err != nil {
		t.Skip("apksigner needs a JDK")
	}
	if why, ok := javaRuns(java, env); !ok {
		t.Skipf("apksigner needs a working JDK: %s", why)
	}
	path := filepath.Join(t.TempDir(), "test.apk")
	if err := os.WriteFile(path, apk, 0o600); err != nil {
		t.Fatal(err)
	}
	// Bounded: a tool that waits on something (a missing JDK's dialog, a
	// locked keystore) must not hold the suite for the go test default.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, apksigner, "verify", "--verbose", path)
	cmd.WaitDelay = 5 * time.Second
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("apksigner did not finish in a minute:\n%s", out)
	}
	if err != nil {
		t.Fatalf("apksigner: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Verified using v2 scheme (APK Signature Scheme v2): true") {
		t.Fatalf("apksigner did not verify the v2 signature:\n%s", out)
	}
}

func TestAdaptiveIconResources(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, androidruntime.Manifest, Manifest{
		Package: "dev.oneo.nsgo.test", Label: "Test", VersionName: "1", VersionCode: 1,
		Icon: []byte("legacy"), Adaptive: &AdaptiveIcon{Foreground: []byte("fg"), Background: []byte("bg"), Monochrome: []byte("mono")},
		Names: map[string]string{"": "Test", "zh-Hans": "测试"},
	}, nil, key); err != nil {
		t.Fatal(err)
	}
	aapt2, err := tool("aapt2")
	if err != nil {
		t.Skip("aapt2 is not installed")
	}
	path := filepath.Join(t.TempDir(), "adaptive.apk")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	resources, err := exec.Command(aapt2, "dump", "resources", path).CombinedOutput()
	if err != nil {
		t.Fatalf("aapt2 dump resources: %v\n%s", err, resources)
	}
	for _, want := range []string{"0x7f010000 mipmap/ic_launcher\n", "mipmap/ic_launcher_foreground", "mipmap/ic_launcher_background", "mipmap/ic_launcher_monochrome", iconFile, adaptiveIconFile, "string/app_name"} {
		if !strings.Contains(string(resources), want) {
			t.Errorf("resource dump lacks %q:\n%s", want, resources)
		}
	}
	xml, err := exec.Command(aapt2, "dump", "xmltree", path, "--file", adaptiveIconFile).CombinedOutput()
	if err != nil {
		t.Fatalf("aapt2 dump adaptive XML: %v\n%s", err, xml)
	}
	for _, want := range []string{"adaptive-icon", "foreground", "background", "monochrome", "@0x7f010001", "@0x7f010002", "@0x7f010003"} {
		if !strings.Contains(string(xml), want) {
			t.Errorf("adaptive XML lacks %q:\n%s", want, xml)
		}
	}
}

// jdkHome returns the JDK to run apksigner with: the one JAVA_HOME names, else on macOS the one /usr/libexec/java_home finds, else the one this machine may keep beside the SDK in ~/Android/jdk. It returns "" when there is none, leaving the java on PATH to apksigner.
func jdkHome() string {
	hasJava := func(home string) bool {
		_, err := os.Stat(filepath.Join(home, "bin", "java"))
		return home != "" && err == nil
	}
	if home := os.Getenv("JAVA_HOME"); hasJava(home) {
		return home
	}
	if runtime.GOOS == "darwin" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, "/usr/libexec/java_home").Output(); err == nil {
			if home := strings.TrimSpace(string(out)); hasJava(home) {
				return home
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if jdk := filepath.Join(home, "Android", "jdk"); hasJava(jdk) {
			return jdk
		}
	}
	return ""
}

// javaRuns reports whether java starts under env, and says why when it does not. macOS's /usr/bin/java is a stub that asks for a JDK installation, and blocks when a tool runs it with a jar.
func javaRuns(java string, env []string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, java, "-version")
	cmd.Env = env
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if ctx.Err() != nil {
		return "java did not answer in 10s", false
	}
	if err != nil || strings.Contains(text, "Unable to locate a Java Runtime") {
		return text, false
	}
	return "", true
}

// tool finds one of the Android SDK's build tools, on PATH or in an SDK
// that ANDROID_HOME, ANDROID_SDK_ROOT or the usual directory holds. The
// APK writer does not need them; the tests use them to check its work.
func tool(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	var dirs []string
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if dir := os.Getenv(env); dir != "" {
			dirs = append(dirs, dir)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs,
			filepath.Join(home, "Android"),
			filepath.Join(home, "Library", "Android", "sdk"))
	}
	var found []string
	for _, dir := range dirs {
		matches, _ := filepath.Glob(filepath.Join(dir, "build-tools", "*", name))
		found = append(found, matches...)
	}
	if len(found) == 0 {
		return "", exec.ErrNotFound
	}
	sort.Strings(found)
	return found[len(found)-1], nil
}
