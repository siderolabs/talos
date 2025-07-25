// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	"golang.org/x/sync/errgroup"
)

var denyInsecurePolicyTemplate = `{
  "Id": "ExamplePolicy",
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowSSLRequestsOnly",
      "Action": "s3:*",
      "Effect": "Deny",
      "Resource": [
        "arn:aws:s3:::%s",
        "arn:aws:s3:::%s/*"
      ],
      "Condition": {
        "Bool": {
          "aws:SecureTransport": "false"
        }
      },
      "Principal": "*"
    }
  ]
}`

// GetAWSDefaultRegions returns a list of regions which are enabled for this account.
func GetAWSDefaultRegions(ctx context.Context) ([]string, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return nil, fmt.Errorf("failed loading AWS config: %w", err)
	}

	svc := ec2.NewFromConfig(cfg)

	resp, err := svc.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		Filters: []types.Filter{
			{
				Name:   pointer.To("opt-in-status"),
				Values: []string{"opt-in-not-required", "opted-in"},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed describing regions: %w", err)
	}

	return xslices.Map(resp.Regions, func(r types.Region) string {
		return pointer.SafeDeref(r.RegionName)
	}), nil
}

// AWSUploader registers AMI in the AWS.
type AWSUploader struct {
	Options Options

	cfg     aws.Config
	ec2svcs map[string]*ec2.Client
}

var awsArchitectures = map[string]types.ArchitectureValues{
	"amd64": types.ArchitectureValuesX8664,
	"arm64": types.ArchitectureValuesArm64,
}

// Upload image and register with AWS.
func (au *AWSUploader) Upload(ctx context.Context) error {
	var err error

	au.cfg, err = config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed loading AWS config: %w", err)
	}

	au.ec2svcs = make(map[string]*ec2.Client)

	for _, region := range au.Options.AWSRegions {
		au.ec2svcs[region] = ec2.NewFromConfig(au.cfg, func(o *ec2.Options) {
			o.Region = region
		})
	}

	return au.RegisterAMIs(ctx)
}

// RegisterAMIs in every region.
func (au *AWSUploader) RegisterAMIs(ctx context.Context) error {
	var g *errgroup.Group

	g, ctx = errgroup.WithContext(ctx)

	for region, svc := range au.ec2svcs {
		g.Go(func() error {
			err := au.registerAMI(ctx, region, svc)
			if err != nil {
				return fmt.Errorf("error registering AMI in %s: %w", region, err)
			}

			return nil
		})
	}

	return g.Wait()
}

func cleanupBucket(s3Svc *s3.Client, bucketName string) {
	// create a detached context, as context might be canceled
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	mpuPaginator := s3.NewListMultipartUploadsPaginator(s3Svc, &s3.ListMultipartUploadsInput{
		Bucket: pointer.To(bucketName),
	})

	for mpuPaginator.HasMorePages() {
		page, err := mpuPaginator.NextPage(ctx)
		if err != nil {
			log.Printf("failed listing multipart uploads: %s", err)

			break
		}

		for _, upload := range page.Uploads {
			_, err = s3Svc.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   pointer.To(bucketName),
				Key:      upload.Key,
				UploadId: upload.UploadId,
			})
			if err != nil {
				log.Printf("failed aborting multipart upload: %s", err)
			}
		}
	}

	objectsPaginator := s3.NewListObjectsV2Paginator(s3Svc, &s3.ListObjectsV2Input{
		Bucket: pointer.To(bucketName),
	})

	for objectsPaginator.HasMorePages() {
		page, err := objectsPaginator.NextPage(ctx)
		if err != nil {
			log.Printf("failed listing objects: %s", err)

			break
		}

		if len(page.Contents) == 0 {
			break
		}

		_, err = s3Svc.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: pointer.To(bucketName),
			Delete: &s3types.Delete{
				Objects: xslices.Map(page.Contents, func(obj s3types.Object) s3types.ObjectIdentifier {
					return s3types.ObjectIdentifier{
						Key: obj.Key,
					}
				}),
			},
		})
		if err != nil {
			log.Printf("failed deleting objects: %s", err)
		}
	}

	_, err := s3Svc.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.Printf("failed deleting bucket: %s", err)
	}

	log.Printf("aws: deleted bucket %q", bucketName)
}

