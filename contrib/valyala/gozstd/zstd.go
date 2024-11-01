//go:build cgo

package gozstd

import (
	"fmt"
	"io"
	"sync"

	"github.com/CAFxX/httpcompression/contrib/internal/utils"
	"github.com/valyala/gozstd"
)

const (
	Encoding = "zstd"
)

type compressor struct {
	pool sync.Pool
	opts gozstd.WriterParams
}

func New(opts gozstd.WriterParams) (c *compressor, err error) {
	defer func() {
		if r := recover(); r != nil {
			c, err = nil, fmt.Errorf("panic: %v", r)
		}
	}()

	tw := gozstd.NewWriterParams(io.Discard, &opts)
	if err := utils.CheckWriter(tw); err != nil {
		return nil, fmt.Errorf("gozstd: writer initialization: %w", err)
	}

	c = &compressor{opts: opts}
	return c, nil
}

func (c *compressor) Get(w io.Writer) io.WriteCloser {
	if gw, ok := c.pool.Get().(*zstdWriter); ok {
		gw.ResetWriterParams(w, &c.opts)
		return gw
	}
	gw := gozstd.NewWriterParams(w, &c.opts)
	return &zstdWriter{
		Writer: gw,
		c:      c,
	}
}

type zstdWriter struct {
	*gozstd.Writer
	c *compressor
}

func (w *zstdWriter) Close() error {
	err := w.Writer.Close()
	w.ResetWriterParams(nil, &w.c.opts) // drop reference to parent writer
	w.c.pool.Put(w)
	return err
}
