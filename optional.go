package httpcompression

import (
	"bufio"
	"net"
	"net/http"
)

// extend returns a http.ResponseWriter that wraps the compressWriter and that
// dynamically exposes some optional interfaces of http.ResponseWriter.
// Currently the supported optional interfaces are http.Hijacker, http.Pusher,
// and http.CloseNotifier.
// This is obviously an horrible way of doing things, but it's really unavoidable
// without proper language support for interface extension; see
// https://blog.merovius.de/2017/07/30/the-trouble-with-optional-interfaces.html
// for details.
func extend(cw *compressWriter) http.ResponseWriter {
	switch cw.ResponseWriter.(type) {
	case interface {
		http.Hijacker
		http.Pusher
		http.CloseNotifier
	}:
		return cwHijackPushCloseNotifier{cw}
	case interface {
		http.Pusher
		http.CloseNotifier
	}:
		return cwPushCloseNotifier{cw}
	case interface {
		http.Hijacker
		http.Pusher
	}:
		return cwHijackPusher{cw}
	case interface {
		http.Hijacker
		http.CloseNotifier
	}:
		return cwHijackCloseNotifier{cw}
	case http.CloseNotifier:
		return cwCloseNotifier{cw}
	case http.Hijacker:
		return cwHijacker{cw}
	case http.Pusher:
		return cwPusher{cw}
	default:
		return cw
	}
}

type cwHijacker struct{ *compressWriter }

func (cw cwHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return cw.ResponseWriter.(http.Hijacker).Hijack()
}

var _ http.Hijacker = cwHijacker{}

type cwCloseNotifier struct{ *compressWriter }

func (cw cwCloseNotifier) CloseNotify() <-chan bool {
	return cw.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

var _ http.CloseNotifier = cwCloseNotifier{}

type cwPusher struct{ *compressWriter }

func (cw cwPusher) Push(target string, opts *http.PushOptions) error {
	return cw.ResponseWriter.(http.Pusher).Push(target, opts)
}

var _ http.Pusher = cwPusher{}

type cwHijackCloseNotifier struct{ *compressWriter }

func (cw cwHijackCloseNotifier) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return cwHijacker{cw.compressWriter}.Hijack()
}
func (cw cwHijackCloseNotifier) CloseNotify() <-chan bool {
	return cwCloseNotifier{cw.compressWriter}.CloseNotify()
}

var (
	_ http.Hijacker      = cwHijackCloseNotifier{}
	_ http.CloseNotifier = cwHijackCloseNotifier{}
)

type cwHijackPusher struct{ *compressWriter }

func (cw cwHijackPusher) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return cwHijacker{cw.compressWriter}.Hijack()
}
func (cw cwHijackPusher) Push(target string, opts *http.PushOptions) error {
	return cwPusher{cw.compressWriter}.Push(target, opts)
}

var (
	_ http.Hijacker = cwHijackPusher{}
	_ http.Pusher   = cwHijackPusher{}
)

type cwPushCloseNotifier struct{ *compressWriter }

func (cw cwPushCloseNotifier) Push(target string, opts *http.PushOptions) error {
	return cwPusher{cw.compressWriter}.Push(target, opts)
}
func (cw cwPushCloseNotifier) CloseNotify() <-chan bool {
	return cwCloseNotifier{cw.compressWriter}.CloseNotify()
}

var (
	_ http.Pusher        = cwPushCloseNotifier{}
	_ http.CloseNotifier = cwPushCloseNotifier{}
)

type cwHijackPushCloseNotifier struct{ *compressWriter }

func (cw cwHijackPushCloseNotifier) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return cwHijacker{cw.compressWriter}.Hijack()
}
func (cw cwHijackPushCloseNotifier) Push(target string, opts *http.PushOptions) error {
	return cwPusher{cw.compressWriter}.Push(target, opts)
}
func (cw cwHijackPushCloseNotifier) CloseNotify() <-chan bool {
	return cwCloseNotifier{cw.compressWriter}.CloseNotify()
}

var (
	_ http.Hijacker      = cwHijackPushCloseNotifier{}
	_ http.Pusher        = cwHijackPushCloseNotifier{}
	_ http.CloseNotifier = cwHijackPushCloseNotifier{}
)
