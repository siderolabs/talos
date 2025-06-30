// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"github.com/siderolabs/go-retry/retry"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCPUploder registers the image in GCP.
type GCPUploder struct {
	Options Options

	storageClient  *storage.Client
	computeService *compute.Service
	projectID      string

	imagePath string
}

// NewGCPUploder creates a new GCPUploder.
func NewGCPUploder(options Options) (*GCPUploder, error) {
	projectID := os.Getenv("GOOGLE_PROJECT_ID")
	credentials := os.Getenv("GOOGLE_CREDENTIALS")

	if projectID == "" {
		return nil, fmt.Errorf("gcp: GOOGLE_PROJECT_ID is not set")
	}

	if credentials == "" {
		return nil, fmt.Errorf("gcp: GOOGLE_CREDENTIALS is not set")
	}

	gcpUploader := &GCPUploder{
		Options: options,
	}

	gcpUploader.projectID = projectID

	var err error

	gcpUploader.storageClient, err = storage.NewClient(context.Background(), option.WithCredentialsJSON([]byte(credentials)))
	if err != nil {
		return nil, fmt.Errorf("gcp: failed to create google storage client: %w", err)
	}

	gcpUploader.computeService, err = compute.NewService(context.Background(), option.WithCredentialsJSON([]byte(credentials)))
	if err != nil {
		return nil, fmt.Errorf("gcp: failed to create google compute service: %w", err)
	}

	return gcpUploader, nil
}

