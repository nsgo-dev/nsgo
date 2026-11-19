package backend

import (
	"errors"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// launchFlags are the engine's command-line flags, which any build of an app takes, with one dash or two:
//
//	--nsgo-fps               show the performance monitor
//	--nsgo-lang=zh-Hans      run in a locale, as core.Scope.SetLocale does
//	--nsgo-appearance=dark   start light, dark or following the system
//	--nsgo-os=ios            run a web build as macos, windows, linux, ios or android (web: as itself)
//
// They override nsgo.yaml and Run's options. Phones and the web take them as the platform launches apps with values: the simulator's and devicectl's arguments, an Android intent's extras (--ez nsgo-fps true, --es nsgo-lang de) and the page's query (?nsgo-fps&nsgo-lang=de).
type launchFlags struct {
	fps        bool
	lang       string
	appearance core.Appearance
	// os is the OS a web build runs as, --nsgo-os.
	os core.OS
	// appearanceSet is whether --nsgo-appearance was given, as its
	// "system" is the zero Appearance.
	appearanceSet bool
	// Windows' installer and COM start the app with these, never a user: --nsgo-register-toast=<shortcut> stamps the Start menu shortcut with the toast activator, --nsgo-unregister-toast removes the activator's registration, and --nsgo-toast-activation starts the app for a toast press, followed by the -Embedding COM appends.
	registerToast   string
	unregisterToast bool
	toastActivation bool
	// dockTask is the label of the app icon menu's item a Windows Jump List task starts the app with, --nsgo-dock-menu=<label> (desktop.SetDockMenu).
	dockTask string
	// launchedAtLogin is set by the login entries of macOS 11 and 12, Windows and Linux.
	launchedAtLogin bool
}

// launch holds the flags the app was started with.
var launch launchFlags

// simulates is whether the build runs as the OS --nsgo-os names, which only the web's does (run_web.go).
var simulates bool

// launchArgs are the program's arguments as it started, its launch flags among them but the Jump List task it chose and the login marker, which a restart passes again.
var launchArgs []string

// loginMarker is the flag the macOS 11/12 LaunchAgent, the Windows Run value and the Linux autostart entry start the app with, so it reports the launch as a login one (desktop.LaunchedAtLogin).
const loginMarker = "--nsgo-launched-at-login"

// The flags leave os.Args before main runs, so the app's own flag parsing never sees them.
func init() {
	launchArgs = restartArgs(os.Args)
	os.Args = launch.parse(os.Args)
}

// restartArgs are a program's arguments after its name, but a Jump List task's, which chose an item once, and the login marker, as the system does not start a restarted app at login. Arguments after "--" are the app's and stay.
func restartArgs(args []string) []string {
	if len(args) < 2 {
		return nil
	}
	var out []string
	for i, arg := range args[1:] {
		if arg == "--" {
			return append(out, args[1+i:]...)
		}
		dockTask := strings.HasPrefix(arg, "-") && strings.HasPrefix(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "nsgo-dock-menu=")
		if !dockTask && arg != loginMarker && arg != loginMarker[1:] {
			out = append(out, arg)
		}
	}
	return out
}

// parse takes the engine's flags out of args, a program's arguments, and returns the rest. A malformed or unknown one is logged and dropped. Arguments after "--" are the app's.
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
	case "os":
		o, ok := map[string]core.OS{"macos": core.OSMacOS, "windows": core.OSWindows, "linux": core.OSLinux, "ios": core.OSIOS, "android": core.OSAndroid, "web": core.OSWeb}[value]
		if !ok {
			return errors.New("needs macos, windows, linux, ios, android or web")
		}
		f.os = o
	default:
		return errors.New("no such flag: --nsgo-fps, --nsgo-lang, --nsgo-appearance or --nsgo-os")
	}
	return nil
}

// apply puts the flags over Run's options. It runs once every init has, so simulates is settled.
func (f *launchFlags) apply(o *Options) {
	o.Monitor = f.fps
	if f.appearanceSet {
		o.Appearance = f.appearance
	}
	o.Simulate = f.os
	if f.os != "" && f.os != core.OSWeb && !simulates {
		log.Print("nsgo: --nsgo-os: only a web build runs as another OS")
	}
}

// localized returns root, run in the locale --nsgo-lang names. The flag has the last word as the app starts, over a choice the app restores, and the user's later choices have theirs.
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
