package main

import (
	"io"
	"os"
)

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

type LimitedWriter struct {
	W io.Writer
	N int64
}

func (state *LimitedWriter) Write(bytes []byte) (written int, err error) {
	var sizeToWrite int
	if state.N < int64(len(bytes)) {
		sizeToWrite = int(state.N)
		if debug {
			if sizeToWrite > 0 {
				Debug.Print("Skipping bytes")
			} else {
				Debug.Print(".")
			}
		}
	} else {
		sizeToWrite = len(bytes)
	}
	if sizeToWrite > 0 {
		written, err = state.W.Write(bytes[:sizeToWrite])
		if written > 0 {
			written = len(bytes) // if the write was successful, mimic that we wrote all bytes
		}
		state.N -= int64(written)
	} else {
		return len(bytes), nil // returning 0 would mean that the write was not successful
	}
	return
}

func LimitWriter(w io.Writer, n int64) io.Writer {
	return &LimitedWriter{w, n}
}
