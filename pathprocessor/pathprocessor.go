// Contains static utility string methods for processing paths
package pathprocessor

import (
	"log/slog"
	"strings"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/skiplist"
)

func GetInterval(intervalStr string) [2]string {
	interval := [2]string{skiplist.STRING_MIN, skiplist.STRING_MAX}
	// Must be in array form
	if !(len(intervalStr) > 2 && intervalStr[0] == '[' && intervalStr[len(intervalStr)-1] == ']') {
		slog.Info("GetInterval: Bad interval, non-array", "interval", intervalStr)
		return interval
	}

	// Get rid of array surrounders and split
	intervalStr = intervalStr[1 : len(intervalStr)-1]
	procArr := strings.Split(intervalStr, ",")

	if len(procArr) != 2 {
		// Too many args
		slog.Info("GetInterval: Bad interval, incorrect args", "interval", intervalStr)
		return interval
	}

	// Success
	interval[0] = procArr[0]
	interval[1] = procArr[1]

	if interval[1] == "" {
		interval[1] = skiplist.STRING_MAX
	}

	slog.Info("GetInterval: Good interval", "arg[0]", interval[0], "arg[1]", interval[1])
	return interval
}
