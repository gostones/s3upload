package internal

import (
	"fmt"
	"testing"
	"time"
)

func TestMD5Sum(t *testing.T) {
	b64, hex, err := MD5SumFile("./testdata/file.txt")
	if err != nil {
		t.FailNow()
	}
	t.Log("bas64:", b64, "hex:", hex)
	if b64 != "OWZlZjJjNDk3YzdjMGU5MTVkMjIwMTdkZGQyZDdjYzM=" || hex != "9fef2c497c7c0e915d22017ddd2d7cc3" {
		t.FailNow()
	}
}

// mkfile.sh
// md5 -q  /tmp/500M.raw|tr -d '\n'|base64
// go test -timeout 30m github.com/gostones/s3upload/internal -run ^TestMD5SumIntegration$ -v
func TestMD5SumIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	files := []string{
		"/tmp/500M.raw",
		"/tmp/1G.raw",
		"/tmp/2G.raw",
		"/tmp/3G.raw",
		"/tmp/5G.raw",
		"/tmp/10G.raw",
		"/tmp/20G.raw",
		"/tmp/30G.raw",
		"/tmp/50G.raw",
	}
	repeat := 1
	//
	compute := func(files []string, repeat int) {
		for _, file := range files {
			track := TimeTrack(file)
			checksum := func() {
				defer track(time.Now())

				md5, hex, _ := MD5SumFile(file)
				fmt.Printf("file: %s base64: %s checksum: %s\n", file, md5, hex)
			}

			for i := 0; i < repeat; i++ {
				checksum()
			}
		}
	}

	compute(files, repeat)
}
