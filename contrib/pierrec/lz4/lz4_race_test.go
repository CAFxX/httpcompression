//go:build race

package lz4_test

import (
	"testing"

	"github.com/CAFxX/httpcompression/contrib/internal"
	"github.com/CAFxX/httpcompression/contrib/pierrec/lz4"
)

func TestLz4Race(t *testing.T) {
	t.Parallel()
	c, _ := lz4.New()
	internal.RaceTestCompressionProvider(c, 100)
}
