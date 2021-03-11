package bloomfilter

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
)

func readTestFilter(t testing.TB, dirname, filename string) []byte {
	content, err := ioutil.ReadFile(fmt.Sprintf("./testdata/filters/%s/%s", dirname, filename))
	if err != nil {
		t.Fatalf("unexpected error reading: %s", err)
	}

	raw, err := hex.DecodeString(strings.TrimSpace(string(content)))
	if err != nil {
		t.Fatalf("unexpected error decoding: %s", err)
	}

	return raw
}

func readTestWords(t testing.TB, filename string) []string {
	content, err := ioutil.ReadFile(fmt.Sprintf("./testdata/words/%s", filename))
	if err != nil {
		t.Fatalf("unexpected error reading %s: %s", filename, err)
	}

	return strings.Split(strings.TrimSpace(string(content)), "\n")
}
