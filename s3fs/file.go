package s3fs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tigrisdata/storage-go"
	"go.uber.org/atomic"
)

const (
	ModeMultipartUpload os.FileMode = fs.ModePerm + 1 // Custom os.FileMode for S3 multipart upload
)

var (
	ErrLockNotSupported      = errors.New("lock not supported by s3")
	ErrTruncateNotSupported  = errors.New("truncate not supported by s3")
	ErrFileClosed            = errors.New("file is closed")
	ErrCantWriteToReadOnly   = errors.New("can't write to read-only file")
	ErrCantReadFromWriteOnly = errors.New("can't read from write-only file")
)

// s3ReadFile implements billy.File for S3, and represents a file opened in read mode.
//
// Upon creation, the file is loaded from S3.
type s3ReadFile struct {
	client *storage.Client // s3 skd client
	bucket string          // S3 bucket name
	key    string          // File object's key in S3
	closed bool            // Is the file closed?
	reader *bytes.Reader   // Buffer for file contents
}

// newS3ReadFile creates a new s3ReadFile.
func newS3ReadFile(client *storage.Client, bucket, key string) (*s3ReadFile, error) {
	// TODO: Check if the file exists
	// ...

	// Create the context
	ctx := context.TODO() // TODO: How can user-supplied contexts be supported?

	// Run the GetObject operation
	res, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to perform GetObject operation: %w", err)
	}

	// Read the file contents and store in a bytes reader
	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read file body: %w", err)
	}
	reader := bytes.NewReader(buf)

	// Return the file
	return &s3ReadFile{
		client: client,
		bucket: bucket,
		key:    key,
		reader: reader,
	}, nil
}

// Name returns the name of the file as presented to Open.
func (f *s3ReadFile) Name() string {
	return f.key
}

// Write implements os.Writer for billy.File
func (f *s3ReadFile) Write(p []byte) (n int, err error) {
	return 0, ErrCantWriteToReadOnly
}

// Read implements os.Reader for billy.File
func (f *s3ReadFile) Read(p []byte) (n int, err error) {
	return f.reader.Read(p)
}

// ReadAt implements io.ReaderAt for billy.File
func (f *s3ReadFile) ReadAt(p []byte, off int64) (n int, err error) {
	return f.reader.ReadAt(p, off)
}

// Seek implements io.Seeker for billy.File
func (f *s3ReadFile) Seek(offset int64, whence int) (int64, error) {
	return f.reader.Seek(offset, whence)
}

// Close implements io.Closer for billy.File
func (f *s3ReadFile) Close() error {
	// Was the file already closed?
	if f.closed {
		return ErrFileClosed
	}

	// Close the underlying file
	f.reader = nil

	// Mark the file as closed
	f.closed = true

	return nil
}

// Lock locks the file like e.g. flock. It protects against access from
// other processes.
func (f *s3ReadFile) Lock() error {
	return ErrLockNotSupported
}

// Unlock unlocks the file.
func (f *s3ReadFile) Unlock() error {
	return ErrLockNotSupported
}

// Truncate the file.
func (f *s3ReadFile) Truncate(size int64) error {
	return ErrTruncateNotSupported
}

// s3WriteFile stores a file opened in write mode and implements billy.File
//
// Upon creation, a buffer is created to store the file contents. Upon close,
// the file is uploaded to S3.
type s3WriteFile struct {
	client *storage.Client // s3 skd client
	bucket string          // S3 bucket name
	key    string          // File object's key in S3
	closed bool            // Is the file closed?
	buf    *bytes.Buffer   // Buffer for storing the file before it's uploaded
}

// newS3WriteFile creates a new s3ReadFile.
func newS3WriteFile(client *storage.Client, bucket, key string) (*s3WriteFile, error) {
	// TODO: Validate the key
	// ...

	return &s3WriteFile{
		client: client,
		bucket: bucket,
		key:    key,
		buf:    bytes.NewBuffer(nil),
	}, nil
}

// Name returns the name of the file as presented to Open.
func (f *s3WriteFile) Name() string {
	return f.key
}

// Write implements os.Writer for billy.File
func (f *s3WriteFile) Write(p []byte) (n int, err error) {
	if f.closed {
		return 0, ErrFileClosed
	}
	return f.buf.Write(p)
}

// Read implements os.Reader for billy.File
func (f *s3WriteFile) Read(p []byte) (n int, err error) {
	return 0, ErrCantReadFromWriteOnly
}

// ReadAt implements io.ReaderAt for billy.File
func (f *s3WriteFile) ReadAt(p []byte, off int64) (n int, err error) {
	return 0, ErrCantReadFromWriteOnly
}

// Seek implements io.Seeker for billy.File
func (f *s3WriteFile) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.New("not implemented")
}

// Close implements io.Closer for billy.File
func (f *s3WriteFile) Close() error {
	if f.closed {
		return ErrFileClosed
	}

	// Set to closed
	f.closed = true

	// Extract the body from the buffer
	body := bytes.NewReader(f.buf.Bytes())

	// Create the context
	ctx := context.TODO() // TODO: How can user-supplied contexts be supported?

	// Run the GetObject operation
	// TODO: Currently `res` is not used. Should it be?
	_, err := f.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &f.bucket,
		Key:    &f.key,
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("unable to perform GetObject operation: %w", err)
	}

	return nil
}

