//go:build race

package gzip_test

import (
	"testing"

	"github.com/CAFxX/httpcompression/contrib/compress/gzip"
	"github.com/CAFxX/httpcompression/contrib/internal"
)

func TestZstdRace(t *testing.T) {
	t.Parallel()
	c, _ := gzip.New(gzip.Options{})
	internal.RaceTestCompressionProvider(c, 100)
}
