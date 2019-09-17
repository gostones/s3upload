package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gorilla/mux"
)

const port = 4000

// AWS credentials are read from environment varilables
var accessKeyID = ""     // Set AWS_ACCESS_KEY_ID env with your access key id
var secretAccessKey = "" // Set AWS_SECRET_ACCESS_KEY env with your secret access key

var bucketName = "" // Set AWS_BUCKET_NAME env with your S3 bucket name

// Change endpoint/region to your region
const (
	endpoint         = "http://s3-us-west-2.amazonaws.com"
	region           = "us-west-2"
	s3ForcePathStyle = true
	signatureVersion = "v4"
)

type startUploadRequest struct {
	FileName string `json:"fileName"`
	FileType string `json:"fileType"`
}

func parseStartUploadRequest(r *http.Request) *startUploadRequest {
	q := r.URL.Query()
	return &startUploadRequest{
		FileName: q.Get("fileName"),
		FileType: q.Get("fileType"),
	}
}

type startUploadResponse struct {
	UploadID string `json:"uploadId"`
}

func writeStartUploadResponse(w http.ResponseWriter, uploadId string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&startUploadResponse{
		UploadID: uploadId,
	})
}

type getUploadRequest struct {
	FileName   string `json:"fileName"`
	PartNumber string `json:"partNumber"`
	UploadID   string `json:"uploadId"`
	MD5        string `json:"md5"`
}

func parseGetUploadRequest(r *http.Request) *getUploadRequest {
	q := r.URL.Query()
	return &getUploadRequest{
		FileName:   q.Get("fileName"),
		PartNumber: q.Get("partNumber"),
		UploadID:   q.Get("uploadId"),
		MD5:        q.Get("md5"),
	}
}

type getUploadResponse struct {
	PresignedURL string `json:"presignedUrl"`
}

func writeGetUploadResponse(w http.ResponseWriter, pURL string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&getUploadResponse{
		PresignedURL: pURL,
	})
}

type completeUploadRequest struct {
	Params completeUploadParams `json:"params"`
}

type completeUploadParams struct {
	FileName string               `json:"fileName"`
	Parts    []completeUploadPart `json:"parts"`
	UploadID string               `json:"uploadId"`
}

type completeUploadPart struct {
	ETag       string
	PartNumber int64
}

func parseCompleteUploadRequest(r *http.Request) *completeUploadRequest {
	var j completeUploadRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&j)
	if err != nil {
		return nil
	}
	return &j
}

type completeUploadResponse struct {
	Data completeUploadData `json:"data"`
}

type completeUploadData struct {
	Location string
	Bucket   string
	Key      string
	ETag     string
}

func writeCompleteUploadResponse(w http.ResponseWriter, location, bucket, key, etag string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&completeUploadResponse{
		Data: completeUploadData{
			Location: location,
			Bucket:   bucket,
			Key:      key,
			ETag:     etag,
		},
	})
}

func credentailFromEnv() {
	accessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	secretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	bucketName = os.Getenv("AWS_BUCKET_NAME")
}

func config() *aws.Config {
	credentailFromEnv()

	creds := credentials.NewStaticCredentials(accessKeyID, secretAccessKey, "")
	_, err := creds.Get()
	if err != nil {
		fmt.Printf("bad credentials: %s", err)
	}
	cfg := aws.Config{
		Credentials:      creds,
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(region),
		S3ForcePathStyle: aws.Bool(s3ForcePathStyle),
	}
	return &cfg
}

// PutObjectInput represents s3.PutObjectInput with additional fields
// required for mutlipart upload with presigned URL
type PutObjectInput struct {
	Bucket     string
	Key        string
	UploadID   string
	PartNumber string
	MD5        string // base64 coded MD5 checksum
}

