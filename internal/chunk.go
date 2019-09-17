package internal

import (
	"bufio"
	"io"
	"os"
	"sync"
)

type FileChunk struct {
	filename  string
	chunksize int64

	file        *os.File
	name        string
	contentType string
	chunk       int     // number of chunk
	size        int64   // file size
	count       Counter // bytes read
}

func NewFileChunk(filename string, chunksize int64) *FileChunk {
	return &FileChunk{
		filename:  filename,
		chunksize: chunksize,
	}
}

func (r *FileChunk) Open() error {
	file, err := os.Open(r.filename)
	if err != nil {
		return err
	}
	fi, err := file.Stat()
	if err != nil {
		return err
	}
	size := fi.Size()
	name := fi.Name()
	chunk := int(size / r.chunksize)
	if size%r.chunksize != 0 {
		chunk++
	}
	contentType, _ := ContentType(file)
	file.Seek(0, io.SeekStart)

	r.file = file
	r.name = name
	r.size = size
	r.chunk = chunk
	r.contentType = contentType
	return nil
}

// Map ranges over the chunks calling the function with the chunk number and a reader.
// Fail fast if the function returns error.
func (r *FileChunk) Map(fn func(int, *ChunkReader) error) []error {
	readers := r.Readers()
	n := len(readers)
	var errs = make([]error, n)
	for i, rd := range readers {
		if err := fn(i, rd); err != nil {
			errs[i] = err
			break
		}
	}
	return errs
}

// MapAsync ranges over the chunks calling the function in separate go routines with the chunk number and a reader.
func (r *FileChunk) MapAsync(fn func(int, *ChunkReader) error) []error {
	var wg sync.WaitGroup
	readers := r.Readers()
	n := len(readers)
	wg.Add(n)
	var errs = make([]error, n)
	for i, rd := range readers {
		go func(chunk int, reader *ChunkReader) {
			defer wg.Done()
			if err := fn(chunk, reader); err != nil {
				errs[chunk] = err
			}
		}(i, rd)
	}
	wg.Wait()

	return errs
}

func (r *FileChunk) Close() error {
	if r.file == nil {
		return os.ErrInvalid
	}
	return r.file.Close()
}

// Readers returns a list of reader for each chunk. It resets the counter for the bytes read of the file.
func (r *FileChunk) Readers() []*ChunkReader {
	r.count.Reset()
	readers := make([]*ChunkReader, r.chunk)
	for i := 0; i < r.chunk; i++ {
		off := int64(i) * r.chunksize
		limit := off + r.chunksize
		// adjust limit for last chunk
		if i == r.chunk-1 {
			limit = r.size
		}
		readers[i] = NewChunkReader(r, off, limit)
	}
	return readers
}

func (r *FileChunk) Filename() string {
	return r.filename
}

func (r *FileChunk) Chunksize() int64 {
	return r.chunksize
}

func (r *FileChunk) Name() string {
	return r.name
}

func (r *FileChunk) ContentType() string {
	return r.contentType
}

func (r *FileChunk) MD5() (string, string, error) {
	return MD5Sum(r.file)
}

func (r *FileChunk) Size() int64 {
	return r.size
}

func (r *FileChunk) Count() int64 {
	return r.count.Get()
}

func (r *FileChunk) Chunk() int {
	return r.chunk
}

type ChunkReader struct {
	reader   *FileChunk
	base     int64
	off      int64
	limit    int64
	counting bool
}

func NewChunkReader(r *FileChunk, off, limit int64) *ChunkReader {
	return &ChunkReader{
		reader:   r,
		base:     off,
		off:      off,
		limit:    limit,
		counting: true,
	}
}

func (r *ChunkReader) MD5() (string, string, error) {
	cr := &ChunkReader{
		reader:   r.reader,
		off:      r.off,
		limit:    r.limit,
		counting: false,
	}
	return MD5Sum(cr)
}

func (r *ChunkReader) read(p []byte) (int, error) {
	if r.off >= r.limit {
		return 0, io.EOF
	}
	if max := r.limit - r.off; int64(len(p)) > max {
		p = p[0:max]
	}
	n, err := r.reader.file.ReadAt(p, r.off)
	r.off += int64(n)
	return n, err
}

func (r *ChunkReader) Read(p []byte) (int, error) {
	n, err := r.read(p)
	if r.counting {
		r.reader.count.Increment(int64(n))
	}
	return n, err
}

func (r *ChunkReader) Reset() {
	r.reader.count.Decrement(r.off - r.base)
	r.off = r.base
}

func (r *ChunkReader) CopyTo(w io.Writer) {
	bw := bufio.NewWriter(w)
	io.Copy(bw, r)
	bw.Flush()
}

func (r *ChunkReader) Size() int64 {
	return r.limit - r.base
}
