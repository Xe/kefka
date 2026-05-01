package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
)

var BucketName string

func init() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
	BucketName = os.Getenv("BUCKET_NAME")
}

func main() {
	// fmt.Println(BucketName)

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}
	client := s3.NewFromConfig(cfg)

	s3fs, err := NewS3FS(client, BucketName)
	if err != nil {
		panic(err)
	}
	fmt.Printf("s3fs.Root() = %q\n", s3fs.Root())
	fmt.Println(s3fs.Join(s3fs.Root(), "hello/", "/"))

	files, err := s3fs.ReadDir("foo/")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Found %d files\n", len(files))
	for _, file := range files {
		fmt.Println(file.Name())
	}

}
