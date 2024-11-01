//go:build race

package xz_test

import (
	"testing"

	"github.com/CAFxX/httpcompression/contrib/internal"
	"github.com/CAFxX/httpcompression/contrib/ulikunitz/xz"
	pxz "github.com/ulikunitz/xz"
)

func TestXzRace(t *testing.T) {
	t.Parallel()
	c, _ := xz.New(pxz.WriterConfig{})
	internal.RaceTestCompressionProvider(c, 100)
}
