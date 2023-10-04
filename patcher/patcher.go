// Package patcher provides a struct to marshal
// patches and a method to apply an input patch
// to an input document.
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
	Value interface{}
}

// A struct that visits a document and applies a patch to it.
type patchVisitor struct {
	patch      Patch
	currPath   string
	patchedDoc map[string]interface{}
}

// Creates a new patch visitor struct for this patch.
func new(patch Patch) patchVisitor {
	vis := patchVisitor{}
	vis.patch = patch
	vis.currPath = strings.TrimPrefix(patch.Path, "/")
	return vis
}

// Applys the provided patch to the provided document.
func ApplyPatch(doc map[string]interface{}, patch Patch) (map[string]interface{}, bool, error) {
	patcher := new(patch)
	patcher.patchedDoc = doc
	succ, err := jsonvisit.Accept(doc, &patcher)
	return patcher.patchedDoc, succ, err
}

// Handles visiting a JSON object with this patch struct.
func (p *patchVisitor) Map(m map[string]any) (bool, error) {
	// Case where we have not reached the final key yet
	if p.currPath != "" {
		// Process the string
		splitpath := strings.SplitAfterN(p.currPath, "/", 2)

		// Top level key
		targetKey := splitpath[0]

		// Store rest of path in the patch object, empty tells us we're in target location.
		if len(splitpath) == 1 && p.patch.Op == "ObjectAdd" {
			// Check if key is in there or not
			for key := range m {
				if key == targetKey {
					return true, nil
				}
			}

			// If key is not in there, we are now confident that we have all the steps in the
			// path and can add the object.
			// Find the proper dictionary to add to.
			prepath := strings.TrimPrefix(strings.TrimSuffix(p.patch.Path, p.currPath), "/")
			cutpath := strings.Split(prepath, "/")

			dict := p.patchedDoc
			for _, key := range cutpath[:len(cutpath)-1] {
				dict = dict[key].(map[string]interface{})
			}

			// Once in target location, add item
			dict[targetKey] = p.patch.Value
			return true, nil
		} else if len(splitpath) == 1 {
			// Should be ArrayAdd or ArrayRemove - have to find array at target key.
			p.currPath = ""
		} else {
			// Keep going deeper into list
			p.currPath = splitpath[1]
		}

		// Iterate over keys and only go deeper on target one.
		ok := false
		for key, val := range m {
			if key == targetKey {
				ok, err := jsonvisit.Accept(val, p)
				if !ok {
					return ok, err
				}
				break
			}
		}

		if !ok {
			// If we get here, patch has invalid path.
			msg := fmt.Sprintf("Missing key \"%s\" in path", targetKey)
			return false, errors.New(msg)
		} else {
			return true, nil
		}
	} else {
		msg := fmt.Sprintf("Expected target to be a slice.")
		return false, errors.New(msg)
	}
}

// Handles visiting a slice with this patch.
func (p *patchVisitor) Slice(s []any) (bool, error) {
	if p.currPath != "" {
		// Case where we find a slice before we expect it.
		return false, errors.New("Reached array before end of path")
	} else if p.patch.Op == "ArrayAdd" {
		// Check if key is in there or not
		for _, val := range s {
			if jsonvisit.Equal(val, p.patch.Value) {
				return true, nil
			}
		}

		newslice := append(s, p.patch.Value)

		// If key is not in there, we are now confident that we have all the steps in the
		// path and can add the object.
		// Find the proper dictionary to add to.
		prepath := strings.TrimPrefix(strings.TrimSuffix(p.patch.Path, p.currPath), "/")
		cutpath := strings.Split(prepath, "/")

		dict := p.patchedDoc
		for _, key := range cutpath[:len(cutpath)-1] {
			dict = dict[key].(map[string]interface{})
		}

		dict[cutpath[len(cutpath)-1]] = newslice

		return true, nil
	} else if p.patch.Op == "ArrayRemove" {
		// Find the proper dictionary to add to.
		prepath := strings.TrimPrefix(strings.TrimSuffix(p.patch.Path, p.currPath), "/")
		cutpath := strings.Split(prepath, "/")

		dict := p.patchedDoc
		for _, key := range cutpath[:len(cutpath)-1] {
			dict = dict[key].(map[string]interface{})
		}

		// Handle array remove.
		for i, val := range s {
			// Not sure if this is what we want to do.
			if jsonvisit.Equal(val, p.patch.Value) {
				newslice := append(s[:i], s[i+1:])
				dict[cutpath[len(cutpath)-1]] = newslice
				return true, nil
			}
		}
		// Object wasn't in, but we still return true.
		return true, nil
	} else {
		return false, errors.New("Invalid Operation")
	}
}

// Handles visiting a bool with this patch.
func (p *patchVisitor) Bool(b bool) (bool, error) {
	return false, errors.New("Path does not point to an object or slice")
}

// Handles visiting a float with this patch.
func (p *patchVisitor) Float64(f float64) (bool, error) {
	return false, errors.New("Path does not point to an object or slice")
}

// Handles visiting a string with this patch.
func (p *patchVisitor) String(string) (bool, error) {
	return false, errors.New("Path does not point to an object or slice")
}

// Handles visiting a null object with this patch.
func (p *patchVisitor) Null() (bool, error) {
	return false, errors.New("Path does not point to an object or slice")
}
