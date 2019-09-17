package internal

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestChunkReader(t *testing.T) {
	var filename = "./testdata/file.txt"
	var chunksize int64 = 3
	fc := NewFileChunk(filename, chunksize)

	if err := fc.Open(); err != nil {
		t.Log(err)
		t.FailNow()
	}
	defer fc.Close()

	b64, hex, err := fc.MD5()
	if err != nil || b64 != "OWZlZjJjNDk3YzdjMGU5MTVkMjIwMTdkZGQyZDdjYzM=" || hex != "9fef2c497c7c0e915d22017ddd2d7cc3" {
		t.FailNow()
	}
	t.Logf("file: %s name: %s content: %s md5: %s  %s error: %v", filename, fc.Name(), fc.ContentType(), b64, hex, err)

	// chunk checksum
	chunkExpected := []string{
		"d9aebf7d5a83db9709fe0af7b92ab73a",
		"02877caceda6a22108cb321692554665",
		"8d62496a8a84cc95207a6502c0a3947a",
		"44e899558b8b9b4996fc883559885641",
		"a181a603769c1f98ad927e7367c7aa51",
		"1f8a656131afd7464350952c5c0f8b58",
		"5d677a22ce10c583311d659c03aa1369",
		"c9bfff4bc263d1ead42f5d2b0d206858",
		"42627bb259dd12f669e198b0c31008ba",
		"65b50b04a6af50bb2f174db30a8c6dad",
		"a498382929241d9ba043e11a272750af",
		"90e9b1d2e17507bd1f8ecdedad09bec7",
		"bd4cce514d35f5ac6311afe18af75bdc",
		"046c2af9e78e52537c28bae486fef69e",
		"0ed09a865943447e2a902fbb596c6ae9",
		"8fc42c6ddf9966db3b09e84365034357",
		"1a46b8973dcfe46a948f1ea9eeb2328d",
		"d9180594744f870aeefb086982e980bb",
	}
	fc.MapAsync(func(i int, r *ChunkReader) error {
		b64, hex, err := r.MD5()
		t.Logf("chunk: %v md5: %s  %s error: %v", i, b64, hex, err)
		if hex != chunkExpected[i] {
			t.FailNow()
		}
		return nil
	})

	//
	var expected string
	b, _ := ioutil.ReadFile(filename)
	expected = string(b)
	t.Log("expected:", expected)

	//
	readers := []io.Reader{}
	fc.Map(func(i int, r *ChunkReader) error {
		readers = append(readers, r)
		return nil
	})
	mr := io.MultiReader(readers...)
	buf := bytes.NewBuffer(nil)
	io.Copy(buf, mr)
	s := string(buf.Bytes())
	t.Log("chuncked", s)

	if string(buf.Bytes()) != string(expected) {
		t.FailNow()
	}
	t.Logf("expected: %v count: %v", len(expected), fc.Count())
	if int64(len(expected)) != fc.Count() {
		t.FailNow()
	}

	t.Logf("counter: %v", fc.Count())

	buf = bytes.NewBuffer(nil)
	fc.Map(func(i int, r *ChunkReader) error {
		io.Copy(buf, r)
		s := string(buf.Bytes())
		t.Log("chuncked", s)
		return nil
	})
	if string(buf.Bytes()) != expected {
		t.FailNow()
	}

	t.Logf("expected: %v count: %v", len(expected), fc.Count())
	if int64(len(expected)) != fc.Count() {
		t.FailNow()
	}
}

func TestFileChunk(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	chunkExpected := []string{
		"9cedd6b972fca97210caef7280f2bf95",
		"715ec6d615393099d304f3e5924fd251",
		"0534503e8a79a27648ece0c31870c5c9",
		"52308c378552f0b8a68af2405955eccd",
		"932ee035c9d8ddfe47a759bb24743cf6",
		"e920223b2aee2b73c3df97bfd8e348c4",
		"e4548974fbfcfb533ca05d950d54419e",
	}

	var base = "/tmp/test"

	var filename = filepath.Join(base, "7G.dat")
	var chunksize int64 = 1024 * 1024 * 1024 // 1GB
	fc := NewFileChunk(filename, chunksize)

	if err := fc.Open(); err != nil {
		t.Log(err)
		t.FailNow()
	}
	defer fc.Close()

	// compute checksume
	go fc.MapAsync(func(idx int, rd *ChunkReader) error {
		b64, hex, err := rd.MD5()
		t.Logf("chunk: %v md5: %s  %s error: %v", idx, b64, hex, err)
		if err != nil || hex != chunkExpected[idx] {
			t.FailNow()
		}
		return nil
	})

	// split file
	// report overall progress
	go func() {
		size := fc.Size()
		percent := func(n int64) float64 {
			return (float64(n) / float64(size)) * 100
		}
		for {
			n := fc.Count()
			fmt.Printf("****** total: %v read: %v progress: %.2f\n", size, n, percent(n))
			time.Sleep(1 * time.Second)
		}
	}()

	// report file checksum
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b64, hex, err := fc.MD5()
		t.Logf("file: %s base64: %v hex: %v %v\n", fc.Filename(), b64, hex, err)
		if err != nil {
			t.FailNow()
		}
	}()
	// save the chunk in a file
	fc.MapAsync(func(idx int, reader *ChunkReader) error {
		var fp = filepath.Join(base, "out", "chunk_7G_"+strconv.Itoa(idx))
		f, err := os.Create(fp)
		if err != nil {
			t.Logf("%s error: %v", fp, err)
			t.FailNow()
		}
		reader.CopyTo(f)
		f.Close()

		//verify checksum
		_, hex, err := MD5SumFile(fp)
		t.Logf("file: %s hex: %v", fp, hex)
		if hex != chunkExpected[idx] {
			t.FailNow()
		}
		return nil
	})

	wg.Wait()
}
