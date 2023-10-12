// Package that helps with string processing.
package relative

import (
	"strings"
)

// Takes a path with a /v1/db/<path> and removes
// the /v1/db/.
func GetRelativePathNonDB(path string) string {
	splitpath := strings.SplitAfterN(path, "/", 4)
	return "/" + splitpath[3]
}

// Takes a path with a /v1/db and removes
// the /v1.
func GetRelativePathDB(path string) string {
	trimmedpath := strings.TrimPrefix(path, "/v1")
	return trimmedpath
}
