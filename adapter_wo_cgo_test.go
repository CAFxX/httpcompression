//go:build !cgo

package httpcompression

import (
	"fmt"
	"github.com/CAFxX/httpcompression/contrib/andybalholm/brotli"
	kpgzip "github.com/CAFxX/httpcompression/contrib/klauspost/gzip"
	"github.com/CAFxX/httpcompression/contrib/klauspost/zstd"
	kpzstd "github.com/klauspost/compress/zstd"
)

var (
	benchMarkComps = map[string]int{stdlibGzip: 9, klauspostGzip: 9, andybalholmBrotli: 11, klauspostZstd: 4, valyalaGozstd: 22}
)

func benchmarkCompressorProvider(ae string, d int) (CompressorProvider, error) {
	switch ae {
	case stdlibGzip:
		return NewDefaultGzipCompressor(d)
	case klauspostGzip:
		return kpgzip.New(kpgzip.Options{Level: d})
	case andybalholmBrotli:
		return brotli.New(brotli.Options{Quality: d})
	case klauspostZstd:
		return zstd.New(kpzstd.WithEncoderLevel(kpzstd.EncoderLevel(d)))
	default:
		return nil, fmt.Errorf("unknown compressor provider: %s", ae)
	}
}
