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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/google/uuid"
	"github.com/talos-systems/go-retry/retry"
	"golang.org/x/sync/errgroup"
)

// GetAWSDefaultRegions returns a list of regions which are enabled for this account.
func GetAWSDefaultRegions() ([]string, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed creating AWS session: %w", err)
	}

	result, err := ec2.New(sess).DescribeRegions(&ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed getting list of regions: %w", err)
	}

	regions := []string{}

	for _, r := range result.Regions {
		if r.OptInStatus != nil {
			if *r.OptInStatus == "opt-in-not-required" || *r.OptInStatus == "opted-in" {
				regions = append(regions, *r.RegionName)
			}
		}
	}

	return regions, nil
}

// AWSUploader registers AMI in the AWS.
type AWSUploader struct {
	Options Options

	sess    *session.Session
	ec2svcs map[string]*ec2.EC2
}

var awsArchitectures = map[string]string{
	"amd64": "x86_64",
	"arm64": "arm64",
}

// Upload image and register with AWS.
func (au *AWSUploader) Upload(ctx context.Context) error {
	var err error

	au.sess, err = session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"), // gets overridden in each uploader with specific region
	})
	if err != nil {
		return fmt.Errorf("failed creating AWS session: %w", err)
	}

	au.ec2svcs = make(map[string]*ec2.EC2)

	for _, region := range au.Options.AWSRegions {
		au.ec2svcs[region] = ec2.New(au.sess, aws.NewConfig().WithRegion(region))
	}

	return au.RegisterAMIs(ctx)
}

