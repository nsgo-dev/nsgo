package android

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run the shipped reader on the JVM with Android's context and log calls
// stubbed. A 32 MiB heap makes trusting a corrupt length fail loudly;
// a real child process verifies that a bad pipe is stopped and reported.
func TestAndroidPipeRejectsCorruptFrames(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil || exec.Command(javac, "-version").Run() != nil {
		t.Skip("no JDK")
	}
	classes := t.TempDir()
	args := []string{"-d", classes, "java/dev/oneo/nsgo/Protocol.java", "java/dev/oneo/nsgo/NsgoEngine.java"}
	if err := filepath.WalkDir("testdata/pipe", func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".java") {
			args = append(args, path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(t.Context(), javac, args...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	java := filepath.Join(filepath.Dir(javac), "java")
	if out, err := exec.CommandContext(t.Context(), java, "-Xmx32m", "-cp", classes, "dev.oneo.nsgo.PipeCheck").CombinedOutput(); err != nil {
		t.Fatalf("java: %v\n%s", err, out)
	}
}
