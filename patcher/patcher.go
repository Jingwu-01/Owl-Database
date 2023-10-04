// Package patcher provides a struct to marshal
// patches and a method to apply an input patch
// to an input document.
package patcher

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/jsonvisit"
)

// A struct that represents a patch.
type Patch struct {
	Op    string
	Path  string
	Value interface{}
}

// A struct that visits a document and applies a patch to it.
type patchVisitor struct {
	patch    Patch
	currPath string
}

// Creates a new patch visitor struct for this patch.
func new(patch Patch) patchVisitor {
	vis := patchVisitor{}
	vis.patch = patch
	vis.currPath = strings.TrimPrefix(patch.Path, "/")
	return vis
}

func ApplyPatch(doc interface{}, patch Patch) (interface{}, error) {
	patcher := new(patch)
	patchedDoc, err := jsonvisit.Accept(doc, &patcher)
	return patchedDoc, err
}

// Handles visiting a JSON object with this patch struct.
func (p *patchVisitor) Map(m map[string]any) (any, error) {
	retval := make(map[string]any)

	// Process the string
	splitpath := strings.SplitAfterN(p.currPath, "/", 2)

	// Top level key
	targetKey := splitpath[0]

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

			retval[key] = updated
			found = true
		}
		retval[key] = val
	}

	if !found {
		// If we get here, patch has invalid path.
		msg := fmt.Sprintf("Missing key \"%s\" in path", targetKey)
		return false, errors.New(msg)
	} else {
		return retval, nil
	}

}

// Handles visiting a slice with this patch.
func (p *patchVisitor) Slice(s []any) (any, error) {
	if p.patch.Op == "ArrayAdd" && p.currPath == "" {
		// Not sure if this is what we want to do.
		arr := append(s, p.patch.Value)
		return arr, nil
	} else if p.patch.Op == "ArrayRemove" && p.currPath == "" {
		// Handle array remove.
		for i, val := range s {
			// Not sure if this is what we want to do.
			if jsonvisit.Equal(val, p.patch.Value) {
				arr := append(s[:i], s[i+1:])
				return arr, nil
			}
		}
		return s, nil
	} else if p.currPath == "" {
		return nil, errors.New("Attempted method which was not ArrayAdd or ArrayRemove")
	} else {
		retval := make([]any, 0)

		// Process the string
		splitpath := strings.SplitAfterN(p.currPath, "/", 2)

		// Top level key
		targetKey := splitpath[0]

		// Convert to an index
		targetIDX, err := strconv.Atoi(targetKey)

		if err != nil {
			return retval, errors.New("Attempted to index an array with a non-integer")
		}
		if targetIDX > len(s) {
			return retval, errors.New("Array out of bounds errors")
		}

		if len(splitpath) != 1 {
			// Update curr path for other cases
			p.currPath = splitpath[1]
		} else {
			p.currPath = ""
		}

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
	return nil, errors.New("Path includes a boolean")
}

// Handles visiting a float with this patch.
func (p *patchVisitor) Float64(f float64) (any, error) {
	return nil, errors.New("Path includes a Float64")
}

// Handles visiting a string with this patch.
func (p *patchVisitor) String(string) (any, error) {
	return nil, errors.New("Path includes a String")
}

// Handles visiting a null object with this patch.
func (p *patchVisitor) Null() (any, error) {
	return nil, errors.New("Path includes a Null")
}
