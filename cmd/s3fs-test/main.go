package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/pflag"
	"tangled.org/xeiaso.net/kefka/internal/s3fs"

	_ "github.com/joho/godotenv/autoload"
)

var (
	bucket = pflag.String("bucket", os.Getenv("BUCKET_NAME"), "bucket to operate on")
)

func main() {
	pflag.Parse()

	fmt.Println(*bucket)

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}
	client := s3.NewFromConfig(cfg)

	s3fs, err := s3fs.NewS3FS(client, *bucket)
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
