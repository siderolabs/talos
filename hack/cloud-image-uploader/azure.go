// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/pageblob"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/hashicorp/go-version"
	"github.com/siderolabs/gen/channel"
	"github.com/ulikunitz/xz"
	"golang.org/x/sync/errgroup"
)

const (
	resourceGroupName = "SideroGallery"
	defaultRegion     = "eastus"
	storageAccount    = "siderogallery"
)

//go:embed azure-disk-template.json
var azureDiskTemplate []byte

//go:embed azure-image-version-template.json
var azureImageVersionTemplate []byte

// TargetRegion describes the region to upload to.
type TargetRegion struct {
	Name                 string `json:"name"`
	RegionalReplicaCount int    `json:"regionalReplicaCount"`
	StorageAccountType   string `json:"storageAccountType"`
}

// Mapping CPU architectures to Azure architectures.
var azureArchitectures = map[string]string{
	"amd64": "x64",
	"arm64": "arm64",
}

// AzureUploader represents an object that has the capability to upload to Azure.
type AzureUploader struct {
	Options Options

	helper azureHelper
}

// extractVersion extracts the version number in the format of int.int.int for Azure and assigns to the Options.AzureTag value.
func (azu *AzureUploader) setVersion() error {
	v, err := version.NewVersion(azu.Options.AzureAbbrevTag)
	if err != nil {
		return err
	}

	versionCore := v.Core().String()

	if fmt.Sprintf("v%s", versionCore) != azu.Options.AzureAbbrevTag {
		azu.Options.AzureGalleryName = "SideroGalleryTest"
		azu.Options.AzureCoreTag = versionCore
		azu.Options.AzurePreRelease = "-prerelease"
	} else {
		azu.Options.AzureGalleryName = "SideroGallery"
		azu.Options.AzureCoreTag = versionCore
		azu.Options.AzurePreRelease = ""
	}

	return err
}

// AzureGalleryUpload uploads the image to Azure.
func (azu *AzureUploader) AzureGalleryUpload(ctx context.Context) error {
	var err error

	var g *errgroup.Group
	g, ctx = errgroup.WithContext(ctx)

	err = azu.setVersion()
	if err != nil {
		log.Printf("azure: error setting version: %v\n", err)
	}

	log.Printf("azure: setting default creds")

	err = azu.helper.setDefaultAzureCreds()
	if err != nil {
		return fmt.Errorf("error setting default Azure credentials: %w", err)
	}

	log.Printf("azure: getting locations")

	err = azu.helper.getAzureLocations(ctx)
	if err != nil {
		return fmt.Errorf("error setting default Azure credentials: %w", err)
	}

	// Upload blob
	log.Printf("azure: creating disks for architectures: %+v\n", azu.Options.Architectures)

	for _, arch := range azu.Options.Architectures {
		arch := arch

		g.Go(func() error {
			log.Printf("azure: starting upload blob for %s\n", arch)
			err = azu.uploadAzureBlob(ctx, arch)
			if err != nil {
				return fmt.Errorf("azure: error uploading page blob for %s: %w", arch, err)
			}

			log.Printf("azure: starting disk creation for %s\n", arch)
			err = azu.createAzureDisk(ctx, azureDiskTemplate, arch)
			if err != nil {
				log.Printf("azure: error creating disk: %v\n", err)
			}

			log.Printf("azure: starting image version creation for %s\n", arch)
			err = azu.createAzureImageVersion(ctx, azureImageVersionTemplate, arch)
			if err != nil {
				log.Printf("azure: error creating image version: %v\n", err)
			}

			return err
		})
	}

	return g.Wait()
}

