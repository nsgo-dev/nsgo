//go:build darwin && !ios

package macos

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

type webHistoryAccess struct{}

func (webHistoryAccess) Allows(core.AccessRequest) bool { return true }
func (webHistoryAccess) WebHosts() []string             { return []string{"*"} }
func (webHistoryAccess) WebSchemes() []string           { return nil }

func TestWebOwnPagesReloadAndReturn(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if os.Getenv("NSGO_TEST_WEB_PROCESS") != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_WEB_PROCESS="+t.Name())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<p>next</p>")
	}))
	defer server.Close()
	finished := 0
	app := func(cx *core.Scope) core.Node {
		cx.ProvideAccess(webHistoryAccess{})
		var views []core.Node
		for i, src := range []core.WebSource{core.WebHTML("<p>own</p>", ""), core.WebSrc("data:text/html,<p>own</p>")} {
			var ref core.Ref
			phase := 0
			views = append(views, core.WebView{Source: src, Ref: &ref,
				Style: core.Style{Width: core.Pt(150), Height: core.Pt(100)},
				OnLoad: func(n core.WebNavigation) {
					phase++
					t.Logf("view %d phase %d: %s", i, phase, n.URL)
					if phase > 7 {
						return
					} // Restoration is not an extra test step.
					if (phase == 3 || phase == 6) && !n.CanGoBack {
						t.Errorf("view %d phase %d cannot go back", i, phase)
					}
					if (phase == 4 || phase == 5) && !n.CanGoForward {
						t.Errorf("view %d cannot go forward after going back", i)
					}
					ref.Evaluate("document.body.textContent.trim()", func(body string, err error) {
						want := "\"own\""
						if phase == 3 || phase == 6 {
							want = "\"next\""
						}
						if err != nil || body != want {
							t.Errorf("view %d phase %d body = %s, %v; want %s", i, phase, body, err, want)
							cx.Quit()
							return
						}
						switch phase {
						case 1:
							ref.Reload()
						case 2:
							ref.InjectJavaScript(fmt.Sprintf("location.href = %q", server.URL+"/next"))
						case 3, 6:
							ref.GoBack()
						case 4:
							ref.Reload()
						case 5:
							ref.GoForward()
						case 7:
							ref.InjectJavaScript(`{
const f = document.createElement('iframe');
let reloaded = false;
f.onload = () => {
  if (!reloaded) { reloaded = true; f.contentWindow.location.reload(); return; }
  location.href = 'about:blank';
};
f.src = 'about:blank';
document.body.append(f);
}`)
						}
					})
				},
				OnError: func(err error) {
					e, ok := err.(*core.WebError)
					if phase != 7 || !ok || e.URL != "about:blank" {
						t.Errorf("view %d phase %d: %v", i, phase, err)
						cx.Quit()
						return
					}
					finished++
					if finished == 2 {
						cx.Quit()
					}
				},
			})
		}
		return core.View{Style: core.Style{Direction: core.Row}, Children: views}
	}
	watchdog := time.AfterFunc(15*time.Second, func() { panic("the own pages did not reload and traverse history") })
	defer watchdog.Stop()
	onMain(func() { Run(app, Options{Title: "Web own history", Width: 300, Height: 100}) })
	if finished != 2 {
		t.Errorf("%d views completed, want 2", finished)
	}
}
