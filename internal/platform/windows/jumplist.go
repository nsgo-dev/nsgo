//go:build windows

package windows

import (
	"log"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// The app's Jump List (core.Scope.SetDockMenu) and recent documents (core.Scope.AddRecentFile).

var (
	setCurrentProcessExplicitAppUserModelID = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	shAddToRecentDocs                       = shell32.NewProc("SHAddToRecentDocs")

	clsidDestinationList         = parseGUID("77f10cf0-3db5-4966-b520-b7c54fd35ed6")
	iidCustomDestinationList     = parseGUID("6332debf-87b5-4670-90c0-5e57b408a49e")
	clsidApplicationDestinations = parseGUID("86c14003-4d6b-4ef3-a7b4-0506663b2e68")
	iidApplicationDestinations   = parseGUID("12337d35-94c6-48a0-bce7-6a9c69d4d600")
	clsidObjectCollection        = parseGUID("2d3468c1-36a7-43b6-ac24-d3f02fd9607a")
	iidObjectCollection          = parseGUID("5632b1a4-e38a-400a-928a-d4cd63230295")
	iidObjectArray               = parseGUID("92ca9dcd-5622-4bba-a805-5e9f541bd8c9")
	pkeyTitle                    = propertyKey{fmtid: parseGUID("f29f85e0-4ff9-1068-ab91-08002b27b3d9"), pid: 2}
)

const (
	// ICustomDestinationList, IApplicationDestinations, IObjectCollection and IShellLinkW methods after IUnknown.
	destinationListSetAppID            = 3
	destinationListBeginList           = 4
	destinationListAppendKnownCategory = 6
	destinationListAddUserTasks        = 7
	destinationListCommitList          = 8
	destinationListDeleteList          = 10
	destinationListAbortList           = 11
	applicationDestinationsSetAppID    = 3
	applicationDestinationsRemoveAll   = 5
	objectCollectionAddObject          = 5
	shellLinkSetArguments              = 11
	shellLinkSetIconLocation           = 17
	shellLinkSetPath                   = 20

	knownCategoryRecent = 2 // KDC_RECENT
	shardPathW          = 3 // SHARD_PATHW
)

// dockTaskArgument starts the argument a Jump List task starts the app's exe with, a launch flag the backend takes out of os.Args (Options.DockTask); the rest is the label of the app icon menu's item it chooses. A label rather than the item's ID, which the next run of the app numbers anew.
const dockTaskArgument = "--nsgo-dock-menu="

// jumpTasks are the app icon menu's entries the Jump List was written from last.
var jumpTasks []driver.MenuEntry

// setProcessAppUserModelID gives the process the app's AppUserModelID, which the installer stamps on the Start menu shortcut and toasts use, so the taskbar button, the shortcut and the Jump List are the one app's, though it was started from its exe. An app without an ID keeps the one Windows makes from its exe's path.
func setProcessAppUserModelID(id string) {
	if id == "" {
		return
	}
	if hr, _, _ := setCurrentProcessExplicitAppUserModelID.Call(uintptr(unsafe.Pointer(wide(id)))); failed(hr) {
		log.Printf("nsgo: SetCurrentProcessExplicitAppUserModelID: %#x", uint32(hr))
	}
}

// jumpTask reports whether e is a Jump List task: an enabled top-level action with a label, as tasks have no submenus, separators or check marks.
func jumpTask(e driver.MenuEntry) bool {
	return e.ID != 0 && e.Label != "" && !e.Disabled && !e.Separator && len(e.Items) == 0
}

// dockTask returns the label of the task args name, those a second launch handed over.
func dockTask(args []string) (string, bool) {
	for _, arg := range args {
		if label, ok := strings.CutPrefix(arg, dockTaskArgument); ok && label != "" {
			return label, true
		}
	}
	return "", false
}

// setDockMenu writes the app's Jump List: the top-level actions of entries as tasks, each starting the app's exe with dockTaskArgument and its label, and the Recent category of the documents AddRecentFile noted. Without a task it deletes the list, which leaves the one Windows makes itself.
func (w *window) setDockMenu(entries []driver.MenuEntry) {
	jumpTasks = entries
	if w.opts.test != nil {
		return
	}
	var tasks []driver.MenuEntry
	for _, e := range entries {
		if jumpTask(e) {
			tasks = append(tasks, e)
		}
	}
	list := newCOMObject(&clsidDestinationList, &iidCustomDestinationList)
	if list == nil {
		return
	}
	defer list.release()
	var appID *uint16
	if w.opts.ID != "" {
		appID = wide(w.opts.ID)
		if hr := list.call(destinationListSetAppID, uintptr(unsafe.Pointer(appID))); failed(hr) {
			log.Printf("nsgo: Jump List: SetAppID: %#x", uint32(hr))
			return
		}
	}
	if len(tasks) == 0 {
		list.call(destinationListDeleteList, uintptr(unsafe.Pointer(appID)))
		return
	}
	exe, err := os.Executable()
	if err != nil {
		log.Printf("nsgo: Jump List: %v", err)
		return
	}
	var slots uint32
	var removed *comObject
	if hr := list.call(destinationListBeginList, uintptr(unsafe.Pointer(&slots)), uintptr(unsafe.Pointer(&iidObjectArray)), uintptr(unsafe.Pointer(&removed))); failed(hr) {
		log.Printf("nsgo: Jump List: BeginList: %#x", uint32(hr))
		return
	}
	removed.release()
	collection := newCOMObject(&clsidObjectCollection, &iidObjectCollection)
	if collection == nil {
		list.call(destinationListAbortList)
		return
	}
	defer collection.release()
	for _, e := range tasks {
		if link := taskLink(exe, e.Label); link != nil {
			collection.call(objectCollectionAddObject, link.ptr())
			link.release()
		}
	}
	// It fails while the app is registered for no file type, and the list goes without the category.
	list.call(destinationListAppendKnownCategory, knownCategoryRecent)
	step, hr := "AddUserTasks", list.call(destinationListAddUserTasks, collection.ptr())
	if !failed(hr) {
		step, hr = "CommitList", list.call(destinationListCommitList)
	}
	if failed(hr) {
		log.Printf("nsgo: Jump List: %s: %#x", step, uint32(hr))
		list.call(destinationListAbortList)
	}
}

// taskLink returns a shell link that starts exe to choose the task labeled label, titled label and with the exe's icon, nil if it cannot be made.
func taskLink(exe, label string) *comObject {
	link := newCOMObject(&clsidShellLink, &iidShellLink)
	if link == nil {
		return nil
	}
	store := link.query(&iidPropertyStore)
	if store == nil {
		link.release()
		return nil
	}
	defer store.release()
	path, args, title := wide(exe), wide(syscall.EscapeArg(dockTaskArgument+label)), wide(label)
	// A task shows its link's System.Title.
	titleVariant := propVariant{vt: 31, value: unsafe.Pointer(title)} // VT_LPWSTR
	ok := !failed(link.call(shellLinkSetPath, uintptr(unsafe.Pointer(path)))) &&
		!failed(link.call(shellLinkSetArguments, uintptr(unsafe.Pointer(args)))) &&
		!failed(link.call(shellLinkSetIconLocation, uintptr(unsafe.Pointer(path)), 0)) &&
		!failed(store.call(propertyStoreSetValue, uintptr(unsafe.Pointer(&pkeyTitle)), uintptr(unsafe.Pointer(&titleVariant)))) &&
		!failed(store.call(propertyStoreCommit))
	if !ok {
		link.release()
		return nil
	}
	return link
}

// recentFile adds path to the documents the shell keeps as recent, which the Jump List's Recent category shows for the file types the app is registered for, or forgets the app's when clearRecent, leaving other apps'.
func (w *window) recentFile(path string, clearRecent bool) {
	if w.opts.test != nil {
		return
	}
	if !clearRecent {
		shAddToRecentDocs.Call(shardPathW, uintptr(unsafe.Pointer(wide(path))))
		return
	}
	dests := newCOMObject(&clsidApplicationDestinations, &iidApplicationDestinations)
	if dests == nil {
		return
	}
	defer dests.release()
	if w.opts.ID != "" {
		if hr := dests.call(applicationDestinationsSetAppID, uintptr(unsafe.Pointer(wide(w.opts.ID)))); failed(hr) {
			log.Printf("nsgo: recent documents: SetAppID: %#x", uint32(hr))
			return
		}
	}
	dests.call(applicationDestinationsRemoveAll)
}

func newCOMObject(clsid, iid *guid) *comObject {
	var object *comObject
	hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&object)))
	if failed(hr) {
		log.Printf("nsgo: CoCreateInstance: %#x", uint32(hr))
		return nil
	}
	return object
}
