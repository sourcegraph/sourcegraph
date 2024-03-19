# Using a managed object storage service (S3 or GCS)

By default, Sourcegraph will use a `sourcegraph/blobstore` server bundled with the instance to temporarily store code graph indexes uploaded by users.

You can alternatively configure your instance to instead store this data in an S3 or GCS bucket. Doing so may decrease your hosting costs as persistent volumes are often more expensive than the same storage space in an object store service.

To target a managed object storage service, you will need to set a handful of environment variables for configuration and authentication to the target service. **If you are running a sourcegraph/server deployment, set the environment variables on the server container. Otherwise, if running via Docker-compose or Kubernetes, set the environment variables on the `frontend`, `worker`, and `precise-code-intel-worker` containers.**

### Using S3

To target an S3 bucket you've already provisioned, set the following environment variables. Authentication can be done through [an access and secret key pair](https://docs.aws.amazon.com/general/latest/gr/aws-sec-cred-types.html#access-keys-and-secret-access-keys) (and optional session token), or via the EC2 metadata API.

**_Warning:_** Remember never to commit aws access keys in git. Consider using a secret handling service offered by your cloud provider. 

- `PRECISE_CODE_INTEL_UPLOAD_BACKEND=S3`
- `PRECISE_CODE_INTEL_UPLOAD_BUCKET=<my bucket name>`
- `PRECISE_CODE_INTEL_UPLOAD_AWS_ENDPOINT=https://s3.us-east-1.amazonaws.com`
- `PRECISE_CODE_INTEL_UPLOAD_AWS_ACCESS_KEY_ID=<your access key>`
- `PRECISE_CODE_INTEL_UPLOAD_AWS_SECRET_ACCESS_KEY=<your secret key>`
- `PRECISE_CODE_INTEL_UPLOAD_AWS_SESSION_TOKEN=<your session token>` (optional)
- `PRECISE_CODE_INTEL_UPLOAD_AWS_USE_EC2_ROLE_CREDENTIALS=true` (optional; set to use EC2 metadata API over static credentials)
- `PRECISE_CODE_INTEL_UPLOAD_AWS_REGION=us-east-1` (default)

**_Note:_** If a non-default region is supplied, ensure that the subdomain of the endpoint URL (_the `AWS_ENDPOINT` value_) matches the target region.

> NOTE: You don't need to set the `PRECISE_CODE_INTEL_UPLOAD_AWS_ACCESS_KEY_ID` environment variable when using `PRECISE_CODE_INTEL_UPLOAD_AWS_USE_EC2_ROLE_CREDENTIALS=true` because role credentials will be automatically resolved. Attach the IAM role to the EC2 instances hosting the `frontend`, `worker`, and `precise-code-intel-worker` containers in a multi-node environment.


### Using GCS

To target a GCS bucket you've already provisioned, set the following environment variables. Authentication is done through a service account key, supplied as either a path to a volume-mounted file, or the contents read in as an environment variable payload.

- `PRECISE_CODE_INTEL_UPLOAD_BACKEND=GCS`
- `PRECISE_CODE_INTEL_UPLOAD_BUCKET=<my bucket name>`
- `PRECISE_CODE_INTEL_UPLOAD_GCP_PROJECT_ID=<my project id>`
- `PRECISE_CODE_INTEL_UPLOAD_GOOGLE_APPLICATION_CREDENTIALS_FILE=</path/to/file>`
- `PRECISE_CODE_INTEL_UPLOAD_GOOGLE_APPLICATION_CREDENTIALS_FILE_CONTENT=<{"my": "content"}>`

### Provisioning buckets

If you would like to allow your Sourcegraph instance to control the creation and lifecycle configuration management of the target buckets, set the following environment variables:

- `PRECISE_CODE_INTEL_UPLOAD_MANAGE_BUCKET=true`
- `PRECISE_CODE_INTEL_UPLOAD_TTL=168h` (default)
