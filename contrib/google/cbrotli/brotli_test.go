//go:build cgo

package cbrotli_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"testing"

	"github.com/CAFxX/httpcompression"
	"github.com/CAFxX/httpcompression/contrib/google/cbrotli"
	gcbrotli "github.com/google/brotli/go/cbrotli"
)

var _ httpcompression.CompressorProvider = &cbrotli.Compressor{}

func TestBrotli(t *testing.T) {
	t.Parallel()

	s := []byte("hello world!")

	c, err := cbrotli.New(gcbrotli.WriterOptions{})
	if err != nil {
		t.Fatal(err)
	}
	b := &bytes.Buffer{}
	w := c.Get(b)
	w.Write(s)
	w.Close()

	r := gcbrotli.NewReader(b)
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

func TestBrotliFinalize(t *testing.T) {
	// no t.Parallel() as finalizerHook is global and we need no other
	// allocations between the GC() calls

	runtime.GC()
	runtime.GC()

	finalizerWriter := make(chan string)
	*cbrotli.FinalizerHook = func(cw io.WriteCloser) {
		finalizerWriter <- fmt.Sprintf("%p", cw)
	}
	defer func() {
		*cbrotli.FinalizerHook = nil
	}()

	c, err := cbrotli.New(gcbrotli.WriterOptions{})
	if err != nil {
		t.Fatal(err)
	}

	cw := c.Get(io.Discard)
	if cw == nil {
		t.Fatal("nil writer")
	}

	writer := fmt.Sprintf("%p", cw)

	cw = nil // cw is now dead

	runtime.GC()
	runtime.GC()

	if fw := <-finalizerWriter; writer != fw {
		t.Fatalf("writer: %q, finalizer writer: %q", writer, fw)
	}
}
