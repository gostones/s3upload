package internal

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
)

// MD5Sum computes MD5 message digest fingerprint, encodes in base64 representation
// and returns checksums in base64 and in hex.
func MD5Sum(r io.Reader) (string, string, error) {
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", "", err
	}
	sum := h.Sum(nil)
	s := hex.EncodeToString(sum)
	return base64.StdEncoding.EncodeToString([]byte(s)), s, nil
}

// MD5SumFile computes and returns checksums for file in base64 and hex.
func MD5SumFile(file string) (string, string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", "", err
	}
	b64, hex, err := MD5Sum(f)
	f.Close()

	return b64, hex, err
}
