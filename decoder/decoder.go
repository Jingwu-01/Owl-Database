// Package that helps with string processing.
package decoder

import (
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
)

// Translates a string with percentages into the proper string.
func PercentDecoding(input string) (string, error) {
	// Finds the first index of a %
	substrs := strings.Split(input, "%")

	if len(substrs) == 1 {
		return input, nil
	} else {
		// Initialize i and retval
		i := 1
		retval := substrs[0]

		for i < len(substrs) {
			// Split substr[i] into characters
			chars := strings.Split(substrs[i], "")

			// Ensure we have 2 characters following a percentage.
			if len(chars) < 2 {
				slog.Error("Not enough characters following %", "number", len(chars))
				return "", errors.New("Not enough characters following %")
			}

			// Translate the characters into their ASCII representation
			trans, err := hex.DecodeString(chars[0] + chars[1])
			if err != nil {
				slog.Error("Error converting hex to string", "error", err, "str", chars[0]+chars[1])
				return "", errors.New("Error converting hex to string")
			}

			// Add the rest of the string to retval
			retval = retval + string(trans)
			j := 2
			for j < len(chars) {
				retval = retval + chars[j]
				j++
			}
			i++
		}
		return retval, nil
	}
}

// Takes a path with a /v1/db/<path> and removes
// the /v1/db/.
func GetRelativePath(path string) string {
	splitpath := strings.SplitAfterN(path, "/", 4)
	return splitpath[3]
}
