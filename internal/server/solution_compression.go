package server

import (
	"compress/gzip"
	"net/http"
	"strings"
)

// withSolutionCompression reduces Markdown-heavy solution responses in transit.
// Storage compression remains independent and is verified when revisions load.
func withSolutionCompression(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if !strings.Contains(strings.ToLower(r.Header.Get("Accept-Encoding")), "gzip") {
			next(w, r)
			return
		}
		writer, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			next(w, r)
			return
		}
		defer writer.Close()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		next(gzipResponseWriter{ResponseWriter: w, writer: writer}, r)
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (w gzipResponseWriter) Write(body []byte) (int, error) {
	return w.writer.Write(body)
}
