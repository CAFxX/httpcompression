package gzip_test

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/CAFxX/httpcompression"
	"github.com/CAFxX/httpcompression/contrib/compress/gzip"

	kpgzip "github.com/klauspost/compress/gzip"
)

var _ httpcompression.CompressorProvider = &gzip.Compressor{}

func TestGzip(t *testing.T) {
	t.Parallel()

	s := []byte("hello world!")

	c, err := gzip.New(gzip.Options{})
	if err != nil {
		t.Fatal(err)
	}
	b := &bytes.Buffer{}
	w := c.Get(b)
	w.Write(s)
	w.Close()

	r, err := kpgzip.NewReader(b)
	if err != nil {
		t.Fatal(err)
	}
	d, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(s, d) {
		t.Fatalf("decoded string mismatch\ngot: %q\nexp: %q", string(s), string(d))
	}
}
