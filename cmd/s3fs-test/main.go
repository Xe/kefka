package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strconv"

	"github.com/spf13/pflag"
	"github.com/tigrisdata/storage-go"
	"tangled.org/xeiaso.net/kefka/s3fs"
	"tangled.org/xeiaso.net/kefka/s3fs/unixmeta"

	_ "github.com/joho/godotenv/autoload"
)

var (
	bucket = pflag.String("bucket", os.Getenv("BUCKET_NAME"), "bucket to operate on")

	fsUnixMetadata = pflag.Bool("fs-unix-metadata", false, "store/read POSIX file attributes as S3 user metadata")
	fsUser         = pflag.String("fs-user", "0", "owner (name or numeric uid) for written files")
	fsGroup        = pflag.String("fs-group", "0", "group (name or numeric gid) for written files")
	fsUmask        = pflag.String("fs-umask", "022", "octal umask applied to new files")
)

func main() {
	pflag.Parse()
	fmt.Println("bucket:", *bucket)

	client, err := storage.New(context.Background())
	if err != nil {
		log.Fatal(fmt.Errorf("can't make storage client: %w", err))
	}

	var opts []s3fs.Option
	if *fsUnixMetadata {
		uid, err := unixmeta.LookupUID(*fsUser)
		if err != nil {
			log.Fatal(fmt.Errorf("--fs-user %q: %w", *fsUser, err))
		}
		gid, err := unixmeta.LookupGID(*fsGroup)
		if err != nil {
			log.Fatal(fmt.Errorf("--fs-group %q: %w", *fsGroup, err))
		}
		umask, err := strconv.ParseUint(*fsUmask, 8, 32)
		if err != nil {
			log.Fatal(fmt.Errorf("--fs-umask %q: must be octal: %w", *fsUmask, err))
		}
		opts = append(opts, s3fs.WithUnixMetadata(uid, gid, os.FileMode(umask)))
	}

	fsys, err := s3fs.NewS3FS(client, *bucket, opts...)
	if err != nil {
		panic(err)
	}

	stat := func(p string) {
		info, err := fsys.Stat(p)
		if err != nil {
			fmt.Printf("Stat(%q) -> err: %v (is fs.ErrNotExist=%v)\n", p, err, errors.Is(err, fs.ErrNotExist))
			return
		}
		owner := ""
		if st, ok := info.Sys().(*s3fs.FileStat); ok {
			owner = fmt.Sprintf(" uid=%d gid=%d", st.UID, st.GID)
		}
		fmt.Printf("Stat(%q) -> name=%q dir=%v mode=%s size=%d mtime=%s%s\n",
			p, info.Name(), info.IsDir(), info.Mode(), info.Size(), info.ModTime().Format("2006-01-02T15:04:05"), owner)
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
