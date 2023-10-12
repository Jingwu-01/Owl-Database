package relative

import "testing"

type test struct {
	instr  string
	outstr string
}

func TestGetRelativePathNonDB(t *testing.T) {
	path1 := "/v1/db/document1/col1"
	path2 := "/v1/db2/doc"

	if GetRelativePathNonDB(path1) != "/document1/col1" {
		t.Errorf("Expected document1/col1, but got %s", GetRelativePathNonDB(path1))
	}

	if GetRelativePathNonDB(path2) != "/doc" {
		t.Errorf("Expected doc, but got %s", GetRelativePathNonDB(path2))
	}
}

func TestGetRelativePathDB(t *testing.T) {
	path1 := "/v1/db"
	path2 := "/v1/db2"

	if GetRelativePathDB(path1) != "/db" {
		t.Errorf("Expected document1/col1, but got %s", GetRelativePathDB(path1))
	}

	if GetRelativePathDB(path2) != "/db2" {
		t.Errorf("Expected doc, but got %s", GetRelativePathDB(path2))
	}
}
