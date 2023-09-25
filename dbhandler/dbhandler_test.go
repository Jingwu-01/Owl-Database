package dbhandler

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RICE-COMP318-FALL23/owldb-p1group20/collection"
	"github.com/RICE-COMP318-FALL23/owldb-p1group20/document"
)

type test struct {
	r        *http.Request
	w        *httptest.ResponseRecorder
	expected string
	code     int
}

func TestServeHTTP(t *testing.T) {
	testhandler := New(false, nil)

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
			t.Errorf("Test %d: Expected error code %d got %d", i, d.code, res.StatusCode)
		}
		i++
	}

	// Have to get specific values to check documents for equality
	db1, _ := testhandler.databases.Load("db1")
	doc1, _ := db1.(collection.Collection).Documents.Load("doc1")
	doc1str := fmt.Sprintf("{\"path\":\"/doc1\",\"doc\":{\"prop\":100},\"meta\":{\"createdBy\":\"%s\",\"createdAt\":%d,\"lastModifiedBy\":\"%s\",\"lastModifiedAt\":%d}}",
		doc1.(document.Document).Output.Meta.CreatedBy, doc1.(document.Document).Output.Meta.CreatedAt, doc1.(document.Document).Output.Meta.LastModifiedBy, doc1.(document.Document).Output.Meta.LastModifiedAt)
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
			t.Errorf("Test %d: Expected error code %d got %d", i, d.code, res.StatusCode)
		}
		i++
	}
}
