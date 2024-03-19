# On-disk database encryption

Sourcegraph supports encryption of database columns that contain sensitive data via the `encryption.keys` config option. It supports multiple encryption backends for different use cases & environments.

Currently supported encryption backends:

* Google Cloud KMS
* AWS KMS
* Mounted key (env var or file) AES encryption

## Enabling

To enable encryption you must specify a key config for each of the keys defined in `encryption.keys` that you wish to encrypt. You can specify the same key for all keys if you choose to, but you must at least specify config for all of them.

When you first enable encryption, new records will be written to the database as encrypted, but existing data will initially be unencrypted. Existing unencrypted records will be encrypted in the background over time. The status of this job can be checked via the `Worker > Record encrypter` dashboard in Grafana, and under **Site-admin > Background jobs**, the name of this job is **record-encrypter**.

We distinguish encrypted and unencrypted records in the database, so partially encrypted/decrypted databases are readable by the application, so enabling or disabling encryption should not impact performance or data integrity of your instance.

### Example configuration 

```json
{
  "encryption.keys": {
    // encrypts data in external_services
    "externalServiceKey": {
      "type": "mounted", // use the mounted AES encryption key
      "filePath": "/path/to/my/encryption.key" // path to a file containing your secret key
    },
    // encrypts data in user_external_accounts
    "userExternalAccountKey": {
      "type": "cloudkms", // use Google Cloud KMS
      "keyname": "/projects/my-project/name/of/my/keyring/cryptoKeys/key", // the resource name of your encryption key
      "credentialsFile": "/path/to/my/service-account.json" // path to a service account key file with the encrypter/decrypter & key viewer roles
    },
    // encrypts data in user_credentials and batch_changes_site_credentials
    "batchChangesCredentialKey": {
      // ...
    },
    // encrypts data in webhook_logs
    "webhookLogKey": {
      // ...
    }
  }
}
```

## Disabling

If you decide to disable encryption, or want to switch to a new key, you must first decrypt the database. To do so, set the environment variable `ALLOW_DECRYPTION` to `true` on the `frontend` and `worker` services. New records will be written to the database as plaintext. Existing encrypted records will be decrypted in the background over time. The status of this job can be checked the same way as enabling the initial encryption job, via the `Worker > Record encrypter` dashboard in Grafana. Once all existing records have been decrypted, the existing keys can be removed from the site configuration.

## Key rotation

If you use the Google Cloud KMS backend (or other future API based encryption backend) key rotation will be handled for you by the API. Currently key rotation is not supported in the 'mounted key' backend and instead you should disable encryption first, and then re-enable it with a new key.
