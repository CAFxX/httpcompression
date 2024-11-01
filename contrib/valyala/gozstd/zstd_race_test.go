//go:build race && cgo

package gozstd_test

import (
	"testing"

	"github.com/CAFxX/httpcompression/contrib/internal"
	"github.com/CAFxX/httpcompression/contrib/valyala/gozstd"
	vzstd "github.com/valyala/gozstd"
)

func TestZstdRace(t *testing.T) {
	t.Parallel()
	c, _ := gozstd.New(vzstd.WriterParams{})
	internal.RaceTestCompressionProvider(c, 100)
}