// Lock locks the file like e.g. flock. It protects against access from
// other processes.
func (f *s3WriteFile) Lock() error {
	return ErrLockNotSupported
}

// Unlock unlocks the file.
func (f *s3WriteFile) Unlock() error {
	return ErrLockNotSupported
}

// Truncate the file.
func (f *s3WriteFile) Truncate(size int64) error {
	return ErrTruncateNotSupported
}

// s3MultipartUploadFile implements billy.File
type s3MultipartUploadFile struct {
	client   *storage.Client // s3 skd client
	bucket   string          // S3 bucket name
	key      string          // File object's key in S3
	closed   bool            // Is the file closed?
	uploadID string          // S3 multipart upload ID
	uploadN  *atomic.Int32   // Counter tracking the number of uploads
}

// newS3MultipartUploadFile creates a new s3ReadFile.
func newS3MultipartUploadFile(client *storage.Client, bucket, key string) (*s3MultipartUploadFile, error) {
	// TODO: Check if the file exists
	// ...

	// Create the context
	ctx := context.TODO() // TODO: How can user-supplied contexts be supported?

	// Run the GetObject operation
	res, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create multipart upload: %w", err)
	}

	// Return the file
	return &s3MultipartUploadFile{
		client:   client,
		bucket:   bucket,
		key:      key,
		uploadID: *res.UploadId,
		uploadN:  atomic.NewInt32(1),
	}, nil
}

// Name returns the name of the file as presented to Open.
func (f *s3MultipartUploadFile) Name() string {
	return f.key
}

// Write implements os.Writer for billy.File
func (f *s3MultipartUploadFile) Write(p []byte) (n int, err error) {
	// Get the size of the data being written
	n = len(p)

	// Create a context for the operation
	ctx := context.TODO() // TODO: How can user-supplied contexts be supported?

	// Create a reader for the data
	r := bytes.NewReader(p)

	// Get the part number
	pn := f.uploadN.Load()

	// Run the UploadPart operation
	_, err = f.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     &f.bucket,
		Key:        &f.key,
		UploadId:   &f.uploadID,
		PartNumber: new(pn),
		Body:       r,
	})
	if err != nil {
		return 0, fmt.Errorf("unable to upload part %d: %w", pn, err)
	}

	// Increment the part number
	f.uploadN.Add(1)

	// Return the number of bytes written
	return n, nil
}

// Read implements os.Reader for billy.File
func (f *s3MultipartUploadFile) Read(p []byte) (n int, err error) {
	return 0, ErrCantReadFromWriteOnly
}

// ReadAt implements io.ReaderAt for billy.File
func (f *s3MultipartUploadFile) ReadAt(p []byte, off int64) (n int, err error) {
	return 0, ErrCantReadFromWriteOnly
}

// Seek implements io.Seeker for billy.File
func (f *s3MultipartUploadFile) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.New("seek not implemented")
}

// Close implements io.Closer for billy.File
func (f *s3MultipartUploadFile) Close() error {
	// Check if the file has been closed
	if f.closed {
		return ErrFileClosed
	}

	// Set to closed
	f.closed = true

	// Create the context
	ctx := context.TODO() // TODO: How can user-supplied contexts be supported?

	// Complete the multipart upload
	// TODO: Currently `res` is not used. Should it be?
	_, err := f.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   &f.bucket,
		Key:      &f.key,
		UploadId: &f.uploadID,
	})
	if err != nil {
		return fmt.Errorf("unable to complete multipart upload: %w", err)
	}

	return nil
}

// Lock locks the file like e.g. flock. It protects against access from
// other processes.
func (f *s3MultipartUploadFile) Lock() error {
	return ErrLockNotSupported
}

// Unlock unlocks the file.
func (f *s3MultipartUploadFile) Unlock() error {
	return ErrLockNotSupported
}

// Truncate the file.
func (f *s3MultipartUploadFile) Truncate(size int64) error {
	return ErrTruncateNotSupported
}

// s3DirFile is a billy.File handle for a directory in S3. S3 has no real
// directories, but WASI guests (via wazero) open the preopen root by calling
// OpenFile(".", O_RDONLY) and then asking IsDir() — so we return a pseudo-file
// that reports as a directory and rejects byte I/O with EISDIR.
type s3DirFile struct {
	name   string
	closed bool
}

func newS3DirFile(name string) *s3DirFile {
	return &s3DirFile{name: name}
}

// Name returns the name of the file as presented to Open.
func (f *s3DirFile) Name() string {
	return f.name
}

func (f *s3DirFile) eisdir(op string) error {
	return &os.PathError{Op: op, Path: f.name, Err: syscall.EISDIR}
}

func (f *s3DirFile) Read(p []byte) (int, error)              { return 0, f.eisdir("read") }
func (f *s3DirFile) ReadAt(p []byte, off int64) (int, error) { return 0, f.eisdir("read") }
func (f *s3DirFile) Write(p []byte) (int, error)             { return 0, f.eisdir("write") }
func (f *s3DirFile) Seek(offset int64, whence int) (int64, error) {
	return 0, f.eisdir("seek")
}
func (f *s3DirFile) Truncate(size int64) error { return f.eisdir("truncate") }

func (f *s3DirFile) Close() error {
	if f.closed {
		return ErrFileClosed
	}
	f.closed = true
	return nil
}

func (f *s3DirFile) Lock() error   { return ErrLockNotSupported }
func (f *s3DirFile) Unlock() error { return ErrLockNotSupported }
