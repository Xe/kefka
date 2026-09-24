package snapshot

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/spf13/pflag"
	"github.com/tigrisdata/storage-go"
	"mvdan.cc/sh/v3/interp"
	"github.com/Xe/kefka/command"
)

type Impl struct {
	Client *storage.Client
}

type snapshotInfo struct {
	Version      string    `json:"version"`
	Name         string    `json:"name"`
	CreationDate time.Time `json:"creation_date,omitzero"`
}

func parseSnapshotName(raw string) (version, name string) {
	version, rest, ok := strings.Cut(raw, ";")
	if !ok {
		return raw, ""
	}
	rest = strings.TrimSpace(rest)
	name = strings.TrimPrefix(rest, "name=")
	return version, name
}

func (i Impl) Exec(ctx context.Context, ec *command.ExecContext, args []string) error {
	stdout := ec.Stdout
	stderr := ec.Stderr

	fs := pflag.NewFlagSet("snapshot", pflag.ContinueOnError)

	fs.SetOutput(stderr)

	useJSON := fs.Bool("json", false, "If true, output data as JSON")
	listMode := fs.Bool("list", false, "If true, list all snapshots for the given bucket")
	name := fs.String("name", "", "The name of the snapshot to take (not eligible when using --list)")

	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: snapshot [flags] [bucket]")
		fmt.Fprintln(stderr, "      To list snapshots in the current bucket:")
		fmt.Fprintln(stderr, "        snapshot --list")
		fmt.Fprintln(stderr, "      To take a new snapshot in the current bucket:")
		fmt.Fprintln(stderr, "        snapshot --name \"Human readable name here\" ")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Environment Variables:")
		fmt.Fprintln(stderr, "      BUCKET_NAME: the Tigris bucket to operate against (set by the platform)")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Flags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, err)
		return interp.ExitStatus(2)
	}

	bucket := cmp.Or(fs.Arg(0), ec.Environ.Get("BUCKET_NAME").String())

	if bucket == "" {
		fmt.Fprintln(stderr, "BUCKET_NAME not set and bucket not passed as argument")
		fs.Usage()
		return interp.ExitStatus(2)
	}

	switch *listMode {
	case true:
		snapshots, err := i.Client.ListBucketSnapshots(ctx, bucket)
		if err != nil {
			fmt.Fprintf(stderr, "error listing snapshots for bucket %q: %v", bucket, err)
			return interp.ExitStatus(2)
		}

		if *useJSON {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			enc.Encode(snapshots)
			return nil
		}

		tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "VERSION\tNAME\tCREATED")
		for _, b := range snapshots.Buckets {
			var raw string
			if b.Name != nil {
				raw = *b.Name
			}
			version, snapName := parseSnapshotName(raw)
			created := ""
			if b.CreationDate != nil {
				created = b.CreationDate.UTC().Format(time.RFC3339)
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", version, snapName, created)
		}
		tw.Flush()
	case false:
		if *name == "" {
			fmt.Fprintln(stderr, "when creating a snapshot, --name must be set")
			fs.Usage()
			return interp.ExitStatus(2)
		}

		resp, err := i.Client.CreateBucketSnapshot(ctx, *name, &s3.CreateBucketInput{
			Bucket: new(bucket),
		}, func(o *s3.Options) {
			o.APIOptions = append(o.APIOptions, awsmiddleware.AddRawResponseToMetadata)
		})
		if err != nil {
			fmt.Fprintf(stderr, "error creating snapshot for bucket %q: %v", bucket, err)
			return interp.ExitStatus(2)
		}

		var version string
		if rawResp, ok := awsmiddleware.GetRawResponse(resp.ResultMetadata).(*smithyhttp.Response); ok && rawResp != nil {
			version = rawResp.Header.Get("X-Tigris-Snapshot-Version")
		}

		info := snapshotInfo{
			Version: version,
			Name:    *name,
		}

		if *useJSON {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			enc.Encode(info)
			return nil
		}

		tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "VERSION\tNAME")
		fmt.Fprintf(tw, "%s\t%s\n", info.Version, info.Name)
		tw.Flush()
	}

	return nil
}
