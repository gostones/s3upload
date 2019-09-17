package main

import (
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/gostones/s3upload/internal"
	. "github.com/gostones/s3upload/internal/types"
	"net/http"
	"strconv"
)

type MultipartUploader struct {
	c  *resty.Client
	fc *internal.FileChunk

	uploadID string
}

func NewMultipartUploader(baseURL string, filename string, chunksize int64) *MultipartUploader {
	return &MultipartUploader{
		c:  resty.New().SetHostURL(baseURL),
		fc: internal.NewFileChunk(filename, chunksize),
	}
}

// startUpload obtains an uploadId generated in the backend
// server by the AWS S3 SDK. This uploadId will be used subsequently for uploading
// the individual chunks of the selectedFile.
func (r *MultipartUploader) startUpload() error {
	if err := r.fc.Open(); err != nil {
		return err
	}

	var result StartUploadResponse
	if resp, err := r.c.R().
		SetQueryParams(map[string]string{
			"fileName": r.fc.Name(),
			"fileType": r.fc.ContentType(),
		}).
		SetHeader("Accept", "application/json").
		SetResult(&result).
		Get("/start-upload"); err != nil || resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("%v %v %v", resp.StatusCode(), resp, err)
	}

	r.uploadID = result.UploadID
	fmt.Printf("response: %v\n", result)

	if err := r.uploadMultipartFile(); err != nil {
		return err
	}
	return nil
}

// uploadMultipartFile function splits the selectedFile into chunks
// and does the following:
// (1) call the backend server for a presigned url for each part,
// (2) uploads them, and
// (3) upon completion of all responses, sends a completeMultipartUpload call to the backend server.
func (r *MultipartUploader) uploadMultipartFile() error {
	var filename = r.fc.Name()
	var parts = make([]CompleteUploadPart, r.fc.Chunk())

	fn := func(idx int, reader *internal.ChunkReader) error {
		partNo := idx + 1

		// (1) Generate presigned URL for each part
		var getUploadURLResp GetUploadURLResponse
		md5, _, _ := reader.MD5()
		if _, err := r.c.R().
			SetQueryParams(map[string]string{
				"fileName":   filename,
				"partNumber": strconv.Itoa(partNo),
				"uploadId":   r.uploadID,
				"md5":        md5,
			}).
			SetHeader("Accept", "application/json").
			SetResult(&getUploadURLResp).
			Get("/get-upload-url"); err != nil {
			return err
		}

		presignedURL := getUploadURLResp.PresignedURL
		fmt.Printf("chunk: %v contentType: %v Presigned URL: %v\n", idx, r.fc.ContentType(), presignedURL)

		// (2) Puts each file part into the storage server
		uploadReq, err := http.NewRequest("PUT", presignedURL, reader)
		if err != nil {
			return err
		}

		uploadReq.Header.Set("Content-Type", r.fc.ContentType())
		uploadReq.Header.Set("Accept", "application/json")

		// b64, _, err := reader.MD5()
		// if err != nil {
		// 	return err
		// }
		// uploadReq.Header.Set("Content-MD5", b64)

		uploadReq.ContentLength = reader.Size()

		fmt.Println("request:", uploadReq)
		uploadResp, err := http.DefaultClient.Do(uploadReq)
		//fmt.Println(uploadResp)
		if err != nil {
			return err
		}

		if uploadResp.StatusCode != http.StatusOK {
			return fmt.Errorf("part: %v %v", partNo, uploadResp)
		}
		etag := uploadResp.Header.Get("ETag")

		parts[idx] = CompleteUploadPart{
			ETag:       etag,
			PartNumber: int64(partNo),
		}
		return nil
	}

	// TODO retry?
	errs := r.fc.Map(fn)
	fmt.Println("errors:", errs)
	if err := checkError(errs); err != nil {
		return err
	}

	// (3) Calls the CompleteMultipartUpload endpoint in the backend server
	completeUploadReq := CompleteUploadRequest{
		Params: CompleteUploadParams{
			FileName: filename,
			Parts:    parts,
			UploadID: r.uploadID,
		},
	}

	var completeUploadResp CompleteUploadResponse
	if _, err := r.c.R().
		SetBody(completeUploadReq).
		SetHeader("Accept", "application/json").
		SetResult(&completeUploadResp).
		Post("/complete-upload"); err != nil {
		return err
	}

	fmt.Println(completeUploadResp)
	return nil
}

func checkError(errs []error) error {
	var sa []string
	for _, err := range errs {
		if err != nil {
			sa = append(sa, err.Error())
		}
	}
	if len(sa) > 0 {
		return fmt.Errorf("%v", sa)
	}
	return nil
}

func main() {
	var chunksize int64 = 10000000 // 10MB
	var filename = "local/35MBb.raw"

	mpu := NewMultipartUploader("http://localhost:4000", filename, chunksize)
	mpu.startUpload()
}