func (au *AWSUploader) registerAMI(ctx context.Context, region string, svc *ec2.Client) error {
	s3Svc := s3.NewFromConfig(au.cfg, func(o *s3.Options) {
		o.Region = region
	})
	bucketName := fmt.Sprintf("talos-image-upload-%s", uuid.New())

	var createBucketConfiguration *s3types.CreateBucketConfiguration

	if region != "us-east-1" {
		createBucketConfiguration = &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(region),
		}
	}

	_, err := s3Svc.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket:                    pointer.To(bucketName),
		CreateBucketConfiguration: createBucketConfiguration,
	})
	if err != nil {
		return fmt.Errorf("failed creating S3 bucket: %w", err)
	}

	if err = s3.NewBucketExistsWaiter(s3Svc).Wait(ctx, &s3.HeadBucketInput{
		Bucket: pointer.To(bucketName),
	}, time.Minute); err != nil {
		return fmt.Errorf("failed waiting for S3 bucket: %w", err)
	}

	log.Printf("aws: created bucket %q for %s", bucketName, region)

	defer func() {
		cleanupBucket(s3Svc, bucketName)
	}()

	_, err = s3Svc.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: pointer.To(bucketName),
		Policy: pointer.To(fmt.Sprintf(denyInsecurePolicyTemplate, bucketName, bucketName)),
	})
	if err != nil {
		return fmt.Errorf("failed applying S3 bucket policy: %w", err)
	}

	log.Printf("aws: applied policy to bucket %q", bucketName)

	uploader := manager.NewUploader(s3Svc)

	var g errgroup.Group

	for _, arch := range au.Options.Architectures {
		g.Go(func() error {
			err = au.registerAMIArch(ctx, region, svc, arch, bucketName, uploader)
			if err != nil {
				log.Printf("WARNING: aws: ignoring failure to upload AMI into %s/%s: %s", region, arch, err)
			}

			return nil
		})
	}

	return g.Wait()
}

//nolint:gocyclo
func (au *AWSUploader) tagSnapshot(ctx context.Context, svc *ec2.Client, snapshotID, imageName string) {
	if snapshotID == "" {
		return
	}

	_, tagErr := svc.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{snapshotID},
		Tags: []types.Tag{{
			Key:   pointer.To("Name"),
			Value: pointer.To(imageName),
		}},
	})
	if tagErr != nil {
		log.Printf("WARNING: failed to tag snapshot %s: %v", snapshotID, tagErr)
	}
}

