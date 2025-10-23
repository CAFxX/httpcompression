package gin

import (
	"github.com/CAFxX/httpcompression"
	"github.com/gin-gonic/gin"
	wraphh "github.com/scr-oath/gin-wraphh"
)

func Adapter(opts ...httpcompression.Option) (gin.HandlerFunc, error) {
	hh, err := httpcompression.Adapter(opts...)
	if err != nil {
		return nil, err
	}
	return wraphh.WrapHH(hh), nil
}

func DefaultAdapter(opts ...httpcompression.Option) (gin.HandlerFunc, error) {
	hh, err := httpcompression.DefaultAdapter(opts...)
	if err != nil {
		return nil, err
	}
	return wraphh.WrapHH(hh), nil
}
