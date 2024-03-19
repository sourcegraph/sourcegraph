# Blobstore debugging tips

If you recently updated to Sourcegraph v4.2.1+, please be sure to look at the [blobstore update notes](blobstore_update_notes.md)

This page provides more tips on debugging why blobstore may not be working properly.

**Please feel free to contact support@sourcegraph.com** if you are encountering any issues and we can help you work through debugging steps.

## Can I disable blobstore?

Today, Sourcegraph uses blobstore for storing precise code intel (LSIF/SCIP) uploads. You can also configure Sourcegraph to use [S3 or GCS for object storage](../external_services/object_storage.md) if you prefer.

In the near future, other Sourcegraph features like Batch Changes may also rely on object storage and so, if possible, it's best to make sure it is working.

## How to check blobstore is working as expected

First, grab a root shell into the container, for example in a Docker Compose deployment:

```
docker exec -u 0 -it blobstore sh
```

or in a Kubernetes deployment:

```
kubectl exec -it deployment/blobstore -- sh
```

Then, try manually creating a bucket by running a curl request like this:

```
curl -i -X PUT http://127.0.0.1:9000/create-bucket-test-attempt1
```

You should see a `200 OK` success like this:

```
HTTP/1.1 200 OK
Date: Mon, 09 Jan 2023 20:41:49 GMT
x-amz-request-id: 4442587FB7D0A2F9
Location: /create-bucket-test-attempt1
Content-Length: 0
Server: Jetty(11.0.11)
```

* If that succeeds -> you are good to go and blobstore is working as expected!
* If that fails, check file permissions using the following steps.

## Checking file permissions

From a shell inside the container, you can run `ls -lah /data` to check the file permissions, look for the `lsif-uploads` line:

```
total 16K    
drwxr-xr-x    4 sourcegr sourcegr    4.0K Jan  9 20:41 .
drwxr-xr-x    1 root     root        4.0K Jan  9 20:29 ..
drwxr-x--x    2 sourcegr sourcegr    4.0K Jan  9 20:41 create-bucket-test-attempt1
drwxr-x--x    2 sourcegr sourcegr    4.0K Dec  5 22:36 lsif-uploads
```

Notice how the owner and group are `sourcegraph` - if this is shown on the `lsif-uploads` folder then you are good to go! Otherwise, you may need to correct the file permissions manually:

```sh
sudo chown -R 100:101 /data
```

## Checking frontend -> blobstore connectivity

If blobstore appears to be working fine according to all of the above, but you are still experiencing some issue, you may also check that the `frontend` container is able to talk to `blobstore` over the network. For example, by acquiring a root shell in a `sourcegraph-frontend` container and using `curl`:

```
docker exec -u 0 -it sourcegraph-frontend-internal sh
```
or

```
kubectl exec -it -u 0 sourcegraph-frontend-internal sh
```


```
curl -i http://blobstore:9000/
```

You should see a 200 response like:

```
HTTP/1.1 200 OK
Date: Mon, 09 Jan 2023 20:47:32 GMT
x-amz-request-id: 4442587FB7D0A2F9
Content-Type: application/xml;charset=utf-8
Transfer-Encoding: chunked
Server: Jetty(11.0.11)
```
