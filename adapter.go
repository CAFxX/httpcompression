package httpcompression

import (
	"net/http"
	"strings"
)

const (
	vary            = "Vary"
	acceptEncoding  = "Accept-Encoding"
	acceptRanges    = "Accept-Ranges"
	contentEncoding = "Content-Encoding"
	contentType     = "Content-Type"
	contentLength   = "Content-Length"
	_range          = "Range"
)

// Adapter returns an HTTP handler wrapping function (a.k.a. middleware)
// which can be used to wrap an HTTP handler to transparently compress the response
// body if the client supports it (via the Accept-Encoding header).
// It is possible to pass one or more options to modify the middleware configuration.
// If no options are provided, no compressors are enabled and therefore the adapter
// is a no-op.
// An error will be returned if invalid options are given.
func Adapter(opts ...Option) (func(http.Handler) http.Handler, error) {
	f, err := NewResponseWriterFactory(opts...)
	if err != nil {
		return nil, err
	}
	return adapter(f)
}

func adapter(f *ResponseWriterFactoryFactory) (func(http.Handler) http.Handler, error) {
	if f.AmountOfCompressors() == 0 {
		// No compressors have been configured, so there is no useful work
		// that this adapter can do.
		return func(h http.Handler) http.Handler {
			return h
		}, nil
	}

	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			ww, finalizer, err := f.Create(rw, req)
			if err != nil {
				f.config.handleError(rw, req, err)
				return
			}

			defer func() {
				if err := finalizer(); err != nil {
					f.config.handleError(rw, req, err)
				}
			}()

			h.ServeHTTP(ww, req)
		})
	}, nil
}

func addVaryHeader(h http.Header, value string) {
	for _, v := range h.Values(vary) {
		if strings.EqualFold(value, v) {
			return
		}
	}
	h.Add(vary, value)
}

// DefaultAdapter is like Adapter, but it includes sane defaults for general usage.
// Currently, the defaults enable gzip and brotli compression, and set a minimum body size
// of 200 bytes.
// The provided opts override the defaults.
// The defaults are not guaranteed to remain constant over time: if you want to avoid this
// use Adapter directly.
func DefaultAdapter(opts ...Option) (func(http.Handler) http.Handler, error) {
	f, err := NewDefaultResponseWriterFactory(opts...)
	if err != nil {
		return nil, err
	}
	return adapter(f)
}
