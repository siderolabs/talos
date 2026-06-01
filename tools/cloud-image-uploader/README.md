# cloud-image-uploader

## vmimport role

Role should be pre-created before running this command.

    aws iam create-role --role-name vmimport --assume-role-policy-document file://trust-policy.json
    aws iam put-role-policy --role-name vmimport --policy-name vmimport --policy-document file://role-policy.json

## Google Cloud Pre-requisites

- `GOOGLE_PROJECT_ID` - Google Cloud Project ID
- `GOOGLE_CREDENTIALS_JSON` - Google Cloud Service Account JSON
