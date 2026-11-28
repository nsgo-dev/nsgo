//go:build ios

package shareextension

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/platform/objcblock"
	"github.com/nsgo-dev/nsgo/internal/platform/shareinbox"
)

const uikitPath = "/System/Library/Frameworks/UIKit.framework/UIKit"

var uikit = sync.OnceValue(func() uintptr {
	lib, err := purego.Dlopen(uikitPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&accessibilityPost, lib, "UIAccessibilityPostNotification")
	return lib
})

var accessibilityPost func(notification uint32, argument objc.ID)

// symbol is the value of UIKit's global variable name, as a font's text style, which is a pointer the size of T.
func symbol[T any](name string) T {
	addr, err := purego.Dlsym(uikit(), name)
	if err != nil {
		panic(err)
	}
	return **(**T)(unsafe.Pointer(&addr))
}

// localized is the extension's word key in the language UIKit picked for its bundle from the user's, which nsgo build writes into each .lproj's Localizable.strings from mobile/locales and the app's catalogs.
func localized(key string) objc.ID {
	return objc.ID(objc.GetClass("NSBundle")).Send(sel("mainBundle")).Send(sel("localizedStringForKey:value:table:"), ns(key), objc.ID(0), objc.ID(0))
}

// sheet is what the extension shows while it receives a share: the app's icon and name over a spinner, then whether the share was saved, with a button that closes it.
type sheet struct {
	spinner, message, button objc.ID
}

// newSheet lays the sheet out in the controller's view: a card in the middle of a clear view, so the host's share sheet shows around it rather than a page of the extension's own.
func newSheet(controller objc.ID) *sheet {
	view := controller.Send(sel("view"))
	view.Send(sel("setBackgroundColor:"), objc.ID(objc.GetClass("UIColor")).Send(sel("clearColor")))
	card := objc.ID(objc.GetClass("UIView")).Send(sel("new"))
	card.Send(sel("setBackgroundColor:"), objc.ID(objc.GetClass("UIColor")).Send(sel("secondarySystemGroupedBackgroundColor")))
	card.Send(sel("layer")).Send(sel("setCornerRadius:"), float64(20))
	card.Send(sel("layer")).Send(sel("setCornerCurve:"), ns("continuous"))
	card.Send(sel("setTranslatesAutoresizingMaskIntoConstraints:"), false)
	view.Send(sel("addSubview:"), card)
	card.Send(sel("release"))

	stack := objc.ID(objc.GetClass("UIStackView")).Send(sel("new"))
	stack.Send(sel("setAxis:"), int(1))      // UILayoutConstraintAxisVertical
	stack.Send(sel("setAlignment:"), int(3)) // UIStackViewAlignmentCenter
	stack.Send(sel("setSpacing:"), float64(12))
	stack.Send(sel("setTranslatesAutoresizingMaskIntoConstraints:"), false)
	card.Send(sel("addSubview:"), stack)
	stack.Send(sel("release"))

	if image := appIcon(); image != 0 {
		icon := objc.ID(objc.GetClass("UIImageView")).Send(sel("alloc")).Send(sel("initWithImage:"), image)
		icon.Send(sel("setTranslatesAutoresizingMaskIntoConstraints:"), false)
		icon.Send(sel("layer")).Send(sel("setCornerRadius:"), float64(14))
		icon.Send(sel("layer")).Send(sel("setCornerCurve:"), ns("continuous"))
		icon.Send(sel("setClipsToBounds:"), true)
		for _, anchor := range []string{"widthAnchor", "heightAnchor"} {
			icon.Send(sel(anchor)).Send(sel("constraintEqualToConstant:"), float64(64)).Send(sel("setActive:"), true)
		}
		stack.Send(sel("addArrangedSubview:"), icon)
		icon.Send(sel("release"))
	}
	name := label("UIFontTextStyleHeadline", "labelColor")
	name.Send(sel("setText:"), localized("appName"))
	name.Send(sel("setAccessibilityTraits:"), symbol[uint64]("UIAccessibilityTraitHeader"))
	stack.Send(sel("addArrangedSubview:"), name)
	name.Send(sel("release"))

	s := &sheet{}
	s.spinner = objc.ID(objc.GetClass("UIActivityIndicatorView")).Send(sel("alloc")).Send(sel("initWithActivityIndicatorStyle:"), int(100)) // UIActivityIndicatorViewStyleMedium
	s.spinner.Send(sel("startAnimating"))
	stack.Send(sel("addArrangedSubview:"), s.spinner)
	s.spinner.Send(sel("release"))
	s.message = label("UIFontTextStyleSubheadline", "secondaryLabelColor")
	s.message.Send(sel("setHidden:"), true)
	stack.Send(sel("addArrangedSubview:"), s.message)
	s.message.Send(sel("release"))
	s.button = objc.ID(objc.GetClass("UIButton")).Send(sel("buttonWithType:"), int(1)) // UIButtonTypeSystem
	s.button.Send(sel("setTitle:forState:"), localized("shareCancel"), uint(0))
	s.button.Send(sel("titleLabel")).Send(sel("setFont:"), objc.ID(objc.GetClass("UIFont")).Send(sel("preferredFontForTextStyle:"), symbol[objc.ID]("UIFontTextStyleBody")))
	s.button.Send(sel("titleLabel")).Send(sel("setAdjustsFontForContentSizeCategory:"), true)
	s.button.Send(sel("addTarget:action:forControlEvents:"), controller, sel("nsgoClose:"), uint(1<<6)) // UIControlEventTouchUpInside
	stack.Send(sel("addArrangedSubview:"), s.button)

	margins := view.Send(sel("layoutMarginsGuide"))
	width := card.Send(sel("widthAnchor")).Send(sel("constraintEqualToConstant:"), float64(320))
	width.Send(sel("setPriority:"), float32(750)) // UILayoutPriorityDefaultHigh: narrower on a narrow screen
	for _, c := range []objc.ID{
		card.Send(sel("centerXAnchor")).Send(sel("constraintEqualToAnchor:"), view.Send(sel("centerXAnchor"))),
		card.Send(sel("centerYAnchor")).Send(sel("constraintEqualToAnchor:"), view.Send(sel("centerYAnchor"))),
		card.Send(sel("leadingAnchor")).Send(sel("constraintGreaterThanOrEqualToAnchor:"), margins.Send(sel("leadingAnchor"))),
		card.Send(sel("topAnchor")).Send(sel("constraintGreaterThanOrEqualToAnchor:"), margins.Send(sel("topAnchor"))),
		width,
		stack.Send(sel("topAnchor")).Send(sel("constraintEqualToAnchor:constant:"), card.Send(sel("topAnchor")), float64(24)),
		stack.Send(sel("bottomAnchor")).Send(sel("constraintEqualToAnchor:constant:"), card.Send(sel("bottomAnchor")), float64(-16)),
		stack.Send(sel("leadingAnchor")).Send(sel("constraintEqualToAnchor:constant:"), card.Send(sel("leadingAnchor")), float64(20)),
		stack.Send(sel("trailingAnchor")).Send(sel("constraintEqualToAnchor:constant:"), card.Send(sel("trailingAnchor")), float64(-20)),
		s.message.Send(sel("widthAnchor")).Send(sel("constraintEqualToAnchor:"), stack.Send(sel("widthAnchor"))),
		name.Send(sel("widthAnchor")).Send(sel("constraintEqualToAnchor:"), stack.Send(sel("widthAnchor"))),
	} {
		c.Send(sel("setActive:"), true)
	}
	return s
}

