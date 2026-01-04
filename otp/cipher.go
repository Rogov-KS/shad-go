//go:build !solution

package otp

import (
	"io"
	// "byte"
)

type MyReader struct {
	r    io.Reader
	prng io.Reader
}

func (myr *MyReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	r_data := make([]byte, len(p))
	nr, nerr := myr.r.Read(r_data)
	if nr == 0 {
		if nerr != nil {
			return 0, nerr
		}
		return 0, nil
	}

	pr_data := make([]byte, nr)
	prn, prerr := myr.prng.Read(pr_data)
	if prerr != nil && prn == 0 {
		return 0, prerr
	}

	min_len := min(nr, prn)
	for ind := 0; ind < min_len; ind++ {
		p[ind] = r_data[ind] ^ pr_data[ind]
	}

	if nerr != nil {
		return min_len, nerr
	}
	return min_len, nil
}

func NewReader(r io.Reader, prng io.Reader) io.Reader {
	res := MyReader{r, prng}
	return &res
}

type MyWriter struct {
	w    io.Writer
	prng io.Reader
}

func (myw *MyWriter) Write(p []byte) (n int, err error) {
	pr_data := make([]byte, len(p))
	prn, prerr := myw.prng.Read(pr_data)
	if prerr != nil {
		return prn, prerr
	}
	min_len := min(prn, len(p))
	w_data := make([]byte, min_len)
	for ind := 0; ind < min_len; ind++ {
		w_data[ind] = p[ind] ^ pr_data[ind]
	}
	wn, werr := myw.w.Write(w_data)
	if werr != nil {
		return wn, werr
	}
	return min_len, nil
}

func NewWriter(w io.Writer, prng io.Reader) io.Writer {
	res := MyWriter{w, prng}
	return &res
}
