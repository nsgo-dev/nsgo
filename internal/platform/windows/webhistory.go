//go:build windows

package windows

import (
	"encoding/json"
	"strconv"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// WebView2 exposes history's kinds in NavigationStarting, but not its entry identities. CDP supplies those identities without asking page JavaScript.
type webHistorySnapshot struct {
	CurrentIndex int `json:"currentIndex"`
	Entries      []struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	} `json:"entries"`
}

func (wv *webView) devTools(method, params string, done func(string, error)) {
	var h *comHandler
	h = newHandler(&iidDevToolsCompleted, func(hr uintptr, result unsafe.Pointer) {
		defer h.release()
		if wv.closed {
			return
		}
		if failed(hr) {
			done("", hresultError("nsgo: the web view could not read or navigate its history", hr))
			return
		}
		done(wideString((*uint16)(result)), nil)
	})
	if hr := wv.web.call(webCallDevTools, uintptr(unsafe.Pointer(wide(method))), uintptr(unsafe.Pointer(wide(params))), h.ptr()); failed(hr) {
		h.release()
		done("", hresultError("nsgo: the web view does not provide its navigation history", hr))
	}
}

func (wv *webView) readHistory(done func(*webHistorySnapshot)) {
	wv.devTools("Page.getNavigationHistory", "{}", func(data string, err error) {
		var history webHistorySnapshot
		if err == nil {
			err = json.Unmarshal([]byte(data), &history)
		}
		if err != nil {
			wv.report(driver.Event{Kind: driver.EventError, Text: err.Error()})
			done(nil)
			return
		}
		live := make(map[int]bool, len(history.Entries))
		for _, entry := range history.Entries {
			live[entry.ID] = true
		}
		for id := range wv.ownEntries {
			if !live[id] {
				delete(wv.ownEntries, id)
			}
		}
		done(&history)
	})
}

func (wv *webView) historyCommand(command driver.WebCommand) {
	started := wv.started
	wv.readHistory(func(history *webHistorySnapshot) {
		if history == nil || wv.started != started {
			return
		}
		index := history.CurrentIndex
		switch command {
		case driver.WebGoBack:
			index--
		case driver.WebGoForward:
			index++
		}
		if index < 0 || index >= len(history.Entries) {
			return
		}
		entry := history.Entries[index]
		source := wv.ownEntries[entry.ID]
		wv.cut = wv.started
		wv.historySource, wv.historyURL = source, entry.URL
		if command == driver.WebReload {
			wv.historyKind = webNavigationReload
			if hr := wv.web.call(webReload); failed(hr) {
				wv.historySource = nil
				wv.report(driver.Event{Kind: driver.EventError, Text: hresultError("nsgo: the web view could not reload", hr).Error()})
			}
			return
		}
		wv.historyKind = webNavigationHistory
		// Select the entry we just verified, even if a page changes history while the asynchronous command is in flight.
		wv.devTools("Page.navigateToHistoryEntry", `{"entryId":`+strconv.Itoa(entry.ID)+`}`, func(_ string, err error) {
			if err != nil {
				wv.historySource = nil
				wv.report(driver.Event{Kind: driver.EventError, Text: err.Error()})
			}
		})
	})
}