//nolint:gocyclo
func (azu *AzureUploader) uploadAzureBlob(ctx context.Context, arch string) error {
	blobURL := fmt.Sprintf("https://siderogallery.blob.core.windows.net/images/talos/talos-%s-%s.vhd", arch, azu.Options.Tag)

	pageBlobClient, err := pageblob.NewClient(blobURL, azu.helper.cred, nil)
	if err != nil {
		log.Printf("azure: error creating pageblob client: %v\n", err)
	}

	source, err := os.Open(azu.Options.AzureImage(arch))
	if err != nil {
		return err
	}

	defer source.Close() //nolint:errcheck

	// calculate totalSize
	file, err := xz.NewReader(source)
	if err != nil {
		return fmt.Errorf("azure: error extracting file from xz: %w", err)
	}

	totalSize, err := io.Copy(io.Discard, file)
	if err != nil {
		return fmt.Errorf("azure: error calculating totalSize: %w", err)
	}

	// second pass: read chunks and upload
	// seek back to the beginning of the source file
	_, err = source.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("azure: error seeking back: %w", err)
	}

	file, err = xz.NewReader(source)
	if err != nil {
		return fmt.Errorf("azure: error extracting file from xz: %w", err)
	}

	// Check if the file size is a multiple of 512 bytes
	if totalSize%pageblob.PageBytes != 0 {
		panic("azure: error: the file size must be a multiple of 512 bytes")
	}

	_, err = pageBlobClient.Create(ctx, totalSize, nil)
	if err != nil {
		log.Printf("azure: error creating vhd: %v\n", err)
	}

	type work struct {
		chunk  []byte
		offset int64
	}

	const (
		concurrency = 8
		chunkSize   = 4 * 1024 * 1024
	)

	workCh := make(chan work)

	var g *errgroup.Group
	g, ctx = errgroup.WithContext(ctx)

	for i := 0; i < concurrency; i++ {
		g.Go(func() error {
			for w := range workCh {
				_, err = pageBlobClient.UploadPages(
					ctx,
					streaming.NopCloser(bytes.NewReader(w.chunk)),
					blob.HTTPRange{Offset: w.offset, Count: int64(len(w.chunk))},
					nil)
				if err != nil {
					return fmt.Errorf("azure: error uploading chunk at offset %d: %w", w.offset, err)
				}
			}

			return nil
		})
	}

	var offset int64

uploadLoop:
	for {
		buf := make([]byte, chunkSize)

		var n int

		n, err = io.ReadFull(file, buf)
		switch {
		case err == io.ErrUnexpectedEOF:
			// this is the last (incomplete) chunk
		case err == io.EOF:
			// end of file, stop
			break uploadLoop
		case err != nil:
			return fmt.Errorf("azure: error reading chunk: %w", err)
		}

		if !channel.SendWithContext(ctx, workCh, work{chunk: buf[:n], offset: offset}) {
			break uploadLoop
		}

		offset += int64(n)

		if offset%(chunkSize*10) == 0 {
			log.Printf("azure: uploaded %d bytes\n", offset)
		}
	}

	close(workCh)

	if err = g.Wait(); err != nil {
		return fmt.Errorf("azure: error uploading chunks: %w", err)
	}

	return nil
}

func (azu *AzureUploader) createAzureDisk(ctx context.Context, armTemplate []byte, arch string) error {
	diskParameters := map[string]interface{}{
		"disk_name": map[string]string{
			"value": fmt.Sprintf("talos-%s-%s%s", arch, azu.Options.AzureCoreTag, azu.Options.AzurePreRelease),
		},
		"storage_account": map[string]string{
			"value": storageAccount,
		},
		"vhd_name": map[string]string{
			"value": fmt.Sprintf("talos-%s-%s.vhd", arch, azu.Options.Tag),
		},
		"region": map[string]string{
			"value": defaultRegion,
		},
		"architecture": map[string]string{
			"value": azureArchitectures[arch],
		},
	}

	deploymentName := fmt.Sprintf("disk-talos-%s-%s", arch, azu.Options.Tag)

	if err := azu.helper.deployResourceFromTemplate(ctx, armTemplate, diskParameters, deploymentName); err != nil {
		return fmt.Errorf("azure: error applying Azure disk template: %w", err)
	}

	return nil
}

func (azu *AzureUploader) createAzureImageVersion(ctx context.Context, armTemplate []byte, arch string) error {
	targetRegions := make([]TargetRegion, 0, len(azu.helper.locations))

	for _, region := range azu.helper.locations {
		targetRegions = append(targetRegions, TargetRegion{
			Name:                 region.Name,
			RegionalReplicaCount: 1,
			StorageAccountType:   "Standard_LRS",
		})
	}

	versionParameters := map[string]interface{}{
		"disk_name": map[string]string{
			"value": fmt.Sprintf("talos-%s-%s%s", arch, azu.Options.AzureCoreTag, azu.Options.AzurePreRelease),
		},
		"image_version": map[string]string{
			"value": azu.Options.AzureCoreTag,
		},
		"gallery_name": map[string]string{
			"value": azu.Options.AzureGalleryName,
		},
		"definition_name": map[string]string{
			"value": fmt.Sprintf("talos-%s", azureArchitectures[arch]),
		},
		"region": map[string]string{
			"value": defaultRegion,
		},
		"resourceGroupName": map[string]string{
			"value": resourceGroupName,
		},
		"targetRegions": map[string]interface{}{
			"value": targetRegions,
		},
	}

	deploymentName := fmt.Sprintf("img-version-talos-%s-%s", arch, azu.Options.Tag)

	if err := azu.helper.deployResourceFromTemplate(ctx, armTemplate, versionParameters, deploymentName); err != nil {
		return fmt.Errorf("azure: error applying Azure image version template: %w", err)
	}

	return nil
}

