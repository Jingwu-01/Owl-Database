// Package patcher provides a struct to marshal
// patches and a method to apply an input patch
// to an input document.
package patcher

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/jsonvisit"
)

// A struct that represents a patch.
type Patch struct {
	Op    string      // The desired operation of the patch.
	Path  string      // A JSON pointer to the target of the patch.
	Value interface{} // The JSON object to be added or removed by the patch.
}

// A struct that visits a document and applies a patch to it.
type patchVisitor struct {
	patch    Patch  // The patch to be added to the visited document.
	currPath string // The remaining part of the path to traverse.
}

// Creates a new patch visitor struct for this patch.
func new(patch Patch) (patchVisitor, error) {
	vis := patchVisitor{}
	vis.patch = patch
	currPath, found := strings.CutPrefix(patch.Path, "/")

	if !found {
		slog.Info("User attempted patch without leading slash", "path", patch.Path)
		return vis, errors.New("Patch path missing leading slash")
	}

	vis.currPath = currPath

	return vis, nil
}

// Applies the input patch to the input document, and returns
// the patched document, or an error, if one occurs.
func ApplyPatch(doc interface{}, patch Patch) (interface{}, error) {
	patcher, err := new(patch)
	if err != nil {
		return nil, err
	}
	patchedDoc, err := jsonvisit.Accept(doc, &patcher)
	return patchedDoc, err
}

// Handles visiting a JSON object with this patch struct.
func (p *patchVisitor) Map(m map[string]any) (any, error) {
	slog.Debug("Patcher called map", "map", m)
	retval := make(map[string]any)

	// Process the string
	splitpath := strings.SplitAfterN(p.currPath, "/", 2)

	// Top level key
	targetKey := strings.TrimSuffix(splitpath[0], "/")

	// Store rest of path in the patch object, empty tells us we're in target location.
	if len(splitpath) == 1 && p.patch.Op == "ObjectAdd" {
		// Check if target key is in the map.
		for key, val := range m {
			if key == targetKey {
				return m, nil
			}
			retval[key] = val
		}
		// If target key not in the map, add it to object.
		retval[targetKey] = p.patch.Value
		slog.Debug("Patcher returning map", "retval", retval)
		return retval, nil
	} else if len(splitpath) != 1 {
		// Update curr path for other cases
		p.currPath = splitpath[1]
	} else {
		p.currPath = ""
	}

	found := false
	for key, val := range m {
		if key == targetKey {
			updated, err := jsonvisit.Accept(val, p)

			if err != nil {
				// Forward error along
				return updated, err
			}

			slog.Debug("Setting updated val", "key", key, "val", updated)
			retval[key] = updated
			found = true
		} else {
			retval[key] = val
		}
	}

	if !found {
		// If we get here, patch has invalid path.
		msg := fmt.Sprintf("Missing key \"%s\" in path", targetKey)
		return retval, errors.New(msg)
	} else {
		slog.Debug("Patcher returning map", "retval", retval)
		return retval, nil
	}

}

// Handles visiting a slice with this patch.
func (p *patchVisitor) Slice(s []any) (any, error) {
	slog.Debug("Patcher called Slice", "slice", s)
	if p.patch.Op == "ArrayAdd" && p.currPath == "" {
		// Not sure if this is what we want to do.
		arr := append(s, p.patch.Value)
		return arr, nil
	} else if p.patch.Op == "ArrayRemove" && p.currPath == "" {
		// Handle array remove.
		for i, val := range s {
			// Not sure if this is what we want to do.
			if jsonvisit.Equal(val, p.patch.Value) {
				arr := append(s[:i], s[i+1:]...)
				return arr, nil
			}
		}
		return s, nil
	} else if p.currPath == "" {
		return nil, errors.New("attempted method which was not ArrayAdd or ArrayRemove")
	} else {
		retval := make([]any, 0)

		// Process the string
		splitpath := strings.SplitAfterN(p.currPath, "/", 2)

		// Top level key
		targetKey := strings.TrimSuffix(splitpath[0], "/")

		// Convert to an index
		targetIDX, err := strconv.Atoi(targetKey)

		// Error cases
		if err != nil {
			return retval, errors.New("attempted to index an array with a non-integer")
		}
		if targetIDX > len(s) {
			return retval, errors.New("array out of bounds errors")
		}
		if len(splitpath) == 1 {
			return retval, errors.New("path ends with slice index")
		}

		p.currPath = splitpath[1]

		// Iterate over the slice and update target value
		for i, val := range s {
			if i == targetIDX {
				update, err := jsonvisit.Accept(val, p)

				if err != nil {
					// Forward error along
					return update, err
				}
				retval = append(retval, update)
			} else {
				retval = append(retval, val)
			}
		}

		return retval, nil
	}
}

// Handles visiting a bool with this patch.
func (p *patchVisitor) Bool(b bool) (any, error) {
	return nil, errors.New("path includes a boolean")
}

// Handles visiting a float with this patch.
func (p *patchVisitor) Float64(f float64) (any, error) {
	return nil, errors.New("path includes a Float64")
}

// Handles visiting a string with this patch.
func (p *patchVisitor) String(string) (any, error) {
	return nil, errors.New("path includes a String")
}

// Handles visiting a null object with this patch.
func (p *patchVisitor) Null() (any, error) {
	return nil, errors.New("path includes a Null")
}
