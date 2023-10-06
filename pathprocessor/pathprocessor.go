// Contains static utility string methods for processing paths
package pathprocessor

import (
	"strings"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/skiplist"
)

func GetInterval(intervalStr string) [2]string {
	interval := [2]string{skiplist.STRING_MIN, skiplist.STRING_MAX}

	// Must be in array form
	if !(len(intervalStr) > 2 && intervalStr[0] == '[' && intervalStr[1] == ']') {
		return interval
	}

	// Get rid of array surrounders and split
	intervalStr = intervalStr[1 : len(intervalStr)-1]
	procArr := strings.Split(intervalStr, ",")

	if len(procArr) > 2 {
		// Too many args
		return interval
	}

	// Success
	interval[0] = procArr[0]
	if len(procArr) == 2 {
		interval[1] = procArr[1]
	}
	return interval
}
