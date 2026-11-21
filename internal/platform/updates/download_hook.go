//go:build !nsgo_steam

package updates

import "github.com/nsgo-dev/nsgo/internal/driver"

func init() { driver.UpdateDownload = Download }