// PutObjectRequest generates a "aws/request.Request" representing the
// client's request for the PutObject operation.
// This is to workaround s3.PutObjectRequest where multipart upload is not supported
// for presigned URL.
func PutObjectRequest(svc *s3.S3, input *PutObjectInput) (*request.Request, *s3.PutObjectOutput) {
	const opPutObject = "PutObject"
	op := &request.Operation{
		Name:       opPutObject,
		HTTPMethod: "PUT",
		// HTTPPath:   "/{Bucket}/{Key+}",
		HTTPPath: fmt.Sprintf("/{Bucket}/{Key+}?uploadId=%s&partNumber=%s", input.UploadID, input.PartNumber),
	}
	output := &s3.PutObjectOutput{}
	req := svc.NewRequest(op, &s3.PutObjectInput{
		Bucket: aws.String(input.Bucket),
		Key:    aws.String(input.Key),
	}, output)

	return req, output
}

// Presign returns the signed URL for the input.
func Presign(svc *s3.S3, input *PutObjectInput, expire time.Duration) (string, error) {
	req, _ := PutObjectRequest(svc, input)
	if input.MD5 != "" {
		req.HTTPRequest.Header.Set("Content-MD5", input.MD5)
	}
	url, err := req.Presign(expire)
	if err != nil {
		return "", err
	}

	return url, nil
}

func main() {
	cfg := config()
	svc := s3.New(session.New(), cfg)

	//
	r := mux.NewRouter()

	r.HandleFunc("/start-upload", func(w http.ResponseWriter, r *http.Request) {
		q := parseStartUploadRequest(r)
		log.Printf("start-upload request: %v\n", q)
		input := &s3.CreateMultipartUploadInput{
			Bucket:      aws.String(bucketName),
			Key:         aws.String(q.FileName),
			ContentType: aws.String(q.FileType),
		}
		output, err := svc.CreateMultipartUpload(input)
		if err != nil {
			log.Println(err)
			// no access to original status code, return 500
			http.Error(w, fmt.Sprintf("can't obtain upload id: %v", err), http.StatusInternalServerError)
			return
		}
		uploadID := *output.UploadId
		log.Printf("start-upload uploadID: %v\n", uploadID)
		writeStartUploadResponse(w, uploadID)
	})

	r.HandleFunc("/get-upload-url", func(w http.ResponseWriter, r *http.Request) {
		q := parseGetUploadRequest(r)
		log.Printf("get-upload-url request: %v\n", q)
		input := &PutObjectInput{
			Bucket:     bucketName,
			Key:        q.FileName,
			UploadID:   q.UploadID,
			PartNumber: q.PartNumber,
			MD5:        q.MD5,
		}
		u, err := Presign(svc, input, time.Minute*15)
		if err != nil {
			fmt.Fprintf(w, "can't presign url %s\n", err.Error())
			return
		}
		log.Printf("get-upload-url url: %v\n", u)
		writeGetUploadResponse(w, u)
	})

	r.HandleFunc("/complete-upload", func(w http.ResponseWriter, r *http.Request) {
		q := parseCompleteUploadRequest(r)
		log.Printf("complete-upload request: %v\n", q)

		var completedParts []*s3.CompletedPart
		for _, p := range q.Params.Parts {
			completedParts = append(completedParts, &s3.CompletedPart{
				ETag:       aws.String(p.ETag),
				PartNumber: aws.Int64(p.PartNumber),
			})
		}
		input := &s3.CompleteMultipartUploadInput{
			Bucket:   aws.String(bucketName),
			Key:      aws.String(q.Params.FileName),
			UploadId: aws.String(q.Params.UploadID),
			MultipartUpload: &s3.CompletedMultipartUpload{
				Parts: completedParts,
			},
		}
		output, err := svc.CompleteMultipartUpload(input)
		if err != nil {
			fmt.Fprintf(w, "can't complete upload %s\n", err.Error())
			return
		}
		log.Printf("complete-upload request: %v\n", *output)

		writeCompleteUploadResponse(w, *output.Location, *output.Bucket, *output.Key, *output.ETag)
	})

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	})

	hostport := fmt.Sprintf(":%v", port)
	fmt.Println("listening on port", hostport)
	if err := http.ListenAndServe(hostport, r); err != nil {
		panic(err)
	}
}
