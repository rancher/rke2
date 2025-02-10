package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/sirupsen/logrus"
)

const bucketName = "e2e-results-log"

var fileName string

func main() {
	flag.StringVar(&fileName, "f", "", "path to the go test json logs file")
	flag.Parse()

	if fileName == "" {
		log.Fatal("--f flag is required")
	}

	logFile, err := readLogsFromFile(fileName)
	if err != nil {
		log.Fatalf("Error reading log file: %v", err)
	}
	defer logFile.Close()

	if err = uploadReport(logFile); err != nil {
		log.Fatalf("Error uploading report: %v", err)
	}
}

func uploadReport(file *os.File) error {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-2"),
	})
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %w", err)
	}

	s3Client := s3.New(sess)
	params := &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(file.Name()),
		ContentType: aws.String("text/plain"),
		Body:        file,
	}

	_, err = s3Client.PutObject(params)
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	logrus.Infof("Successfully uploaded %s to S3\n", file.Name())

	return nil
}

func readLogsFromFile(fileName string) (*os.File, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}

	return file, nil
}
