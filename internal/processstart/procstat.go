package processstart

import (
	"bytes"
	"strconv"
)

func procStartTicks(stat []byte) (int64, bool) {
	// comm can contain whitespace, newlines and parentheses. Only the
	// final ')' delimits it; after it, state is field 3, starttime is 22.
	open, close := bytes.IndexByte(stat, '('), bytes.LastIndexByte(stat, ')')
	if open < 2 || stat[open-1] != ' ' || close <= open || close+1 >= len(stat) || stat[close+1] != ' ' {
		return 0, false
	}
	field := 3
	for part := range bytes.FieldsSeq(stat[close+1:]) {
		if field == 22 {
			ticks, err := strconv.ParseInt(string(part), 10, 64)
			return ticks, err == nil && ticks >= 0
		}
		field++
	}
	return 0, false
}
