package core

import (
	"fmt"
	"strings"
	"testing"
)

func TestCommitChecksBelongToTheirGraph(t *testing.T) {
	for _, checked := range []bool{false, true, false, true} {
		t.Run(fmt.Sprint(checked), func(t *testing.T) {
			t.Parallel()
			root := NewRoot(RootOptions{CheckCommits: checked})
			defer root.Dispose()
			tree := NewTree(root, ApproxTextEngine{})
			tree.Resize(Size{Width: 200, Height: 100}, Geometry{})
			tree.Mount(func(*Scope) Node {
				return View{Style: Style{Width: Pt(40), Height: Pt(20)}}
			})
			tree.Commit()
			// Deliberately bypass the dirty marks. Only this graph's
			// full-layout diagnostic can find the missed width change.
			tree.view.children[0].style.Width = Pt(80)
			tree.changed = true
			defer func() {
				r := recover()
				if checked && (r == nil || !strings.Contains(fmt.Sprint(r), "commit")) {
					t.Errorf("checked graph missed an unmarked change: %v", r)
				} else if !checked && r != nil {
					t.Errorf("unchecked graph ran the diagnostic: %v", r)
				}
			}()
			tree.Commit()
		})
	}
}
