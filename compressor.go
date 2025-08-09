package httpcompression

import (
	"io"
)

// CompressorProvider is the interface for compression implementations.
type CompressorProvider interface {
	// Get returns a writer that writes compressed output to the supplied parent io.Writer.
	// Callers of Get() must ensure to always call Close() when the compressor is not needed
	// anymore. Callers of Close() must also ensure to not use the io.WriteCloser once Close()
	// is called. Implementations of CompressorProvider are allowed to recycle the compressor
	// (e.g. put the WriteCloser in a pool to be reused by a later call to Get) when
	// Close() is called.
	Get(parent io.Writer) (compressor io.WriteCloser)
}

// Compressor returns an Option that sets the CompressorProvider for a specific Content-Encoding.
// If multiple CompressorProviders are set for the same Content-Encoding, the last one is used.
// If compressor is nil, it disables the specified Content-Encoding.
// Priority is used to specify the priority of the Content-Encoding. A higher number means higher
// priority. See PreferType to understand how priority is used to select the Content-Encoding for
// a specific request.
func Compressor(contentEncoding string, priority int, compressor CompressorProvider) Option {
	return func(c *config) error {
		if compressor == nil {
			delete(c.compressor, contentEncoding)
			return nil
		}
		c.compressor[contentEncoding] = comp{compressor, priority}
		return nil
	}
}

