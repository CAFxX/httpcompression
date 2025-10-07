package httpcompression

import (
	"compress/gzip"
	"fmt"
	"github.com/CAFxX/httpcompression/contrib/andybalholm/brotli"
	"github.com/CAFxX/httpcompression/contrib/compress/zlib"
	"net/http"
	"sync"
)

type codings map[string]float64

type ResponseWriterFactoryFactory struct {
	config     *config
	bufPool    *sync.Pool
	writerPool *sync.Pool
}

func NewResponseWriterFactory(opts ...Option) (*ResponseWriterFactoryFactory, error) {
	f := ResponseWriterFactoryFactory{
		config: &config{
			prefer:     PreferServer,
			compressor: comps{},
		},
		bufPool:    &sync.Pool{},
		writerPool: &sync.Pool{},
	}

	f.bufPool.New = func() interface{} {
		return &[]byte{}
	}
	f.writerPool.New = func() interface{} {
		return &compressWriter{
			config: f.config,
			pool:   f.bufPool,
		}
	}

	for _, o := range opts {
		err := o(f.config)
		if err != nil {
			return nil, err
		}
	}

	if f.config.minSize == 0 && f.config.minSizeFunc == nil {
		f.config.minSize = DefaultMinSize
	}

	return &f, nil
}

// NewDefaultResponseWriterFactory is like NewResponseWriterFactory, but it includes sane
// defaults for general usage.
// Currently, the defaults enable gzip and brotli compression, and set a minimum body size
// of 200 bytes.
// The provided opts override the defaults.
// The defaults are not guaranteed to remain constant over time: if you want to avoid this
// use NewResponseWriterFactory directly.
func NewDefaultResponseWriterFactory(opts ...Option) (*ResponseWriterFactoryFactory, error) {
	defaults := []Option{
		DeflateCompressionLevel(zlib.DefaultCompression),
		GzipCompressionLevel(gzip.DefaultCompression),
		BrotliCompressionLevel(brotli.DefaultCompression),
		defaultZstandardCompressor(),
	}
	opts = append(defaults, opts...)
	return NewResponseWriterFactory(opts...)
}

// Create wraps the given http.ResponseWriter into a new instance
// with is using compressor (if supported and requested).
//
// Important: Finalizer() must be called *always*, as this will
// in turn Close() the compressor. This is important because
// it is guaranteed by the CompressorProvider interface, and
// because some compressors may be implemented via cgo, and they
// may rely on Close() being called to release memory resources.
func (f *ResponseWriterFactoryFactory) Create(rw http.ResponseWriter, req *http.Request) (http.ResponseWriter, Finalizer, error) {
	addVaryHeader(rw.Header(), acceptEncoding)

	accept := parseEncodings(req.Header.Values(acceptEncoding))
	common := acceptedCompression(accept, f.config.compressor)
	if len(common) == 0 {
		return rw, noopFinalizer, nil
	}

	// We do not handle range requests when compression is used, as the
	// range specified applies to the compressed data, not to the uncompressed one.
	// So we would need to (1) ensure that compressors are deterministic and (2)
	// generate the whole uncompressed response anyway, compress it, and then discard
	// the bits outside the range.
	// Let's keep it simple, and simply ignore completely the range header.
	// We also need to remove the Accept: Range header from any response that is
	// compressed; this is done in the ResponseWriter.
	// See https://github.com/nytimes/gziphandler/issues/83.
	req.Header.Del(_range)

	minSize := f.config.minSize
	if msf := f.config.minSizeFunc; msf != nil {
		v, err := msf(req)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot resolve minSize for request: %w", err)
		}
		minSize = v
	}

	cw := f.writerPool.Get().(*compressWriter)
	cw.configure(rw, minSize, accept, common)

	if _, ok := rw.(http.CloseNotifier); ok {
		rw = compressWriterWithCloseNotify{cw}
	} else {
		rw = cw
	}

	return rw, func() error {
		defer f.writerPool.Put(cw)
		defer cw.clean()
		return cw.Close()
	}, nil
}

// AmountOfCompressors returns the amount of compressors configured at this ResponseWriterFactoryFactory.
func (f *ResponseWriterFactoryFactory) AmountOfCompressors() int {
	return len(f.config.compressor)
}

func (f *ResponseWriterFactoryFactory) getBuffer() *[]byte {
	b := f.bufPool.Get()
	if b == nil {
		var s []byte
		return &s
	}
	return b.(*[]byte)
}

func (f *ResponseWriterFactoryFactory) recycleBuffer(target *[]byte) {
	if target == nil {
		return
	}
	if cap(*target) > maxBuf {
		// If the buffer is too big, let's drop it to avoid
		// keeping huge buffers alive in the pool. In this case
		// we still recycle the pointer to the slice.
		*target = nil
	}
	if len(*target) > 0 {
		// Reset the buffer to zero length.
		*target = (*target)[:0]
	}
	f.bufPool.Put(target)
}

type Finalizer func() error

func noopFinalizer() error { return nil }
