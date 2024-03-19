# Manage Embeddings

<p class="subtitle">Learn how you can manage embeddings via filtering and storing embedding indexes.</p>

## Filter files from embeddings

The `fileFilters` setting in the Sourcegraph embeddings configuration allows you to filter file paths meeting certain conditions from being used in generating embeddings. You can exclude files that have low information value, such as test fixtures, mocks, auto-generated files, and other irrelevant files paths  by specifying glob patterns in `excludedFilePathPatterns` and `includedFilePathPatterns`.

To use `fileFilters`, add it to your embeddings site configuration. For example, to exclude all files under `node_modules`, include only `.go` files, and limit the maximum file size to 300KB, you would add the following setting to your configuration file.

```json
{
  // [...]
  "embeddings": {
    // [...]
    "fileFilters": {
      "excludedFilePathPatterns": [
        "node_modules/"
      ],
      "includedFilePathPatterns": [
        "*.go"
      ],
      "maxFileSizeBytes": 300000 //300 KB
    }
  }
}
```

By default, the following patterns are excluded from embeddings:

- *ignore" // Files like .gitignore, .eslintignore
- .gitattributes
- .mailmap
- *.csv
- *.svg
- *.xml
- \_\_fixtures\_\_/
- node_modules/
- testdata/
- mocks/
- vendor/

> NOTE: The `excludedFilePathPatterns` setting is only available in Sourcegraph version `5.0.1` and later.

## Store embedding indexes

To store embedding indexes, you'll need to set environment variables for configuration and authentication to the target service. The settings vary depending on the service you choose.

- If you are running a `sourcegraph/server` deployment, set the environment variables on the server container
- Otherwise, if you are running via Docker-compose or Kubernetes, set the environment variables on the `frontend`, `embeddings`, and `worker` containers

Here are the storing embedding instructions for S3 and GCS:

### Using S3

To target an S3 bucket you've already provisioned, set the following environment variables. Authentication can be done through an [access and secret key pair](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html#access-keys-and-secret-access-keys) (and optional session token), or via the EC2 metadata API.

>NOTE: Make sure that you never commit your AWS access keys in Git. Instead, consider using a secret handling service offered by your cloud provider.

- `EMBEDDINGS_UPLOAD_BACKEND=S3`
- `EMBEDDINGS_UPLOAD_BUCKET=<my bucket name>`
- `EMBEDDINGS_UPLOAD_AWS_ENDPOINT=https://s3.us-east-1.amazonaws.com`
- `EMBEDDINGS_UPLOAD_AWS_ACCESS_KEY_ID=<your access key>`
- `EMBEDDINGS_UPLOAD_AWS_SECRET_ACCESS_KEY=<your secret key>`
- `EMBEDDINGS_UPLOAD_AWS_SESSION_TOKEN=<your session token>` (optional)
- `EMBEDDINGS_UPLOAD_AWS_USE_EC2_ROLE_CREDENTIALS=true` (optional — set to use EC2 metadata API over static credentials)
- `EMBEDDINGS_UPLOAD_AWS_REGION=us-east-1` (default)

>NOTE: If a non-default region is supplied, ensure that the subdomain of the endpoint URL (the `AWS_ENDPOINT` value) matches the target region.

> NOTE: You don't need to set the `EMBEDDINGS_UPLOAD_AWS_ACCESS_KEY_ID` environment variable when using `EMBEDDINGS_UPLOAD_AWS_USE_EC2_ROLE_CREDENTIALS=true` because role credentials will be automatically resolved.

### Using GCS

To target a GCS bucket you've already provisioned, set the following environment variables. Authentication is done through a service account key, supplied as either a path to a volume-mounted file, or the contents read in as an environment variable payload.

- `EMBEDDINGS_UPLOAD_BACKEND=GCS`
- `EMBEDDINGS_UPLOAD_BUCKET=<my bucket name>`
- `EMBEDDINGS_UPLOAD_GCP_PROJECT_ID=<my project id>`
- `EMBEDDINGS_UPLOAD_GOOGLE_APPLICATION_CREDENTIALS_FILE=</path/to/file>`
- `EMBEDDINGS_UPLOAD_GOOGLE_APPLICATION_CREDENTIALS_FILE_CONTENT=<{"my": "content"}>`

### Provisioning buckets

If you would like to allow your Sourcegraph instance to control the creation and lifecycle configuration management of the target buckets, set the following environment variables:

```
EMBEDDINGS_UPLOAD_MANAGE_BUCKET=true
```

### Environment variables for the `embeddings` service

`EMBEDDINGS_CACHE_SIZE` defines the maximum size of the in-memory cache that holds the embeddings for commonly-searched repos. If embeddings for a repo are larger than this size, the repo will not be held in the cache and must be re-fetched for each embeddings search. The default acceptable size is `6GiB`.