// Upload uploads the image to GCP.
func (u *GCPUploder) Upload(ctx context.Context) error {
	bucketName := fmt.Sprintf("talos-image-upload-%s", uuid.New())

	bucketHandle := u.storageClient.Bucket(bucketName)

	if err := bucketHandle.Create(ctx, u.projectID, &storage.BucketAttrs{
		PublicAccessPrevention: storage.PublicAccessPreventionEnforced,
	}); err != nil {
		return fmt.Errorf("gcp: failed to create bucket %s: %w", bucketName, err)
	}

	log.Println("gcp: created bucket", bucketName)

	defer func() {
		objects := bucketHandle.Objects(ctx, nil)

		for {
			objAttr, err := objects.Next()
			if errors.Is(err, iterator.Done) {
				break
			}

			if err != nil {
				log.Printf("gcp: failed to list objects: %v", err)
			}

			if err := bucketHandle.Object(objAttr.Name).Delete(ctx); err != nil {
				log.Printf("gcp: failed to delete object %s: %v", objAttr.Name, err)
			}
		}

		if err := bucketHandle.Delete(ctx); err != nil {
			log.Printf("gcp: failed to delete bucket %s: %v", bucketName, err)
		}
	}()

	var g errgroup.Group

	for _, arch := range u.Options.Architectures {
		g.Go(func() error {
			return u.uploadImage(ctx, arch, bucketName)
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("gcp: failed to upload images: %w", err)
	}

	return nil
}

func (u *GCPUploder) uploadImage(ctx context.Context, arch, bucketName string) error {
	objectPath := u.Options.GCPImage(arch)

	objectName := filepath.Base(objectPath)

	objectReader, err := os.Open(objectPath)
	if err != nil {
		return fmt.Errorf("gcp: failed to open object data file %s: %w", objectPath, err)
	}

	objectHandle := u.storageClient.Bucket(bucketName).Object(objectName)

	objectWriter := objectHandle.NewWriter(ctx)

	defer objectWriter.Close() //nolint:errcheck

	if _, err := io.Copy(objectWriter, objectReader); err != nil {
		return fmt.Errorf("gcp: failed to write object data: %w", err)
	}

	if err := objectWriter.Close(); err != nil {
		return fmt.Errorf("gcp: failed to close object writer: %w", err)
	}

	u.imagePath = fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, objectName)

	log.Println("gcp: uploaded image", u.imagePath)

	return u.registerImage(arch)
}

//nolint:gocyclo
func (u *GCPUploder) registerImage(arch string) error {
	imageName := fmt.Sprintf("talos-%s-%s", strings.ReplaceAll(u.Options.Tag, ".", "-"), arch)

	if u.Options.NamePrefix != "" {
		imageName = fmt.Sprintf("%s-talos-%s-%s", u.Options.NamePrefix, strings.ReplaceAll(u.Options.Tag, ".", "-"), arch)
	}

	exists, err := u.checkImageExists(imageName)
	if err != nil {
		return err
	}

	if exists {
		log.Printf("gcp: image %s already exists, deleting", imageName)

		if deleteErr := u.deleteImage(imageName); deleteErr != nil {
			return deleteErr
		}
	}

	operationID, link, err := u.insertImage(imageName, arch)
	if err != nil {
		return err
	}

	if err := retry.Constant(15*time.Minute, retry.WithUnits(30*time.Second)).Retry(func() error {
		op, err := u.computeService.GlobalOperations.Get(u.projectID, operationID).Do()
		if err != nil {
			return fmt.Errorf("gcp: failed to get operation: %w", err)
		}

		if op.HTTPStatusCode != http.StatusOK {
			return fmt.Errorf("gcp: operation failed with http error message: %s", op.HttpErrorMessage)
		}

		if op.Error != nil {
			return fmt.Errorf("gcp: operation faild with error message: %s", op.Error.Errors[0].Message)
		}

		if op.Status == "DONE" {
			return nil
		}

		log.Printf("gcp: image creation progress: %d", op.Progress)

		return retry.ExpectedError(fmt.Errorf("gcp: image status is %s", op.Status))
	}); err != nil {
		return fmt.Errorf("gcp: image creation is taking longer than expected: %w", err)
	}

	pushResult(CloudImage{
		Cloud:  "gcp",
		Tag:    u.Options.Tag,
		Region: "us",
		Arch:   arch,
		Type:   "compute#image",
		ID:     link,
	})

	return nil
}

func (u *GCPUploder) checkImageExists(imageName string) (bool, error) {
	_, err := u.computeService.Images.Get(u.projectID, imageName).Do()
	if err != nil {
		var googleErr *googleapi.Error
		if errors.As(err, &googleErr) {
			if googleErr.Code == http.StatusNotFound {
				return false, nil
			}
		}

		return false, fmt.Errorf("gcp: failed to get image %s: %w", imageName, err)
	}

	return true, nil
}

func (u *GCPUploder) insertImage(imageName, arch string) (operationID, imageLink string, err error) {
	var archImage string

	switch arch {
	case "amd64":
		archImage = "x86_64"
	case "arm64":
		archImage = "ARM64"
	default:
		return "", "", fmt.Errorf("gcp: unknown architecture %s", arch)
	}

	op, err := u.computeService.Images.Insert(u.projectID, &compute.Image{
		Architecture: archImage,
		Description:  fmt.Sprintf("Talos %s %s", u.Options.Tag, arch),
		GuestOsFeatures: []*compute.GuestOsFeature{
			{
				Type: "VIRTIO_SCSI_MULTIQUEUE",
			},
			{
				Type: "UEFI_COMPATIBLE",
			},
		},
		Name: imageName,
		RawDisk: &compute.ImageRawDisk{
			Source: u.imagePath,
		},
		ShieldedInstanceInitialState: &compute.InitialStateConfig{},
	}).Do()
	if err != nil {
		return "", "", fmt.Errorf("gcp: failed to insert image: %w", err)
	}

	if op.HTTPStatusCode != http.StatusOK {
		return "", "", fmt.Errorf("gcp: insert image failed with http error message: %s", op.HttpErrorMessage)
	}

	if op.Error != nil {
		return "", "", fmt.Errorf("gcp: insert image failed with error message: %s", op.Error.Errors[0].Message)
	}

	log.Printf("gcp: image %s is being created with operation %s", imageName, op.Name)

	return op.Name, op.TargetLink, nil
}

func (u *GCPUploder) deleteImage(imageName string) error {
	if _, err := u.computeService.Images.Delete(u.projectID, imageName).Do(); err != nil {
		return fmt.Errorf("gcp: failed to delete image %s: %w", imageName, err)
	}

	if err := retry.Constant(5*time.Minute, retry.WithUnits(30*time.Second)).Retry(func() error {
		_, err := u.computeService.Images.Get(u.projectID, imageName).Do()
		if err != nil {
			var googleErr *googleapi.Error

			if errors.As(err, &googleErr) {
				if googleErr.Code == http.StatusNotFound {
					return nil
				}
			}

			return err
		}

		return retry.ExpectedError(fmt.Errorf("gcp: image %s still exists", imageName))
	}); err != nil {
		return fmt.Errorf("gcp: failed to delete image %s: %w", imageName, err)
	}

	return nil
}