func label(style, color string) objc.ID {
	l := objc.ID(objc.GetClass("UILabel")).Send(sel("new"))
	l.Send(sel("setFont:"), objc.ID(objc.GetClass("UIFont")).Send(sel("preferredFontForTextStyle:"), symbol[objc.ID](style)))
	l.Send(sel("setAdjustsFontForContentSizeCategory:"), true)
	l.Send(sel("setTextColor:"), objc.ID(objc.GetClass("UIColor")).Send(sel(color)))
	l.Send(sel("setTextAlignment:"), int(1)) // NSTextAlignmentCenter
	l.Send(sel("setNumberOfLines:"), int(0))
	return l
}

// appIcon is the containing app's icon, which its bundle two directories up names in CFBundleIcons, or 0.
func appIcon() objc.ID {
	url := objc.ID(objc.GetClass("NSBundle")).Send(sel("mainBundle")).Send(sel("bundleURL")).Send(sel("URLByDeletingLastPathComponent")).Send(sel("URLByDeletingLastPathComponent"))
	app := objc.ID(objc.GetClass("NSBundle")).Send(sel("bundleWithURL:"), url)
	if app == 0 {
		return 0
	}
	primary := app.Send(sel("objectForInfoDictionaryKey:"), ns("CFBundleIcons")).Send(sel("objectForKey:"), ns("CFBundlePrimaryIcon"))
	if primary == 0 {
		return 0
	}
	var names []objc.ID
	if files := primary.Send(sel("objectForKey:"), ns("CFBundleIconFiles")); files != 0 {
		// The last file is the largest.
		for i := objc.Send[uint](files, sel("count")); i > 0; i-- {
			names = append(names, files.Send(sel("objectAtIndex:"), i-1))
		}
	}
	if name := primary.Send(sel("objectForKey:"), ns("CFBundleIconName")); name != 0 {
		names = append(names, name)
	}
	for _, name := range names {
		if image := objc.ID(objc.GetClass("UIImage")).Send(sel("imageNamed:inBundle:compatibleWithTraitCollection:"), name, app, objc.ID(0)); image != 0 {
			return image
		}
	}
	return 0
}

// show replaces the spinner with the import's result and the Cancel button with Done, and says the result to VoiceOver.
func (s *sheet) show(err error) {
	key := "shareSaved"
	switch {
	case err == nil:
	case errors.Is(err, shareinbox.ErrTooLarge):
		key = "shareTooLarge"
	default:
		key = "shareFailed"
	}
	onMain(func() {
		text := localized(key)
		s.spinner.Send(sel("stopAnimating"))
		s.spinner.Send(sel("setHidden:"), true)
		s.message.Send(sel("setText:"), text)
		s.message.Send(sel("setHidden:"), false)
		s.button.Send(sel("setTitle:forState:"), localized("shareDone"), uint(0))
		accessibilityPost(symbol[uint32]("UIAccessibilityAnnouncementNotification"), text)
	})
}

// onMain runs fn on UIKit's main operation queue.
func onMain(fn func()) {
	block := objcblock.New(func(objc.Block) { fn() })
	objc.ID(objc.GetClass("NSOperationQueue")).Send(sel("mainQueue")).Send(sel("addOperationWithBlock:"), block)
	block.Release()
}
