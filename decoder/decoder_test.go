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
	}

	for _, d := range data {
		result, _ := PercentDecoding(d.instr)
		if result != d.outstr {
			t.Errorf("Expected %s, got %s", d.outstr, result)
		}
	}
}
