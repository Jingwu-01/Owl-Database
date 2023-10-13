package dbhandler

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

type test struct {
	r        *http.Request
	w        *httptest.ResponseRecorder
	expected string
	code     int
}

type skeletonAuthenticator struct {
}

func (skeletonAuthenticator) ValidateToken(w http.ResponseWriter, r *http.Request) (bool, string) {
	return true, "charlie"
}

func TestServeHTTPSequential(t *testing.T) {
	// Compile the schema
	testschema, _ := jsonschema.Compile("testschema.json")

	testhandler := New(false, testschema, skeletonAuthenticator{})

	// Tests Put and Get on dbs and docs
	data := []test{
		{httptest.NewRequest(http.MethodPut, "/db1", nil),
			httptest.NewRecorder(),
			"", 400},
		{httptest.NewRequest(http.MethodPut, "/v1/db1", nil),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1\"}", 201},
		{httptest.NewRequest(http.MethodGet, "/v1/db1/", nil),
			httptest.NewRecorder(),
			"[]", 200},
		{httptest.NewRequest(http.MethodPut, "/v1/db1/doc1", strings.NewReader("{\"prop\":100}")),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1/doc1\"}", 201},
		{httptest.NewRequest(http.MethodPut, "/v1/db1/doc1", strings.NewReader("{\"prop\":100}")),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1/doc1\"}", 200},
		// get document: document not found
		{httptest.NewRequest(http.MethodGet, "/v1/db1/doc2", nil),
			httptest.NewRecorder(),
			"", 404},
		// get database: database not found
		{httptest.NewRequest(http.MethodGet, "/v1/db2/", nil),
			httptest.NewRecorder(),
			"", 404},
		// get database or database: bad request
		{httptest.NewRequest(http.MethodGet, "/invalidPath", nil),
			httptest.NewRecorder(),
			"", 400},
		// put document or database: bad request
		{httptest.NewRequest(http.MethodPut, "/invalidPath", nil),
			httptest.NewRecorder(),
			"", 400},
	}

	i := 0
	for _, d := range data {
		testhandler.ServeHTTP(d.w, d.r)
		res := d.w.Result()
		defer res.Body.Close()
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Errorf("Test %d: Expected no error, got %v", i, err)
		}
		if string(data) != d.expected && d.expected != "" {
			t.Errorf("Test %d: Expected response %s got %s", i, d.expected, string(data))
		}
		if res.StatusCode != d.code {
			t.Errorf("Test %d: Expected response code %d got %d", i, d.code, res.StatusCode)
		}
		i++
	}

	// Have to get specific values to check documents for equality
	db1, _ := testhandler.databases.CollectionFind("db1")
	doc1, _ := db1.DocumentFind("doc1")
	doc1str := fmt.Sprintf("{\"path\":\"/doc1\",\"doc\":{\"prop\":100},\"meta\":{\"createdBy\":\"%s\",\"createdAt\":%d,\"lastModifiedBy\":\"%s\",\"lastModifiedAt\":%d}}",
		doc1.Output.Meta.CreatedBy, doc1.Output.Meta.CreatedAt, doc1.Output.Meta.LastModifiedBy, doc1.Output.Meta.LastModifiedAt)

	// Tests Get on DB and Doc
	data = []test{
		{httptest.NewRequest(http.MethodGet, "/v1/db1/doc1", nil),
			httptest.NewRecorder(),
			doc1str, 200},
		{httptest.NewRequest(http.MethodGet, "/v1/db1/", nil),
			httptest.NewRecorder(),
			"[" + doc1str + "]", 200},
	}

	for _, d := range data {
		testhandler.ServeHTTP(d.w, d.r)
		res := d.w.Result()
		defer res.Body.Close()
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Errorf("Test %d: Expected no error, got %v", i, err)
		}
		if string(data) != d.expected {
			t.Errorf("Test %d: Expected response %s got %s", i, d.expected, string(data))
		}
		if res.StatusCode != d.code {
			t.Errorf("Test %d: Expected response code %d got %d", i, d.code, res.StatusCode)
		}
		i++
	}

	// Tests Put for Collection and Post.
	data = []test{
		{httptest.NewRequest(http.MethodPut, "/v1/db1/doc1/col/", nil),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1/doc1/col/\"}", 201},
		{httptest.NewRequest(http.MethodPost, "/v1/db1/", strings.NewReader("{\"prop\":100}")),
			httptest.NewRecorder(),
			"", 201},
		{httptest.NewRequest(http.MethodPost, "/v1/db1/doc1/col/", strings.NewReader("{\"prop\":100}")),
			httptest.NewRecorder(),
			"", 201},
	}

	for _, d := range data {
		testhandler.ServeHTTP(d.w, d.r)
		res := d.w.Result()
		defer res.Body.Close()
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Errorf("Test %d: Expected no error, got %v", i, err)
		}
		if string(data) != d.expected && d.expected != "" {
			t.Errorf("Test %d: Expected response %s got %s", i, d.expected, string(data))
		}
		if res.StatusCode != d.code {
			t.Errorf("Test %d: Expected response code %d got %d", i, d.code, res.StatusCode)
		}
		i++
	}

	// Test that patch returns correct codes - modification is tested in patcher.
	data = []test{
		{httptest.NewRequest(http.MethodPatch, "/v1/db1/doc1", strings.NewReader("[{\"op\":\"ObjectAdd\",\"path\":\"/a\",\"value\":100}]")),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1/doc1\",\"patchFailed\":false,\"message\":\"patches applied\"}", 200},
		{httptest.NewRequest(http.MethodPatch, "/v1/db1/doc1", strings.NewReader("[{\"op\":\"ObjectAdd\",\"path\":\"/b\",\"value\":100},{\"op\":\"ObjectAdd\",\"path\":\"/c\",\"value\":100}]")),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1/doc1\",\"patchFailed\":false,\"message\":\"patches applied\"}", 200},
		{httptest.NewRequest(http.MethodPatch, "/v1/db1/doc1", strings.NewReader("[{\"op\":\"ArrayAdd\",\"path\":\"/b\",\"value\":100},{\"op\":\"ObjectAdd\",\"path\":\"/c\",\"value\":100}]")),
			httptest.NewRecorder(),
			"", 400},
	}

	for _, d := range data {
		testhandler.ServeHTTP(d.w, d.r)
		res := d.w.Result()
		defer res.Body.Close()
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Errorf("Test %d: Expected no error, got %v", i, err)
		}
		if string(data) != d.expected && d.expected != "" {
			t.Errorf("Test %d: Expected response %s got %s", i, d.expected, string(data))
		}
		if res.StatusCode != d.code {
			t.Errorf("Test %d: Expected response code %d got %d", i, d.code, res.StatusCode)
		}
		i++
	}

	// Tests delete on collections, docs, and dbs.
	data = []test{
		{httptest.NewRequest(http.MethodDelete, "/v1/db1/doc1/col/", nil),
			httptest.NewRecorder(),
			"", 204},
		{httptest.NewRequest(http.MethodDelete, "/v1/db1/doc1/col/", nil),
			httptest.NewRecorder(),
			"", 404},
		{httptest.NewRequest(http.MethodDelete, "/v1/db1/doc1", nil),
			httptest.NewRecorder(),
			"", 204},
		{httptest.NewRequest(http.MethodDelete, "/v1/db1/doc1", nil),
			httptest.NewRecorder(),
			"", 404},
		{httptest.NewRequest(http.MethodDelete, "/v1/db1", nil),
			httptest.NewRecorder(),
			"", 204},
		{httptest.NewRequest(http.MethodDelete, "/v1/db1", nil),
			httptest.NewRecorder(),
			"", 404},
	}

	for _, d := range data {
		testhandler.ServeHTTP(d.w, d.r)
		res := d.w.Result()
		defer res.Body.Close()

		if res.StatusCode != d.code {
			t.Errorf("Test %d: Expected response code %d got %d", i, d.code, res.StatusCode)
		}
		i++
	}

}
