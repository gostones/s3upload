package internal

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// TimeTrack reports elapsed time.
func TimeTrack(name string) func(time.Time) {
	var avg int64
	var n int64
	return func(start time.Time) {
		elapsed := time.Since(start)
		n++
		avg = (avg*(n-1) + int64(elapsed)) / n
		fmt.Printf("%s n: %v elapsed: %v average: %v\n\n", name, n, elapsed, time.Duration(avg))
	}
}

// ContentType reads first 512 bytes and sniffs the mime type.
func ContentType(r io.Reader) (string, error) {
	buf := make([]byte, 512)
	_, err := r.Read(buf)
	if err != nil {
		return "", err
	}
	ct := http.DetectContentType(buf)
	return ct, nil
}
