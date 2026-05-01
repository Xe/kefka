// basic.go implements the interface billy.Basic

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-git/go-billy/v5"
)

const (
	O_RDONLY      int = os.O_RDONLY // open the file read-only.
	O_WRONLY      int = os.O_WRONLY // open the file write-only.
	O_WRMULTIPART int = 0x4         // open the file for write-only using multipart upload.

	SupportedOFlags = O_RDONLY | O_WRONLY | O_WRMULTIPART // supported open flags for s3fs
)

var (
	ErrOpenFlagNotSupported = errors.New("open flag not supported")
)

// Create implements billy.Basic
// Create creates the named file with mode 0666 (before umask), truncating
// it if it already exists. If successful, methods on the returned File can
// be used for I/O; the associated file descriptor has mode O_RDWR.
func (fs3 *S3FS) Create(filename string) (billy.File, error) {
	return fs3.OpenFile(filename, O_WRONLY, 0666)
}

// Open opens the named file for reading. If successful, methods on the
// returned file can be used for reading; the associated file descriptor has
// mode O_RDONLY.
func (fs3 *S3FS) Open(filename string) (billy.File, error) {
	return fs3.OpenFile(filename, O_RDONLY, 0666)
}

// OpenFile is the generalized open call; most users will use Open or Create
// instead. It opens the named file with specified flag (O_RDONLY etc.) and
// perm, (0666 etc.) if applicable. If successful, methods on the returned
// File can be used for I/O.
func (fs3 *S3FS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	// Is the supplied flag supported?
	if flag&SupportedOFlags != flag {
		return nil, errors.New("unsupported open flag")
	}

	// Get the file path
	p := path.Join(fs3.root, filename)

	switch flag & SupportedOFlags {
	case O_RDONLY:
		return newS3ReadFile(fs3.client, fs3.bucket, p)

	case O_WRONLY:
		return newS3WriteFile(fs3.client, fs3.bucket, p)

	case O_WRMULTIPART:
		return newS3MultipartUploadFile(fs3.client, fs3.bucket, p)

	default:
		return nil, errors.New("unsupported open flag")
	}
}

// Stat returns a FileInfo describing the named file.
func (fs3 *S3FS) Stat(filename string) (os.FileInfo, error) {
	return nil, errors.New("not implemented")
}

// Rename renames (moves) oldpath to newpath. If newpath already exists and
// is not a directory, Rename replaces it. OS-specific restrictions may
// apply when oldpath and newpath are in different directories.
func (fs3 *S3FS) Rename(oldpath, newpath string) error {
	// TODO: Validate the paths?

	// Create a context
	ctx := context.TODO() // TODO: Get user-supplied context?

	// Format the paths
	src := path.Join(fs3.root, oldpath)
	dst := path.Join(fs3.root, newpath)

	// Send the copy request
	_, err := fs3.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     &fs3.bucket,
		CopySource: &src,
		Key:        &dst,
	})
	if err != nil {
		return fmt.Errorf("failed to rename file: %s", err)
	}

	// Delete the old file
	// TODO: Parse the response?
	_, err = fs3.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &fs3.bucket,
		Key:    &src,
	})
	if err != nil {
		return fmt.Errorf("failed to remove file: %s", err)
	}

	return nil
}

// Remove removes the named file or directory.
func (fs3 *S3FS) Remove(filename string) error {
	// TODO: Validate the path?
	// ...

	// Create a context
	ctx := context.TODO() // TODO: Get user-supplied context?

	// Format the path
	p := path.Join(fs3.root, filename)

	// Send the request
	// TODO: Parse the response?
	_, err := fs3.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &fs3.bucket,
		Key:    &p,
	})
	if err != nil {
		return fmt.Errorf("failed to remove file: %s", err)
	}
	return nil
}

// Join joins any number of path elements into a single path
func (fs3 *S3FS) Join(elem ...string) string {
	j := path.Join(elem...)
	c := path.Clean(j)
	return c
}
