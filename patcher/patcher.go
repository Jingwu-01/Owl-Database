package patcher

import (
	"errors"
	"fmt"
	"strings"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/jsonvisit"
)

// A struct that represents a patch.
type Patch struct {
	Op    string
	Path  string
	Value map[string]interface{}
}

// A struct that visits a document and applies a patch to it.
type PatchVisitor struct {
	patch Patch
}

// Creates a new patch visitor struct for this patch.
func New(patches Patch) PatchVisitor {
	return PatchVisitor{patches}
}

// Handles visiting a JSON object with this patch struct.
func (p PatchVisitor) Map(m map[string]any) (bool, error) {
	// Case where we have not reached the final key yet
	if p.patch.Path != "" {
		// Process the string
		cutpath := strings.TrimPrefix(p.patch.Path, "/")
		splitpath := strings.SplitAfterN(cutpath, "/", 2)

		// Top level key
		targetKey := splitpath[0]

		// Store rest of path in the patch object, empty tells us we're in target location.
		if len(splitpath) == 1 {
			p.patch.Path = ""
		} else {
			p.patch.Path = splitpath[1]
		}

		// Iterate over keys and only go deeper on target one.
		for key, val := range m {
			if key == targetKey {
				return jsonvisit.Accept(val, p)
			}
		}

		// If we get here, patch has invalid path.
		msg := fmt.Sprintf("Missing key \"%s\" in path", targetKey)
		return false, errors.New(msg)
	} else {
		// We've reached the target object.
		if p.patch.Op != "ObjectAdd" {
			// Attempting an invalid operation on this object
			return false, errors.New("Invalid operation")
		} else {
			// Add the keys and vals to this object - not sure if right.
			for key, val := range p.patch.Value {
				m[key] = val
			}
			return true, nil
		}
	}
}

// Handles visiting a slice with this patch.
func (p PatchVisitor) Slice(s []any) (bool, error) {
	if p.patch.Path != "" {
		// Case where we find a slice before we expect it.
		return false, errors.New("Reached array before end of path")
	} else if p.patch.Op == "ArrayAdd" {
		// Not sure if this is what we want to do.
		s = append(s, p.patch.Value)
		return true, nil
	} else if p.patch.Op == "ArrayRemove" {
		// Handle array remove.
		for i, val := range s {
			// Not sure if this is what we want to do.
			if jsonvisit.Equal(val, p.patch.Value) {
				s = append(s[:i], s[i+1:])
				return true, nil
			}
		}
		return false, errors.New("Element to remove not in the array")
	} else {
		return false, errors.New("Invalid Operation")
	}
}

// Handles visiting a bool with this patch.
func (p PatchVisitor) Bool(b bool) (bool, error) {
	return false, errors.New("Path does not point to an object")
}

// Handles visiting a float with this patch.
func (p PatchVisitor) Float64(f float64) (bool, error) {
	return false, errors.New("Path does not point to an object")
}

// Handles visiting a string with this patch.
func (p PatchVisitor) String(string) (bool, error) {
	return false, errors.New("Path does not point to an object")
}

// Handles visiting a null object with this patch.
func (p PatchVisitor) Null() (bool, error) {
	return false, errors.New("Path does not point to an object")
}
