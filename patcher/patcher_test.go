package patcher

import (
	"log/slog"
	"os"
	"testing"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/jsonvisit"
)

// Creates test documents for testing patch method.
func createTestDocs() (map[string]interface{}, interface{}) {

	// Create a document for testing
	doc := make(map[string]interface{})
	doc["a"] = 1
	doc["b"] = make(map[string]interface{})
	doc["b"].(map[string]interface{})["c"] = true
	doc["array"] = make([]interface{}, 0)
	map1 := make(map[string]interface{})
	map1["a"] = 2
	doc["array"] = append(doc["array"].([]interface{}), map1)
	doc["array"] = append(doc["array"].([]interface{}), 100)

	// Create a copy of it
	patchedDoc := make(map[string]interface{})
	patchedDoc["a"] = 1
	patchedDoc["b"] = make(map[string]interface{})
	patchedDoc["b"].(map[string]interface{})["c"] = true
	patchedDoc["array"] = make([]interface{}, 0)
	map2 := make(map[string]interface{})
	map2["a"] = 2
	patchedDoc["array"] = append(patchedDoc["array"].([]interface{}), map2)
	patchedDoc["array"] = append(patchedDoc["array"].([]interface{}), 100)

	return doc, patchedDoc
}

/*
 * General tests.
 */

// Tests that apply patch errors on a bad path
func TestApplyPatchBadPath(t *testing.T) {
	// Create docs for testing
	_, patchedDoc := createTestDocs()

	patch := Patch{"ObjectAdd", "/c/d", 100}

	_, err := ApplyPatch(patchedDoc, patch)

	if err == nil {
		t.Error("Expected error, did not get")
	}
}

// Tests that apply patch errors on a bad path
func TestApplyPatchPathMissingSlash(t *testing.T) {
	// Create docs for testing
	_, patchedDoc := createTestDocs()

	patch := Patch{"ObjectAdd", "c", 100}

	_, err := ApplyPatch(patchedDoc, patch)

	if err == nil {
		t.Error("Expected error, did not get")
	}
}

// Tests that apply patch does not modify input doc on error.
func TestApplyPatchNotModErr(t *testing.T) {
	// Create docs for testing
	doc, patchedDoc := createTestDocs()

	patch := Patch{"ObjectAdd", "/c/d", 100}

	_, err := ApplyPatch(patchedDoc, patch)

	if !jsonvisit.Equal(doc, patchedDoc) || err == nil {
		t.Error("Expected doc = patchedDoc, got", "doc", doc, "patchedDoc", patchedDoc)
	}
}

// Tests that apply patch does not modify input doc on success.
func TestApplyPatchNotModSucc(t *testing.T) {
	// Create docs for testing
	doc, patchedDoc := createTestDocs()

	patch := Patch{"ObjectAdd", "/c", 100}

	_, err := ApplyPatch(patchedDoc, patch)

	if !jsonvisit.Equal(doc, patchedDoc) || err != nil {
		t.Error("Expected doc = patchedDoc, got", "doc", doc, "patchedDoc", patchedDoc)
	}
}

/*
 * ObjectAdd
 */

// Tests a simple ObjAdd
func TestApplyPatchObjAdd(t *testing.T) {
	// Create docs for testing
	doc, patchedDoc := createTestDocs()

	patch := Patch{"ObjectAdd", "/c", 100}

	patchedDoc, _ = ApplyPatch(patchedDoc, patch)

	doc["c"] = 100

	if !jsonvisit.Equal(doc, patchedDoc) {
		t.Error("Expected doc = patchedDoc, got", "doc", doc, "patchedDoc", patchedDoc)
	}
}

// Tests that ObjAdd does not overwrite
func TestApplyPatchOverwrite(t *testing.T) {
	// Create docs for testing
	doc, patchedDoc := createTestDocs()

	patch := Patch{"ObjectAdd", "/a", 100}

	patchedDoc, _ = ApplyPatch(patchedDoc, patch)

	if !jsonvisit.Equal(doc, patchedDoc) {
		t.Error("Expected doc = patchedDoc, got", "doc", doc, "patchedDoc", patchedDoc)
	}
}

// Tests a nested ObjAdd
func TestApplyPatchNestedObjAdd(t *testing.T) {
	// Create docs for testing
	doc, patchedDoc := createTestDocs()

	patch := Patch{"ObjectAdd", "/b/a", 100}

	patchedDoc, _ = ApplyPatch(patchedDoc, patch)

	doc["b"].(map[string]interface{})["a"] = 100

	if !jsonvisit.Equal(doc, patchedDoc) {
		t.Error("Expected doc = patchedDoc, got", "doc", doc, "patchedDoc", patchedDoc)
	}
}

// Tests an ObjAdd to an obj in a slice
func TestApplyPatchSliceNestedObjAdd(t *testing.T) {
	// Create docs for testing
	doc, patchedDoc := createTestDocs()

	patch := Patch{"ObjectAdd", "/array/0/b", 100}

	patchedDoc, _ = ApplyPatch(patchedDoc, patch)

	doc["array"].([]interface{})[0].(map[string]interface{})["b"] = 100

	if !jsonvisit.Equal(doc, patchedDoc) {
		t.Error("Expected doc = patchedDoc, got", "doc", doc, "patchedDoc", patchedDoc)
	}
}

/*
 * ArrayAdd
 */

// Tests a simple ArrayAdd
func TestApplyPatchArrayAdd(t *testing.T) {
	// Create docs for testing
	doc, patchedDoc := createTestDocs()

	patch := Patch{"ArrayAdd", "/array", 100}

	patchedDoc, _ = ApplyPatch(patchedDoc, patch)

	doc["array"] = append(doc["array"].([]interface{}), 100)

	if !jsonvisit.Equal(doc, patchedDoc) {
		t.Error("Expected doc = patchedDoc, got", "doc", doc, "patchedDoc", patchedDoc)
	}
}

/*
 * ArrayRemove
 */

// Tests a simple ArrayRemove
func TestApplyPatchArrayRemove(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	// Create docs for testing
	doc, patchedDoc := createTestDocs()

	map1 := make(map[string]interface{})
	map1["a"] = 2
	patch := Patch{"ArrayRemove", "/array", map1}

	patchedDoc, _ = ApplyPatch(patchedDoc, patch)

	doc["array"] = make([]interface{}, 0)
	doc["array"] = append(doc["array"].([]interface{}), 100)

	if !jsonvisit.Equal(doc, patchedDoc) {
		t.Error("Expected doc = patchedDoc, got", "doc", doc, "patchedDoc", patchedDoc)
	}
}
