//go:build android

package android

import (
	"slices"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// withoutWebCookies hands the cookie requests of surface id's batch b to the runtime's CookieManager, which every web view of the app shares, Incognito ones included, as msgCookies, and returns b without them. Java answers each with msgCookiesSet once it set and flushed its cookies, carrying the surface and the request's count back.
func (s *surface) withoutWebCookies(id uint32, b *driver.MountBatch) *driver.MountBatch {
	cookies := func(it driver.MountItem) bool { return it.Kind == driver.ItemWebCookies }
	if !slices.ContainsFunc(b.Items, cookies) {
		return b
	}
	for _, it := range b.Items {
		if cookies(it) {
			s.send(msgCookies, func(w *writer) { writeWebCookies(w, id, it.Count, it.Cookies.Cookies) })
		}
	}
	out := *b
	out.Items = slices.DeleteFunc(slices.Clone(b.Items), cookies)
	return &out
}

// writeWebCookies writes a msgCookies as NsgoView.webCookies reads it: the surface and the request's count, then each cookie as a URL it goes to and its Set-Cookie value, which CookieManager.setCookie takes.
func writeWebCookies(w *writer, surface, count uint32, cookies []driver.WebCookie) {
	w.u32(surface)
	w.u32(count)
	w.u32(uint32(len(cookies)))
	for _, c := range cookies {
		w.str(c.URL())
		w.str(c.SetCookie())
	}
}

// webCookiesSet dispatches a msgCookiesSet to the surface whose request it answers.
func (s *surface) webCookiesSet(r *reader) {
	surface, count := r.u32(), r.u32()
	s.dispatchTo(surface, driver.Event{Kind: driver.EventWebCookies, Count: count, Text: r.str()})
}