type azureHelper struct {
	subscriptionID  string
	cred            *azidentity.DefaultAzureCredential
	authorizer      autorest.Authorizer
	providersClient resources.ProvidersClient
	locations       map[string]Location
}

func (helper *azureHelper) setDefaultAzureCreds() error {
	helper.subscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")
	if len(helper.subscriptionID) == 0 {
		log.Fatalln("AZURE_SUBSCRIPTION_ID is not set.")
	}

	authFromEnvironment, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return err
	}

	helper.authorizer = authFromEnvironment

	// Create a new instance of the DefaultAzureCredential
	helper.cred, err = azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return err
	}

	// Initialize the Storage Accounts Client
	var storageClientFactory *armstorage.ClientFactory

	storageClientFactory, err = armstorage.NewClientFactory(helper.subscriptionID, helper.cred, nil)
	if err != nil {
		return err
	}

	_ = storageClientFactory.NewAccountsClient()
	helper.providersClient = resources.NewProvidersClient(helper.subscriptionID)
	helper.providersClient.Authorizer = helper.authorizer

	return nil
}

//nolint:gocyclo
func (helper *azureHelper) getAzureLocations(ctx context.Context) error {
	providers, err := helper.listProviders(ctx)
	if err != nil {
		return err
	}

	var computeProvider resources.Provider

	for _, provider := range providers {
		if provider.Namespace != nil && *provider.Namespace == "Microsoft.Compute" {
			computeProvider = provider

			break
		}
	}

	helper.locations = make(map[string]Location)

	if computeProvider.ResourceTypes != nil {
		for _, rt := range *computeProvider.ResourceTypes {
			if rt.ResourceType != nil && *rt.ResourceType == "virtualMachines" {
				if rt.Locations != nil {
					for _, region := range *rt.Locations {
						abbr := strings.ReplaceAll(region, " ", "")
						abbr = strings.ToLower(abbr)
						helper.locations[abbr] = Location{Abbreviation: abbr, Name: region}
					}
				}

				break
			}
		}
	}

	return nil
}

func (helper *azureHelper) listProviders(ctx context.Context) (result []resources.Provider, err error) {
	for list, err := helper.providersClient.List(ctx, ""); list.NotDone(); err = list.NextWithContext(ctx) {
		if err != nil {
			return nil, fmt.Errorf("azure: error getting providers list: %v", err)
		}

		result = append(result, list.Values()...)
	}

	return
}

func (helper *azureHelper) deployResourceFromTemplate(ctx context.Context, templateBytes []byte, parameters map[string]interface{}, deploymentName string) error {
	// Create a new instance of the DeploymentsClient
	deploymentsClient, err := armresources.NewDeploymentsClient(helper.subscriptionID, helper.cred, nil)
	if err != nil {
		return err
	}

	// Replace these variables with your own values
	resourceGroupName := "SideroGallery"

	// Parse the template JSON
	var template map[string]interface{}

	if err = json.Unmarshal(templateBytes, &template); err != nil {
		return fmt.Errorf("azure: error parsing template JSON: %w", err)
	}

	deployment := armresources.Deployment{
		Properties: &armresources.DeploymentProperties{
			Template:   template,
			Parameters: parameters,
			Mode:       to.Ptr(armresources.DeploymentModeIncremental),
		},
	}

	poller, err := deploymentsClient.BeginCreateOrUpdate(ctx, resourceGroupName, deploymentName, deployment, nil)
	if err != nil {
		return fmt.Errorf("azure: failed to create deployment: %w", err)
	}

	// PollUntilDone requires a context and a poll interval
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("azure: failed to poll deployment status: %w", err)
	}

	log.Printf("azure: deployment operation for %s: %+v\n", *result.Name, *result.Properties.ProvisioningState)

	return nil
}
