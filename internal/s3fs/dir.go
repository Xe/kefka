// dir.go implements the interface billy.Dir

package main

import (
	"context"
	"errors"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ReadDir reads the directory named by dirname and returns a list of
// directory entries sorted by filename.
func (fs3 *S3FS) ReadDir(path string) ([]os.FileInfo, error) {
	// p := fs3.cleanPath(fs3.root, path)
	// if p != "" {
	// 	p += "/"
	// }
	// fmt.Println("ReadDir:", p)
	p := path

	// Create a context with a timeout
	ctx := context.TODO() // TODO: Get user context?

	var ct *string
	var dirs []os.FileInfo
	var files []os.FileInfo
	for {
		res, err := fs3.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            &fs3.bucket,
			Prefix:            &p,
			ContinuationToken: ct,
			Delimiter:         &fs3.separator,
		})
		if err != nil {
			return nil, err
		}

		// Add the directories to the list
		for _, d := range res.CommonPrefixes {
			dirs = append(dirs, newDirInfo(*d.Prefix))
		}

		// Add the files to the list
		for _, f := range res.Contents {
			files = append(files, newFileInfo(
				aws.ToString(f.Key),
				*f.Size,
				aws.ToTime(f.LastModified),
			))
		}

		// Set the last key
		ct = res.NextContinuationToken

		// If there are no more keys, break
		if !*res.IsTruncated {
			break
		}
	}

	// Join the directories and files & return
	res := append(dirs, files...)
	return res, nil
}

// MkdirAll creates a directory named path, along with any necessary
// parents, and returns nil, or else returns an error. The permission bits
// perm are used for all directories that MkdirAll creates. If path is/
// already a directory, MkdirAll does nothing and returns nil.
func (fs3 *S3FS) MkdirAll(filename string, perm os.FileMode) error {
	return errors.New("not implemented")
}
