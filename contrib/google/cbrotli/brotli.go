//go:build cgo

package cbrotli

import (
	"fmt"
	"io"
	"runtime"

	"github.com/CAFxX/httpcompression/contrib/internal/utils"
	"github.com/google/brotli/go/cbrotli"
)

const (
	Encoding = "br"
)

type compressor struct {
	opts cbrotli.WriterOptions
}

func New(opts cbrotli.WriterOptions) (c *compressor, err error) {
	tw := cbrotli.NewWriter(io.Discard, opts)
	if err := utils.CheckWriter(tw); err != nil {
		return nil, fmt.Errorf("cbrotli: writer initialization: %w", err)
	}

	c = &compressor{opts: opts}
	return c, nil
}

func (c *compressor) Get(w io.Writer) io.WriteCloser {
	gw := cbrotli.NewWriter(w, c.opts)

	cw := &writer{
		Writer: gw,
	}
	runtime.SetFinalizer(cw, finalizer)
	return cw
}

type writer struct {
	*cbrotli.Writer
}

func (w *writer) Close() error {
	defer runtime.SetFinalizer(w, nil)
	return w.Writer.Close()
}

var finalizerHook func(io.WriteCloser)

func finalizer(cw *writer) {
	go func(cw *writer) {
		defer func() {
			if finalizerHook != nil {
				finalizerHook(cw) // for testing only
			}
			recover()
		}()
		_ = cw.Close()
	}(cw)
}
