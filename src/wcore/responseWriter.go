package wcore

import (
	"bytes"
	"net/http"
)

type BufferedResponseWriter struct {
	httpRespW http.ResponseWriter
	doBuff    bool
	buffer    bytes.Buffer
}

func NewBufferedWriter(w http.ResponseWriter) *BufferedResponseWriter {
	return &BufferedResponseWriter{
		httpRespW: w,
		doBuff:    true,
		// A Buffer needs no initialization.
	}
}
func (w *BufferedResponseWriter) Header() http.Header {
	return w.httpRespW.Header()
}

func (w *BufferedResponseWriter) WriteHeader(code int) {
	w.httpRespW.WriteHeader(code)
	if w.doBuff {
		w.Flush()
	}
}

func (w *BufferedResponseWriter) Write(b []byte) (int, error) {
	if w.doBuff {
		return w.buffer.Write(b)
	} else {
		return w.httpRespW.Write(b)
	}
}
func (w *BufferedResponseWriter) Flush() (int64, error) {
	if w.doBuff {
		w.doBuff = false

		return w.buffer.WriteTo(w.httpRespW)
	}
	return 0, nil
}
