# cloud-image-uploader

## Image tags

Published images are tagged with their build type, which is what cleanup tooling keys off
of to decide how long an image should stick around:

- AWS: the tag `BuildType`, on both the AMI and the EBS snapshot backing it.
- GCP: the label `build-type` on the image. GCP label keys must be lowercase, hence the
  different spelling.

The values are the same on both:

| Value | Meaning |
| --- | --- |
| `e2e` | throwaway image built for an e2e run |
| `nightly` | branch build, versioned by `git describe` (e.g. `v1.14.0-alpha.1-101-g11a7fbe4c`) |
| `release` | tagged release, including `alpha`/`beta`/`rc` pre-releases |

The value is derived from `--tag` and `--name-prefix`.

## vmimport role

Role should be pre-created before running this command.

    aws iam create-role --role-name vmimport --assume-role-policy-document file://trust-policy.json
    aws iam put-role-policy --role-name vmimport --policy-name vmimport --policy-document file://role-policy.json

## Google Cloud Pre-requisites

- `GOOGLE_PROJECT_ID` - Google Cloud Project ID
- `GOOGLE_CREDENTIALS_JSON` - Google Cloud Service Account JSON
