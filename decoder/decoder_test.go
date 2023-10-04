package decoder

import "testing"

type test struct {
	instr  string
	outstr string
}

func TestPercentDecoder(t *testing.T) {
	data := []test{
		{"hello%20asd", "hello asd"},
		{"%21as%23ad", "!as#ad"},
		{"%28a%2fa%23ad", "(a/a#ad"},
		{"%28a%2Fa%23ad", "(a/a#ad"},
	}

	for _, d := range data {
		result, _ := PercentDecoding(d.instr)
		if result != d.outstr {
			t.Errorf("Expected %s, got %s", d.outstr, result)
		}
	}
}

func TestGetRelativePath(t *testing.T) {
	path1 := "/v1/db/document1/col1"
	path2 := "/v1/db2/doc"

	if GetRelativePath(path1) != "document1/col1" {
		t.Errorf("Expected document1/col1, but got %s", GetRelativePath(path1))
	}

	if GetRelativePath(path2) != "doc" {
		t.Errorf("Expected doc, but got %s", GetRelativePath(path2))
	}
}
