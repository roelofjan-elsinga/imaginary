package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"io"
	"net/http"
	"os"
)

const ImageSourceTypeS3 ImageSourceType = "s3"
const PathQueryKey = "s3path"

type S3ImageSource struct {
	Config    *SourceConfig
	s3        *s3.S3
	awsBucket string
}

func NewS3ImageSource() func(config *SourceConfig) ImageSource {

	awsBucket := os.Getenv("AWS_BUCKET")
	awsRegion := os.Getenv("AWS_DEFAULT_REGION")

	return func(config *SourceConfig) ImageSource {

		s3Session := session.Must(session.NewSession(&aws.Config{
			Region:      aws.String(awsRegion),
			Credentials: credentials.NewEnvCredentials(),
		}))

		return &S3ImageSource{
			Config:    config,
			s3:        s3.New(s3Session),
			awsBucket: awsBucket,
		}
	}

}

func (s *S3ImageSource) Matches(r *http.Request) bool {
	return r.Method == http.MethodGet && r.URL.Query().Get(PathQueryKey) != ""
}

func (s *S3ImageSource) GetImage(req *http.Request) ([]byte, error) {
	return s.fetchImage(req.URL.Query().Get(PathQueryKey))
}

func (s *S3ImageSource) fetchImage(path string) ([]byte, error) {

	// Check if image exists
	if s.Config.MaxAllowedSize > 0 {
		headObject, err := s.s3.HeadObject(&s3.HeadObjectInput{
			Bucket: aws.String(s.awsBucket),
			Key:    aws.String(path),
		})

		if err != nil {
			return nil, fmt.Errorf("error fetching remote http image headers: %v", err)
		}

		if headObject.ContentLength == nil {
			return nil, NewError(fmt.Sprintf("no image found (key=%s)", path), 500)
		}

		contentLength := int(*headObject.ContentLength)

		if contentLength > s.Config.MaxAllowedSize {
			return nil, fmt.Errorf("Content-Length %d exceeds maximum allowed %d bytes", contentLength, s.Config.MaxAllowedSize)
		}
	}

	// Fetch the file

	getObject, err := s.s3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.awsBucket),
		Key:    aws.String(path),
	})

	if err != nil {
		return nil, fmt.Errorf("error fetching remote http image: %v", err)
	}

	// Read the body
	buf, err := io.ReadAll(getObject.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to create image from response body: %s (url=%s)", err, path)
	}
	return buf, nil
}

func init() {
	RegisterSource(ImageSourceTypeS3, NewS3ImageSource())
}
