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
	patch      Patch
	currPath   string
	patchedDoc map[string]interface{}
}

// Creates a new patch visitor struct for this patch.
func new(patch Patch) PatchVisitor {
	vis := PatchVisitor{}
	vis.patch = patch
	vis.currPath = strings.TrimPrefix(patch.Path, "/")
	return vis
}

// Applys the provided patch to the provided document.
func ApplyPatch(doc map[string]interface{}, patch Patch) map[string]interface{} {
	patcher := new(patch)
	jsonvisit.Accept(doc, &patcher)
	return patcher.patchedDoc
}

// Handles visiting a JSON object with this patch struct.
func (p *PatchVisitor) Map(m map[string]any) (bool, error) {
	// Case where we have not reached the final key yet
	if p.currPath != "" {
		// Process the string
		splitpath := strings.SplitAfterN(p.currPath, "/", 2)

		// Top level key
		targetKey := splitpath[0]

		// Store rest of path in the patch object, empty tells us we're in target location.
		if len(splitpath) == 1 {
			p.currPath = ""
		} else {
			p.currPath = splitpath[1]
		}

		// Find the proper dictionary to add to.
		prepath := strings.TrimPrefix(strings.TrimSuffix(p.patch.Path, p.currPath), "/")
		cutpath := strings.Split(prepath, "/")

		dict := p.patchedDoc
		for _, key := range cutpath {
			val := dict[key]
			dict = val.(map[string]interface{})
		}

		// Iterate over keys and only go deeper on target one.
		ok := false
		for key, val := range m {
			if key == targetKey {
				ok, err := jsonvisit.Accept(val, p)
				if !ok {
					return ok, err
				}
			} else {
				dict[key] = val
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
		// We've reached the target object.
		if p.patch.Op != "ObjectAdd" {
			// Attempting an invalid operation on this object
			return false, errors.New("Invalid operation")
		} else {
			// Find the proper dictionary to add to.
			prepath := strings.TrimPrefix(p.patch.Path, "/")
			cutpath := strings.Split(prepath, "/")

			dict := p.patchedDoc
			for _, key := range cutpath {
				val := dict[key]
				dict = val.(map[string]interface{})
			}

			// Copy keys and vals into the document.
			for key, val := range m {
				dict[key] = val
			}

			// Copy patch into the document.
			for key, val := range p.patch.Value {
				dict[key] = val
			}

			return true, nil
		}
	}
}

// Handles visiting a slice with this patch.
func (p *PatchVisitor) Slice(s []any) (bool, error) {
	if p.currPath != "" {
		// Case where we find a slice before we expect it.
		return false, errors.New("Reached array before end of path")
	} else if p.patch.Op == "ArrayAdd" {
		// Find the proper dictionary to add to.
		prepath := strings.TrimPrefix(p.patch.Path, "/")
		cutpath := strings.Split(prepath, "/")

		dict := p.patchedDoc
		for _, key := range cutpath[:len(cutpath)-1] {
			val := dict[key]
			dict = val.(map[string]interface{})
		}

		// Not sure if this is what we want to do.
		s = append(s, p.patch.Value)

		dict[cutpath[len(cutpath)-1]] = s

		return true, nil
	} else if p.patch.Op == "ArrayRemove" {
		// Handle array remove.

		// Find the proper dictionary to add to.
		prepath := strings.TrimPrefix(p.patch.Path, "/")
		cutpath := strings.Split(prepath, "/")

		dict := p.patchedDoc
		for _, key := range cutpath[:len(cutpath)-1] {
			val := dict[key]
			dict = val.(map[string]interface{})
		}

		for i, val := range s {
			// Not sure if this is what we want to do.
			if jsonvisit.Equal(val, p.patch.Value) {
				s = append(s[:i], s[i+1:])
				dict[cutpath[len(cutpath)-1]] = s
				return true, nil
			}
		}
		return false, errors.New("Element to remove not in the array")
	} else {
		return false, errors.New("Invalid Operation")
	}
}

// Handles visiting a bool with this patch.
func (p *PatchVisitor) Bool(b bool) (bool, error) {
	return false, errors.New("Path does not point to an object")
}

// Handles visiting a float with this patch.
func (p *PatchVisitor) Float64(f float64) (bool, error) {
	return false, errors.New("Path does not point to an object")
}

// Handles visiting a string with this patch.
func (p *PatchVisitor) String(string) (bool, error) {
	return false, errors.New("Path does not point to an object")
}

// Handles visiting a null object with this patch.
func (p *PatchVisitor) Null() (bool, error) {
	return false, errors.New("Path does not point to an object")
}
