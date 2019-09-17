package internal

import (
	"os"
	"testing"
)

func TestContentType(t *testing.T) {
	file, err := os.Open("./testdata/file.txt")
	if err != nil {
		t.Logf("err opening file: %s", err)
		panic(err)
	}
	defer file.Close()
	contentType, err := ContentType(file)
	if err != nil {
		t.FailNow()
	}
	t.Log(contentType)
}
