# cloud-image-uploader

## vmimport role

Role should be pre-created before running this command.

    aws iam create-role --role-name vmimport --assume-role-policy-document file://trust-policy.json
    aws iam put-role-policy --role-name vmimport --policy-name vmimport --policy-document file://role-policy.json

## Azure Pre-requisites

### Configuring the Portal

Community Gallery (preview) information can be found [here](https://learn.microsoft.com/en-us/azure/virtual-machines/share-gallery-community?tabs=cli).

- Create **Resource Group**: `SideroGallery`
  - [Azure Documentation](https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal)
- Create **Storage Account**: `siderogallery`
  - [Azure Documentation](https://learn.microsoft.com/en-us/azure/storage/common/storage-account-create?tabs=azure-portal)
- Create storage **Container**: `images`
  - [Azure Documentation](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction)
- Create **Azure Compute Gallery**: `SideroLabs`
  - [Azure Documentation](https://learn.microsoft.com/en-us/azure/virtual-machines/azure-compute-gallery)
  - Search for **Azure Compute Gallery** in the portal search bar.
  - Select **Create**.
  - Fill in the required information.
    - In the **Sharing** Tab select **RBAC + share to public community gallery (PREVIEW)**
    - Select **Review + create**
- Create Compute Gallery **Image Definition**: `talos-arm64`, `talos-x64
  - [Azure Documentation](https://learn.microsoft.com/en-us/azure/virtual-machines/azure-compute-gallery)
  - Select the `SideroLabs` Compute Gallery.
  - Select the notification at the top of the page to share the gallery.
  - Select **New Image Definition**
    - Create an Image definition for each architecture type:
      - This is where V2 must be selected for the VM generation in order for an arm64 image version to be created in the definition.
        - **Publisher**: `siderolabs`
        - **Offer**: `talos`
        - SKU: must be unique
        - Do not create an image version yet.

### App Registration

The App Registration is what we will use to authenticate to Azure for uploading blobs and creating resources.

[Azure Documentation](https://learn.microsoft.com/en-us/azure/active-directory/develop/quickstart-register-app)

#### Create an App Registration

- Search for and Select **Azure Active Directory**.
- Select **App registrations**, then select **New registration**.
- Name the application, for example "example-app".
- Select a supported account type, which determines who can use the application.
- Under **Redirect URI**, select **Web** for the type of application you want to create, enter the URI where the access token is sent to.
- Select **Register**.

#### Environment Variables

Get the following values for azure-go-sdk

- **Subscription ID**
  -Login into your Azure account
  - Select Subscriptions in the left sidebar
  - Select whichever subscription is needed
  - Click on Overview
  - Copy the Subscription ID
- **Client ID**
- **Client Secret**
- **Tenant ID**

These are stored as Drone secrets as:

- azure_subscription_id
- azure_client_id
- azure_client_secret
- azure_tenant_id

#### Add permissions for App Registration

The App registration only needs permissions to the Compute Gallery and the Storage Account.

- Compute Gallery:

  - Select the `SideroLabs` Compute Gallery
  - Select Access control (IAM)
  - Select Add role assignment
  - Select the **Contributor** role
- Storage Account:
  - Select the `siderolabs` Storage Account
  - Select Access control (IAM)
  - Select Add role assignment
  - Select the **Storage Blob Data Contributor** role

## Google Cloud Pre-requisites

- `GOOGLE_PROJECT_ID` - Google Cloud Project ID
- `GOOGLE_CREDENTIALS_JSON` - Google Cloud Service Account JSON