//nolint:gocyclo
func (au *AWSUploader) registerAMIArch(ctx context.Context, region string, svc *ec2.Client, arch, bucketName string, uploader *manager.Uploader) error {
	err := retry.Constant(30*time.Minute, retry.WithUnits(time.Second), retry.WithErrorLogging(true)).RetryWithContext(ctx, func(ctx context.Context) error {
		source, err := os.Open(au.Options.AWSImage(arch))
		if err != nil {
			return err
		}

		defer source.Close() //nolint:errcheck

		image, err := zstd.NewReader(source)
		if err != nil {
			return err
		}

		defer image.Close()

		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket: pointer.To(bucketName),
			Key:    pointer.To(fmt.Sprintf("disk-%s.raw", arch)),
			Body:   image,
		})

		return retry.ExpectedError(err)
	})
	if err != nil {
		return fmt.Errorf("failed to upload image to the bucket: %w", err)
	}

	log.Printf("aws: import into %s/%s, image uploaded to S3", region, arch)

	resp, err := svc.ImportSnapshot(ctx, &ec2.ImportSnapshotInput{
		Description: pointer.To(fmt.Sprintf("Talos Image %s %s %s", au.Options.Tag, arch, region)),
		DiskContainer: &types.SnapshotDiskContainer{
			Description: pointer.To(fmt.Sprintf("Talos Image %s %s %s", au.Options.Tag, arch, region)),
			Format:      pointer.To("raw"),
			UserBucket: &types.UserBucket{
				S3Bucket: pointer.To(bucketName),
				S3Key:    pointer.To(fmt.Sprintf("disk-%s.raw", arch)),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to import snapshot: %w", err)
	}

	taskID := *resp.ImportTaskId

	var snapshotID string

	log.Printf("aws: import into %s/%s, task ID %q", region, arch, taskID)

	progress := "0"

	err = retry.Constant(30*time.Minute, retry.WithUnits(30*time.Second)).Retry(func() error {
		var status *ec2.DescribeImportSnapshotTasksOutput

		status, err = svc.DescribeImportSnapshotTasks(ctx, &ec2.DescribeImportSnapshotTasksInput{
			ImportTaskIds: []string{taskID},
		})
		if err != nil {
			return err
		}

		for _, task := range status.ImportSnapshotTasks {
			if pointer.SafeDeref(task.ImportTaskId) == taskID {
				if task.SnapshotTaskDetail == nil {
					continue
				}

				if pointer.SafeDeref(task.SnapshotTaskDetail.Status) == "completed" {
					snapshotID = pointer.SafeDeref(task.SnapshotTaskDetail.SnapshotId)

					return nil
				}

				if pointer.SafeDeref(task.SnapshotTaskDetail.Progress) != progress {
					progress = pointer.SafeDeref(task.SnapshotTaskDetail.Progress)

					log.Printf("aws: import into %s/%s, import snapshot %s%%", region, arch, progress)
				}

				return retry.ExpectedErrorf("task status is %s", *task.SnapshotTaskDetail.Status)
			}
		}

		return retry.ExpectedErrorf("task status not found")
	})
	if err != nil {
		return fmt.Errorf("failed to wait for import task: %w", err)
	}

	log.Printf("aws: import into %s/%s, snapshot ID %q", region, arch, snapshotID)

	imageName := fmt.Sprintf("talos-%s-%s-%s", au.Options.Tag, region, arch)

	if au.Options.NamePrefix != "" {
		imageName = fmt.Sprintf("%s-%s-%s-%s", au.Options.NamePrefix, au.Options.Tag, region, arch)
	}

	au.tagSnapshot(ctx, svc, snapshotID, imageName)

	imageResp, err := svc.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{imageName},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to describe images: %w", err)
	}

	for _, image := range imageResp.Images {
		_, err = svc.DeregisterImage(ctx, &ec2.DeregisterImageInput{
			ImageId: image.ImageId,
		})
		if err != nil {
			return fmt.Errorf("failed to deregister image: %w", err)
		}

		log.Printf("aws: import into %s/%s, deregistered image ID %q", region, arch, *image.ImageId)
	}

	registerReq := &ec2.RegisterImageInput{
		Name: aws.String(imageName),
		BlockDeviceMappings: []types.BlockDeviceMapping{
			{
				DeviceName:  pointer.To("/dev/xvda"),
				VirtualName: pointer.To("talos"),
				Ebs: &types.EbsBlockDevice{
					DeleteOnTermination: pointer.To(true),
					SnapshotId:          pointer.To(snapshotID),
					VolumeSize:          pointer.To[int32](20),
					VolumeType:          types.VolumeTypeGp2,
				},
			},
		},
		RootDeviceName:     pointer.To("/dev/xvda"),
		VirtualizationType: pointer.To("hvm"),
		EnaSupport:         pointer.To(true),
		Description:        pointer.To(fmt.Sprintf("Talos AMI %s %s %s", au.Options.Tag, arch, region)),
		Architecture:       awsArchitectures[arch],
		ImdsSupport:        types.ImdsSupportValuesV20,
	}

	if !au.Options.AWSForceBIOS {
		registerReq.BootMode = types.BootModeValuesUefiPreferred
	}

	registerResp, err := svc.RegisterImage(ctx, registerReq)
	if err != nil {
		return fmt.Errorf("failed to register image: %w", err)
	}

	imageID := *registerResp.ImageId

	log.Printf("aws: import into %s/%s, registered image ID %q", region, arch, imageID)

	_, err = svc.ModifyImageAttribute(ctx, &ec2.ModifyImageAttributeInput{
		ImageId: aws.String(imageID),
		LaunchPermission: &types.LaunchPermissionModifications{
			Add: []types.LaunchPermission{
				{
					Group: types.PermissionGroupAll,
				},
			},
		},
		Attribute: aws.String("launchPermission"),
	})
	if err != nil {
		return fmt.Errorf("failed to modify image attribute: %w", err)
	}

	pushResult(CloudImage{
		Cloud:  "aws",
		Tag:    au.Options.Tag,
		Region: region,
		Arch:   arch,
		Type:   "hvm",
		ID:     imageID,
	})

	return nil
}
