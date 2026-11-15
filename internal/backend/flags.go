package backend

import (
	"errors"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// launchFlags are the engine's command-line flags, which any build of an
// app takes, with one dash or two:
//
//	--nsgo-fps               show the performance monitor
//	--nsgo-lang=zh-Hans      run in a locale, as core.Scope.SetLocale does
//	--nsgo-appearance=dark   start light, dark or following the system
//
// They override nsgo.yaml and Run's options. Phones and the web take them
// as the platform launches apps with values: the simulator's and
// devicectl's arguments, an Android intent's extras (--ez nsgo-fps true,
// --es nsgo-lang de) and the page's query (?nsgo-fps&nsgo-lang=de).
type launchFlags struct {
	fps        bool
	lang       string
	appearance core.Appearance
	// appearanceSet is whether --nsgo-appearance was given, as its
	// "system" is the zero Appearance.
	appearanceSet bool
	// Windows' installer and COM start the app with these, never a user: --nsgo-register-toast=<shortcut> stamps the Start menu shortcut with the toast activator, --nsgo-unregister-toast removes the activator's registration, and --nsgo-toast-activation starts the app for a toast press, followed by the -Embedding COM appends.
	registerToast   string
	unregisterToast bool
	toastActivation bool
	// dockTask is the label of the app icon menu's item a Windows Jump List task starts the app with, --nsgo-dock-menu=<label> (desktop.SetDockMenu).
	dockTask string
	// launchedAtLogin is set by the legacy macOS LaunchAgent.
	launchedAtLogin bool
}

// launch holds the flags the app was started with.
var launch launchFlags

// launchArgs are the program's arguments as it started, its launch flags among them but the Jump List task it chose, which a restart passes again.
var launchArgs []string

// The flags leave os.Args before main runs, so the app's own flag
// parsing never sees them.
func init() {
	launchArgs = restartArgs(os.Args)
	os.Args = launch.parse(os.Args)
}

// restartArgs are a program's arguments after its name, but a Jump List task's, which chose an item once.
func restartArgs(args []string) []string {
	if len(args) < 2 {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(args[1:]), func(arg string) bool {
		return strings.HasPrefix(arg, "-") && strings.HasPrefix(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "nsgo-dock-menu=")
	})
}

// parse takes the engine's flags out of args, a program's arguments, and
// returns the rest. A malformed or unknown one is logged and dropped.
// Arguments after "--" are the app's.
func (f *launchFlags) parse(args []string) []string {
	if len(args) == 0 {
		return args
	}
	rest := []string{args[0]}
	for i, arg := range args[1:] {
		if arg == "--" {
			return append(rest, args[1+i:]...)
		}
		if f.toastActivation && (arg == "-Embedding" || arg == "/Embedding") {
			continue
		}
		name, ok := strings.CutPrefix(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "nsgo-")
		if !ok || !strings.HasPrefix(arg, "-") {
			rest = append(rest, arg)
			continue
		}
		if err := f.set(name); err != nil {
			log.Printf("nsgo: %s: %v", arg, err)
		}
	}
	return rest
}

// set applies one flag, name=value without its --nsgo- prefix.
func (f *launchFlags) set(flag string) error {
	name, value, hasValue := strings.Cut(flag, "=")
	switch name {
	case "fps":
		on := true
		if hasValue {
			var err error
			if on, err = strconv.ParseBool(value); err != nil {
				return err
			}
		}
		f.fps = on
	case "lang":
		if value == "" {
			return errors.New("needs a locale, as --nsgo-lang=zh-Hans")
		}
		f.lang = value
	case "appearance":
		a, ok := map[string]core.Appearance{"system": core.AppearanceSystem, "light": core.AppearanceLight, "dark": core.AppearanceDark}[value]
		if !ok {
			return errors.New("needs light, dark or system")
		}
		f.appearance, f.appearanceSet = a, true
	case "register-toast":
		if value == "" {
			return errors.New("needs the shortcut's path")
		}
		f.registerToast = value
	case "unregister-toast":
		f.unregisterToast = true
	case "toast-activation":
		f.toastActivation = true
	case "dock-menu":
		if value == "" {
			return errors.New("needs the item's label")
		}
		f.dockTask = value
	case "launched-at-login":
		f.launchedAtLogin = true
	default:
		return errors.New("no such flag: --nsgo-fps, --nsgo-lang or --nsgo-appearance")
	}
	return nil
}

// apply puts the flags over Run's options.
func (f *launchFlags) apply(o *Options) {
	o.Monitor = f.fps
	if f.appearanceSet {
		o.Appearance = f.appearance
	}
}

// localized returns root, run in the locale --nsgo-lang names. The flag
// has the last word as the app starts, over a choice the app restores,
// and the user's later choices have theirs.
func (f *launchFlags) localized(root engine.App) engine.App {
	if f.lang == "" {
		return root
	}
	return func(cx *core.Scope) core.Node {
		node := root(cx)
		cx.SetLocale(f.lang)
		return node
	}
}

// localizedApp is localized for the app's function.
func (f *launchFlags) localizedApp(app func(*core.Scope)) func(*core.Scope) {
	if f.lang == "" {
		return app
	}
	return func(cx *core.Scope) {
		app(cx)
		cx.SetLocale(f.lang)
	}
}