// RegisterAMIs in every region.
func (au *AWSUploader) RegisterAMIs(ctx context.Context) error {
	var g *errgroup.Group

	g, ctx = errgroup.WithContext(ctx)

	for region, svc := range au.ec2svcs {
		region := region
		svc := svc

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

func (au *AWSUploader) registerAMI(ctx context.Context, region string, svc *ec2.EC2) error {
	s3Svc := s3.New(au.sess, aws.NewConfig().WithRegion(region))
	bucketName := fmt.Sprintf("talos-image-upload-%s", uuid.New())

	_, err := s3Svc.CreateBucketWithContext(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed creating S3 bucket: %w", err)
	}

	err = s3Svc.WaitUntilBucketExistsWithContext(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed creating S3 bucket: %w", err)
	}

	log.Printf("aws: created bucket %q for %s", bucketName, region)

	defer func() {
		iter := s3manager.NewDeleteListIterator(s3Svc, &s3.ListObjectsInput{
			Bucket: aws.String(bucketName),
		})

		if err = s3manager.NewBatchDeleteWithClient(s3Svc).Delete(aws.BackgroundContext(), iter); err != nil {
			log.Printf("Unable to delete objects from bucket %q, %v", bucketName, err)
		}

		_, err = s3Svc.DeleteBucket(&s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			log.Printf("failed deleting bucket: %s", err)
		}
	}()

	uploader := s3manager.NewUploaderWithClient(s3Svc)

	var g errgroup.Group

	for _, arch := range au.Options.Architectures {
		arch := arch

		g.Go(func() error {
			err = au.registerAMIArch(ctx, region, svc, arch, bucketName, uploader)
			if err != nil {
				return fmt.Errorf("error registering AMI for %s: %w", arch, err)
			}

			return nil
		})
	}

	return g.Wait()
}

func (au *AWSUploader) registerAMIArch(ctx context.Context, region string, svc *ec2.EC2, arch, bucketName string, uploader *s3manager.Uploader) error {
	err := retry.Constant(5*time.Minute, retry.WithUnits(time.Second)).Retry(func() error {
		source, err := os.Open(au.Options.AWSImage(arch))
		if err != nil {
			return err
		}

		defer source.Close() //nolint:errcheck

		image, err := ExtractFileFromTarGz("disk.raw", source)
		if err != nil {
			return err
		}

		_, err = uploader.UploadWithContext(ctx, &s3manager.UploadInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(fmt.Sprintf("disk-%s.raw", arch)),
			Body:   image,
		})

		return retry.ExpectedError(err)
	})
	if err != nil {
		return err
	}

	log.Printf("aws: import into %s/%s, image uploaded to S3", region, arch)

	resp, err := svc.ImportSnapshotWithContext(ctx, &ec2.ImportSnapshotInput{
		Description: aws.String(fmt.Sprintf("Talos Image %s %s %s", au.Options.Tag, arch, region)),
		DiskContainer: &ec2.SnapshotDiskContainer{
			Format: aws.String("raw"),
			UserBucket: &ec2.UserBucket{
				S3Bucket: aws.String(bucketName),
				S3Key:    aws.String(fmt.Sprintf("disk-%s.raw", arch)),
			},
		},
	})
	if err != nil {
		return err
	}

	taskID := *resp.ImportTaskId

	var snapshotID string

	log.Printf("aws: import into %s/%s, task ID %q", region, arch, taskID)

	progress := "0"

	err = retry.Constant(30*time.Minute, retry.WithUnits(30*time.Second)).Retry(func() error {
		var status *ec2.DescribeImportSnapshotTasksOutput

		status, err = svc.DescribeImportSnapshotTasksWithContext(ctx, &ec2.DescribeImportSnapshotTasksInput{
			ImportTaskIds: aws.StringSlice([]string{taskID}),
		})
		if err != nil {
			return retry.UnexpectedError(err)
		}

		for _, task := range status.ImportSnapshotTasks {
			if *task.ImportTaskId == taskID {
				if *task.SnapshotTaskDetail.Status == "completed" {
					snapshotID = *task.SnapshotTaskDetail.SnapshotId

					return nil
				}

				if *task.SnapshotTaskDetail.Progress != progress {
					progress = *task.SnapshotTaskDetail.Progress

					log.Printf("aws: import into %s/%s, import snapshot %s%%", region, arch, progress)
				}

				return retry.ExpectedError(fmt.Errorf("task status is %s", *task.SnapshotTaskDetail.Status))
			}
		}

		return retry.ExpectedError(fmt.Errorf("task status not found"))
	})
	if err != nil {
		return err
	}

	log.Printf("aws: import into %s/%s, snapshot ID %q", region, arch, snapshotID)

	imageName := fmt.Sprintf("talos-%s-%s-%s", au.Options.Tag, region, arch)

	imageResp, err := svc.DescribeImagesWithContext(ctx, &ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("name"),
				Values: aws.StringSlice([]string{imageName}),
			},
		},
	})
	if err != nil {
		return err
	}

	for _, image := range imageResp.Images {
		_, err = svc.DeregisterImageWithContext(ctx, &ec2.DeregisterImageInput{
			ImageId: image.ImageId,
		})
		if err != nil {
			return err
		}

		log.Printf("aws: import into %s/%s, deregistered image ID %q", region, arch, *image.ImageId)
	}

	registerResp, err := svc.RegisterImageWithContext(ctx, &ec2.RegisterImageInput{
		Name: aws.String(imageName),
		BlockDeviceMappings: []*ec2.BlockDeviceMapping{
			{
				DeviceName:  aws.String("/dev/xvda"),
				VirtualName: aws.String("talos"),
				Ebs: &ec2.EbsBlockDevice{
					DeleteOnTermination: aws.Bool(true),
					SnapshotId:          aws.String(snapshotID),
					VolumeSize:          aws.Int64(20),
					VolumeType:          aws.String("gp2"),
				},
			},
		},
		RootDeviceName:     aws.String("/dev/xvda"),
		VirtualizationType: aws.String("hvm"),
		EnaSupport:         aws.Bool(true),
		Description:        aws.String(fmt.Sprintf("Talos AMI %s %s %s", au.Options.Tag, arch, region)),
		Architecture:       aws.String(awsArchitectures[arch]),
	})
	if err != nil {
		return err
	}

	imageID := *registerResp.ImageId

	log.Printf("aws: import into %s/%s, registered image ID %q", region, arch, imageID)

	_, err = svc.ModifyImageAttributeWithContext(ctx, &ec2.ModifyImageAttributeInput{
		ImageId: aws.String(imageID),
		LaunchPermission: &ec2.LaunchPermissionModifications{
			Add: []*ec2.LaunchPermission{
				{
					Group: aws.String("all"),
				},
			},
		},
		Attribute: aws.String("launchPermission"),
	})

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
