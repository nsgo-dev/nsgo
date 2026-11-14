package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestInstallUpdate(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	pkg := filepath.Join(t.TempDir(), "hello-1.1.apk")
	if err := os.WriteFile(pkg, []byte("PK"), 0o600); err != nil {
		t.Fatal(err)
	}
	was := driver.UpdateInstall
	t.Cleanup(func() { driver.UpdateInstall = was })

	// Off Android, the backend's installer, or none.
	driver.UpdateInstall = nil
	var err error
	InstallUpdate(cx, pkg, func(e error) { err = e })
	if err == nil {
		t.Error("installed without an installer")
	}
	driver.UpdateInstall = func(path string, done func(error)) { done(errors.New("exit status 1")) }
	InstallUpdate(cx, pkg, func(e error) { err = e })
	h.drain()
	if err == nil || err.Error() != "exit status 1" {
		t.Errorf("the installer's failure answered %v", err)
	}

	// Android's package installer, which may not start.
	h.tree.SetEnvironment(Environment{OS: OSAndroid})
	h.commit()
	driver.UpdateInstall = func(string, func(error)) { t.Error("Android ran the desktop installer") }
	err = nil
	InstallUpdate(cx, pkg, func(e error) { err = e })
	it := command(h.commit(), driver.ItemInstallPackage)
	if it == nil || it.Open.Path != pkg {
		t.Fatalf("install = %+v", it)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Finished: false})
	if err == nil {
		t.Error("an installer that did not start answered no error")
	}
	InstallUpdate(cx, pkg, func(e error) { err = e })
	it = command(h.commit(), driver.ItemInstallPackage)
	h.tree.Dispatch(driver.Event{Kind: driver.EventFiles, Count: it.Count, Finished: true})
	if err != nil {
		t.Errorf("a started installer answered %v", err)
	}
}
