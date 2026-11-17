//go:build darwin && !ios

package macos

import (
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

func TestBackdropTintFollowsTransitionsAndAnimatedBackgrounds(t *testing.T) {
	for _, material := range []driver.BackdropMaterial{driver.BackdropUltraThin, driver.BackdropThin} {
		name := "radius"
		if material == driver.BackdropThin {
			name = "material"
		}
		t.Run(name, func(t *testing.T) {
			onMain(func() {
				loadAppKit()
				s := &surface{native: objc.ID(classView).Send(selNew), animator: driver.NewAnimator(nil)}
				defer s.native.Send(selRelease)
				v := s.create(driver.KindView, 1)
				s.views = []*view{nil, v}
				defer s.delete(v, false)
				v.native.Send(selSetFrame, apple.Rect(0, 0, 200, 120))
				style := driver.ViewStyle{Opacity: 1, Background: driver.RGBA(0, 0.2), Material: material,
					Effects: &driver.Effects{BackdropFilters: []driver.Filter{{Kind: driver.FilterBlur, Amount: 10}}},
					Transition: driver.Transition{Duration: time.Second, Delay: 100 * time.Millisecond,
						Properties: driver.TransitionBackground, Easing: driver.Curve{X1: 0.2, Y1: 0.3, X2: 0.4, Y2: 0.5}}}
				apply := func() {
					s.apply(&driver.MountBatch{Root: 1, Items: []driver.MountItem{{Kind: driver.ItemViewStyle, ID: 1, ViewStyle: &style}}})
				}
				animation := func() objc.ID {
					return v.tint.Send(selLayer).Send(sel("animationForKey:"), apple.NSString("nsgo.backgroundColor"))
				}
				color := func(got objc.ID, want driver.Color) {
					t.Helper()
					expected := apple.CGColor(want)
					defer apple.Release(expected)
					if got == 0 || !objc.Send[bool](got, sel("isEqual:"), expected) {
						t.Errorf("tint color %v does not match %#x", got, want)
					}
				}
				apply()
				if v.tint == 0 {
					t.Fatal("no backdrop tint")
				}
				if animation() != 0 {
					t.Error("initial style starts a transition")
				}
				color(v.tint.Send(selLayer).Send(sel("backgroundColor")), style.Background)
				from := style.Background
				style.Background = driver.RGBA(0x224466, 0.8)
				apply()
				anim := animation()
				if anim == 0 {
					t.Fatal("a backdrop background change starts no tint transition")
				}
				color(anim.Send(sel("fromValue")), from)
				if got := objc.Send[float64](anim, sel("duration")); got != 1 {
					t.Errorf("duration = %v, want 1 second", got)
				}
				if objc.Send[float64](anim, sel("beginTime")) == 0 || apple.GoString(anim.Send(sel("fillMode"))) != "backwards" {
					t.Error("the tint transition lost its delay")
				}
				// An opacity-only animation frame must not restart or remove the tint transition.
				before := objc.Send[float64](anim, sel("beginTime"))
				v.animated = driver.AnimatedValues{Set: 1 << driver.PropOpacity, Opacity: 0.5}
				apple.WithoutAnimation(v.restyle)
				if animation() == 0 || objc.Send[float64](animation(), sel("beginTime")) != before {
					t.Error("an unrelated animated property replaced the tint transition")
				}
				// Background values override both the model style and any active tint transition.
				v.animated = driver.AnimatedValues{Set: 1 << driver.PropBackground, Background: driver.RGB(0xff0000)}
				apply()
				color(v.tint.Send(selLayer).Send(sel("backgroundColor")), v.animated.Background)
				if animation() != 0 {
					t.Error("a background binding left the style transition running")
				}
				v.animated.Background = driver.RGB(0x00ff00)
				apple.WithoutAnimation(v.restyle)
				color(v.tint.Send(selLayer).Send(sel("backgroundColor")), v.animated.Background)
				v.animated = driver.AnimatedValues{}
				apple.WithoutAnimation(v.restyle)
				color(v.tint.Send(selLayer).Send(sel("backgroundColor")), style.Background)
				style.Transition.Properties = driver.TransitionOpacity
				style.Background = driver.RGB(0x0000ff)
				apply()
				if animation() != 0 {
					t.Error("a transition excluding background animates the tint")
				}
				color(v.tint.Send(selLayer).Send(sel("backgroundColor")), style.Background)
				style.Transition.Properties = 0 // Default transitions include background.
				style.Background = driver.RGB(0xffffff)
				apply()
				if animation() == 0 {
					t.Error("the default transition excludes the tint")
				}
				style.Transition = driver.Transition{}
				style.Background = 0
				apply()
				if animation() != 0 || v.tint.Send(selLayer).Send(sel("backgroundColor")) != 0 {
					t.Error("clearing the background without a transition leaves a tint")
				}
				style.Effects = nil
				apply()
				if v.tint != 0 {
					t.Error("removing the blur keeps its tint")
				}
				style.Effects = &driver.Effects{BackdropFilters: []driver.Filter{{Kind: driver.FilterBlur, Amount: 10}}}
				style.Background = driver.RGB(0x884422)
				style.Transition = driver.Transition{Duration: time.Second}
				apply()
				if animation() != 0 {
					t.Error("a recreated tint starts a transition")
				}
				color(v.tint.Send(selLayer).Send(sel("backgroundColor")), style.Background)
			})
		})
	}
}
