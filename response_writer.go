package httpcompression

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
)

var (
	// ErrHijackerNotSupported indicates that a provided http.ResponseWriter does not implement
	// http.Hijacker and can be therefore not be called by wrapped responses.
	ErrHijackerNotSupported = fmt.Errorf("http.Hijacker interface is not supported")
)

// compressWriter provides a http.ResponseWriter interface, which gzips
// bytes before writing them to the underlying response. This doesn't close the
// writers, so don't forget to do that.
// It can be configured to skip response smaller than minSize.
type compressWriter struct {
	http.ResponseWriter

	config  *config
	minSize int
	accept  codings
	common  []string
	pool    *sync.Pool // pool of buffers (buf []byte); max size of each buf is maxBuf

	w    io.Writer
	enc  string
	code int     // Saves the WriteHeader value.
	buf  *[]byte // Holds the first part of the write before reaching the minSize or the end of the write.
}

var (
	_ io.WriteCloser  = &compressWriter{}
	_ http.Flusher    = &compressWriter{}
	_ http.Hijacker   = &compressWriter{}
	_ io.StringWriter = &compressWriter{}
)

type compressWriterWithCloseNotify struct {
	*compressWriter
}

func (w compressWriterWithCloseNotify) CloseNotify() <-chan bool {
	return w.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

var (
	_ io.WriteCloser  = compressWriterWithCloseNotify{}
	_ http.Flusher    = compressWriterWithCloseNotify{}
	_ http.Hijacker   = compressWriterWithCloseNotify{}
	_ io.StringWriter = compressWriterWithCloseNotify{}
)

const maxBuf = 1 << 16 // maximum size of recycled buffer

func (w *compressWriter) configure(
	rw http.ResponseWriter,
	minSize int,
	accept codings,
	common []string,
) {
	w.ResponseWriter = rw
	w.minSize = minSize
	w.accept = accept
	w.common = common
	w.w = nil
}

func (w *compressWriter) clean() {
	w.ResponseWriter = nil
	w.minSize = 0
	w.accept = nil
	w.common = nil
	w.w = nil
}

// Write compresses and appends the given byte slice to the underlying ResponseWriter.
func (w *compressWriter) Write(b []byte) (int, error) {
	if w.w != nil {
		// The responseWriter is already initialized: use it.
		return w.w.Write(b)
	}

	var (
		ct = w.Header().Get(contentType)
		ce = w.Header().Get(contentEncoding)
		cl = 0
	)
	if clv := w.Header().Get(contentLength); clv != "" {
		cl, _ = strconv.Atoi(clv)
	}

	// Fast path: we have enough information to know whether we will compress
	// or not this response from the first write, so we don't need to buffer
	// writes to defer the decision until we have more data.
	if w.buf == nil && (ct != "" || len(w.config.contentTypes) == 0) && (cl > 0 || len(b) >= w.minSize) {
		if ce == "" && (cl >= w.minSize || len(b) >= w.minSize) && handleContentType(ct, w.config.contentTypes, w.config.blacklist) {
			enc := preferredEncoding(w.accept, w.config.compressor, w.common, w.config.prefer)
			if err := w.startCompress(enc, b); err != nil {
				return 0, err
			}
			return len(b), nil
		}
		if err := w.startPlain(b); err != nil {
			return 0, err
		}
		return len(b), nil
	}

	// Slow path: we don't have yet enough information to decide whether we should
	// compress this response. Append the data to a temporary buffer and then try again.
	if w.buf == nil {
		w.buf = w.getBuffer()
	}
	*w.buf = append(*w.buf, b...)

	// Only continue if they didn't already choose an encoding or a known unhandled content length or type.
	if ce == "" && (cl == 0 || cl >= w.minSize) && (ct == "" || handleContentType(ct, w.config.contentTypes, w.config.blacklist)) {
		// If the current buffer is less than minSize and a Content-Length isn't set, then wait until we have more data.
		if len(*w.buf) < w.minSize && cl == 0 {
			return len(b), nil
		}
		// If the Content-Length is larger than minSize or the current buffer is larger than minSize, then continue.
		if cl >= w.minSize || len(*w.buf) >= w.minSize {
			// If a Content-Type wasn't specified, infer it from the current buffer.
			if ct == "" {
				ct = http.DetectContentType(*w.buf)
				if ct != "" {
					// net/http by default performs content sniffing but this is disabled if content-encoding is set.
					// Since we set content-encoding, if content-type was not set, and we successfully sniffed it,
					// set the content-type.
					w.Header().Set(contentType, ct)
				}
			}
			if handleContentType(ct, w.config.contentTypes, w.config.blacklist) {
				enc := preferredEncoding(w.accept, w.config.compressor, w.common, w.config.prefer)
				if err := w.startCompress(enc, *w.buf); err != nil {
					return 0, err
				}
				return len(b), nil
			}
		}
	}
	// If we got here, we should not GZIP this response.
	if err := w.startPlain(*w.buf); err != nil {
		return 0, err
	}
	return len(b), nil
}

// WriteString compresses and appends the given string to the underlying ResponseWriter.
//
// This makes use of an optional method (WriteString) exposed by the compressors, or by
// the underlying ResponseWriter.
func (w *compressWriter) WriteString(s string) (int, error) {
	// Since WriteString is an optional interface of the compressor, and the actual compressor
	// is chosen only after the first call to Write, we can't statically know whether the interface
	// is supported. We therefore have to check dynamically.
	if ws, _ := w.w.(io.StringWriter); ws != nil {
		// The responseWriter is already initialized and it implements WriteString.
		return ws.WriteString(s)
	}
	// Fallback: the writer has not been initialized yet, or it has been initialized
	// and it does not implement WriteString. We could in theory do something unsafe
	// here but for now let's keep it simple and fallback to Write.
	// TODO: in case the string is large, we should avoid allocating a full copy:
	// instead we should copy the string in chunks.
	return w.Write([]byte(s))
}

// startCompress initializes a compressing writer and writes the buffer.
func (w *compressWriter) startCompress(enc string, buf []byte) error {
	comp, ok := w.config.compressor[enc]
	if !ok {
		panic("unknown compressor")
	}

	w.Header().Set(contentEncoding, enc)

	// if the Content-Length is already set, then calls to Write on gzip
	// will fail to set the Content-Length header since its already set
	// See: https://github.com/golang/go/issues/14975.
	w.Header().Del(contentLength)

	// See the comment about ranges in adapter.go
	w.Header().Del(acceptRanges)

	// Write the header to gzip response.
	if w.code != 0 {
		w.ResponseWriter.WriteHeader(w.code)
		// Ensure that no other WriteHeader's happen
		w.code = 0
	}

	defer w.recycleBuffer()

	// Initialize and flush the buffer into the gzip response if there are any bytes.
	// If there aren't any, we shouldn't initialize it yet because on Close it will
	// write the gzip header even if nothing was ever written.
	if len(buf) > 0 {
		w.w = comp.comp.Get(w.ResponseWriter)
		w.enc = enc

		n, err := w.w.Write(buf)

		// This should never happen (per io.Writer docs), but if the write didn't
		// accept the entire buffer but returned no specific error, we have no clue
		// what's going on, so abort just to be safe.
		if err == nil && n < len(buf) {
			err = io.ErrShortWrite
		}
		return err
	}
	return nil
}

// startPlain writes to sent bytes and buffer the underlying ResponseWriter without gzip.
func (w *compressWriter) startPlain(buf []byte) error {
	// See the comment about ranges in adapter.go; we need to do it even in this case
	// because adapter will strip the range header anyway.
	w.Header().Del(acceptRanges)

	if w.code != 0 {
		w.ResponseWriter.WriteHeader(w.code)
		// Ensure that no other WriteHeader's happen
		w.code = 0
	}
	w.w = w.ResponseWriter
	w.enc = ""
	// If Write was never called then don't call Write on the underlying ResponseWriter.
	if buf == nil {
		return nil
	}
	n, err := w.ResponseWriter.Write(buf)
	// This should never happen (per io.Writer docs), but if the write operation
	// didn't accept the entire buffer but returned no specific error, we have no
	// clue what's going on, so abort just to be safe.
	if err == nil && n < len(buf) {
		err = io.ErrShortWrite
	}
	w.recycleBuffer()
	return err
}

// WriteHeader sets the response code that will be returned in the response.
func (w *compressWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
}

// Close closes the compression Writer.
func (w *compressWriter) Close() error {
	if w.w != nil && w.enc == "" {
		return nil
	}
	if cw, ok := w.w.(io.Closer); ok {
		w.w = nil
		return cw.Close()
	}

	// compression not triggered yet, write out regular response.
	var buf []byte
	if w.buf != nil {
		buf = *w.buf
	}
	err := w.startPlain(buf)
	// Returns the error if any at write.
	if err != nil {
		err = fmt.Errorf("httpcompression: write to regular responseWriter at close gets error: %v", err)
	}
	return err
}

// Flush flushes the underlying compressor Writer and then the underlying
// http.ResponseWriter if it is a http.Flusher. This makes compressWriter
// a http.Flusher.
// Flush is a no-op until enough data has been written to decide whether the
// response should be compressed or not (e.g. less than MinSize bytes have
// been written).
func (w *compressWriter) Flush() {
	if w.w == nil {
		// Flush is thus a no-op until we're certain whether a plain
		// or compressed response will be served.
		return
	}

	// Flush the compressor, if supported.
	// note: http.ResponseWriter does not implement Flusher (http.Flusher does not return an error),
	// so we need to later call ResponseWriter.Flush anyway:
	// - in case we are bypassing compression, w.w is the parent ResponseWriter, and therefore we skip
	//   this as the parent ResponseWriter does not implement Flusher.
	// - in case we are NOT bypassing compression, w.w is the compressor, and therefore we flush the
	//   compressor, and then we flush the parent ResponseWriter.
	if fw, ok := w.w.(Flusher); ok {
		_ = fw.Flush()
	}

	// Flush the ResponseWriter (the previous Flusher is not expected to flush the parent writer).
	if fw, ok := w.ResponseWriter.(http.Flusher); ok {
		fw.Flush()
	}
}

// Hijack implements http.Hijacker. If the underlying ResponseWriter is a
// Hijacker, its Hijack method is returned. Otherwise, an error is returned.
func (w *compressWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, ErrHijackerNotSupported
}

func (w *compressWriter) getBuffer() *[]byte {
	return w.pool.Get().(*[]byte)
}

func (w *compressWriter) recycleBuffer() {
	if w.buf == nil {
		return
	}
	buf := w.buf
	w.buf = nil
	if cap(*buf) > maxBuf {
		// If the buffer is too big, let's drop it to avoid
		// keeping huge buffers alive in the pool. In this case
		// we still recycle the pointer to the slice.
		*buf = nil
	}
	if len(*buf) > 0 {
		// Reset the buffer to zero length.
		*buf = (*buf)[:0]
	}
	w.pool.Put(buf)
}
