package httpcompression_test

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/CAFxX/httpcompression"
	"github.com/CAFxX/httpcompression/contrib/andybalholm/brotli"
	"github.com/CAFxX/httpcompression/contrib/klauspost/gzip"
	"github.com/CAFxX/httpcompression/contrib/klauspost/zstd"
	"github.com/CAFxX/httpcompression/contrib/pierrec/lz4"
	kpzstd "github.com/klauspost/compress/zstd"
)

func Example() {
	// Create a compression adapter with default configuration
	compress, err := httpcompression.DefaultAdapter()
	if err != nil {
		log.Fatal(err)
	}
	// Define your handler, and apply the compression adapter.
	http.Handle("/", compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello world!"))
	})))
	// ...
}

func Example_custom() {
	brEnc, err := brotli.New(brotli.Options{})
	if err != nil {
		log.Fatal(err)
	}
	gzEnc, err := gzip.New(gzip.Options{})
	if err != nil {
		log.Fatal(err)
	}
	_, _ = httpcompression.Adapter(
		httpcompression.Compressor(brotli.Encoding, 1, brEnc),
		httpcompression.Compressor(gzip.Encoding, 0, gzEnc),
		httpcompression.Prefer(httpcompression.PreferServer),
		httpcompression.MinSize(100),
		httpcompression.ContentTypes([]string{
			"image/jpeg",
			"image/gif",
			"image/png",
		}, true),
	)
}

func Example_withDictionary() {
	// Default zstd compressor
	zEnc, err := zstd.New()
	if err != nil {
		log.Fatal(err)
	}
	// zstd compressor with custom dictionary
	dict, coding, err := readZstdDictionary("tests/dictionary")
	if err != nil {
		log.Fatal(err)
	}
	zdEnc, err := zstd.New(kpzstd.WithEncoderDict(dict))
	if err != nil {
		log.Fatal(err)
	}
	_, _ = httpcompression.DefaultAdapter(
		// Add the zstd compressor with the dictionary.
		// We need to pick a custom content-encoding name. It is recommended to:
		// - avoid names that contain standard names (e.g. "gzip", "deflate", "br" or "zstd")
		// - include the dictionary ID, so that multiple dictionaries can be used (including
		//   e.g. multiple versions of the same dictionary)
		httpcompression.Compressor(coding, 3, zdEnc),
		httpcompression.Compressor(zstd.Encoding, 2, zEnc),
		httpcompression.Prefer(httpcompression.PreferServer),
		httpcompression.MinSize(0),
		httpcompression.ContentTypes([]string{
			"image/jpeg",
			"image/gif",
			"image/png",
		}, true),
	)
}

func readZstdDictionary(file string) (dict []byte, coding string, err error) {
	dictFile, err := os.Open(file)
	if err != nil {
		return nil, "", err
	}
	dict, err = ioutil.ReadAll(dictFile)
	if err != nil {
		return nil, "", err
	}
	if len(dict) < 8 {
		return nil, "", fmt.Errorf("invalid dictionary")
	}
	dictID := binary.LittleEndian.Uint32(dict[4:8]) // read the dictionary ID
	// Build the encoding name: z_XXXXXXXX (where XXXXXXXX is the dictionary ID in hex lowercase).
	// There is no standard way to communicate the use of a dictionary with a content-encoding:
	// so we simply use a non-standard name to identify the encoding-dictionary pair in use.
	// This naming scheme is arbitrary and as long as it is not one of those in the IANA registry
	// (https://www.iana.org/assignments/http-parameters/http-parameters.xhtml#content-coding)
	// anything should work. It is recommended not to include one of the standard names even as a
	// substring of the chosen name as some poorly-configured proxies may simply perform a case
	// insensitive substring match for e.g. "deflate", in which case the name e.g.
	// "deflate_12345678" would still match, even though it should not as a deflate decompressor
	// without the dictionary will fail to decompress the contents.
	coding = fmt.Sprintf("z_%08x", dictID)
	return
}

// Example_customCompressor shows how to create an httpcompression adapter with a custom compressor.
// In this case we use the pierrec/lz4 compressor in contrib, but it could be replaced with any
// other compressor, as long as it implements the CompressorProvider interface.
func Example_customCompressor() {
	c, err := lz4.New()
	if err != nil {
		log.Fatal(err)
	}
	_, _ = httpcompression.Adapter(
		// Only enable the custom compressor; other options/compressors can be added if needed,
		// or DefaultAdapter could be used as a baseline.
		httpcompression.Compressor(lz4.Encoding, 0, c),
	)
}
