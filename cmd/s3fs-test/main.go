package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/pflag"
	"tangled.org/xeiaso.net/kefka/s3fs"

	_ "github.com/joho/godotenv/autoload"
)

var (
	bucket = pflag.String("bucket", os.Getenv("BUCKET_NAME"), "bucket to operate on")
)

func main() {
	pflag.Parse()
	fmt.Println("bucket:", *bucket)

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}
	client := s3.NewFromConfig(cfg)

	fsys, err := s3fs.NewS3FS(client, *bucket)
	if err != nil {
		panic(err)
	}

	stat := func(p string) {
		info, err := fsys.Stat(p)
		if err != nil {
			fmt.Printf("Stat(%q) -> err: %v (is fs.ErrNotExist=%v)\n", p, err, errors.Is(err, fs.ErrNotExist))
			return
		}
		fmt.Printf("Stat(%q) -> name=%q dir=%v size=%d mtime=%s\n",
			p, info.Name(), info.IsDir(), info.Size(), info.ModTime().Format("2006-01-02T15:04:05"))
	}

	readdir := func(p string) {
		entries, err := fsys.ReadDir(p)
		if err != nil {
			fmt.Printf("ReadDir(%q) -> err: %v\n", p, err)
			return
		}
		fmt.Printf("ReadDir(%q) -> %d entries:\n", p, len(entries))
		for _, e := range entries {
			fmt.Printf("  %s (dir=%v size=%d)\n", e.Name(), e.IsDir(), e.Size())
		}
	}

	stat(".")
	stat("etc")
	stat("moby-dick.txt")
	stat("etc/motd")
	stat("does-not-exist")

	readdir(".")
	readdir("etc")
}
