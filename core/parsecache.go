package core

// parseCache keeps two generations of parsed values on one graph. An old value used again is promoted, and a full generation replaces the old one, as measureCache does. Callers use it on the graph's goroutine.
type parseCache[K comparable, V any] struct {
	now, old map[K]V
}

const parseCacheSize = 1024

func (c *parseCache[K, V]) get(key K) (V, bool) {
	if value, ok := c.now[key]; ok {
		return value, true
	}
	value, ok := c.old[key]
	if ok {
		c.put(key, value)
	}
	return value, ok
}

func (c *parseCache[K, V]) put(key K, value V) {
	if c.now == nil {
		c.now = make(map[K]V)
	}
	if len(c.now) >= parseCacheSize {
		c.old, c.now = c.now, c.old
		if c.now == nil {
			c.now = make(map[K]V)
		} else {
			clear(c.now)
		}
	}
	c.now[key] = value
}
