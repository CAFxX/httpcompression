//go:build cgo

package gozstd_test

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/CAFxX/httpcompression"
	"github.com/CAFxX/httpcompression/contrib/valyala/gozstd"
	vzstd "github.com/valyala/gozstd"
)

var _ httpcompression.CompressorProvider = &gozstd.Compressor{}

func TestZstd(t *testing.T) {
	t.Parallel()

	s := []byte("hello world!")

	c, err := gozstd.New(vzstd.WriterParams{})
	if err != nil {
		t.Fatal(err)
	}
	b := &bytes.Buffer{}
	w := c.Get(b)
	w.Write(s)
	w.Close()

	r := vzstd.NewReader(b)
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
