# How to troubleshoot a failure to update repositories when new repositories are added.

This Document will take you through the steps for investigating the error below while trying to Update Repositories in your Sourcegraph instance.

> Please note this is one of the ways to troubleshoot this kind of error.

For instance, You may want to add a new repository to your Sourcegraph instance and then your run into this error below, what do you do?

``` 
Warning
External service updated, but we encountered a problem while validating the external service: error in 
syncExternalService for service "GITHUB" with ID 15:context deadline exceeded
```
## Troubleshooting Steps
You check logs from the  Repo-Updater container and you should find the following below:

```status 401: Bad credentials```
## Resolution
- Look through your PATs (Personal Access Token) and you would find out that the token for syncing github to Sourcegraph has expired.
- Create a new  PAT (Personal Access Token) in Github and update the token value below with the new token generated. 
- You can also delete expired credentials.
- Refresh your Sourcegraph instance and click the ```update repositories``` button, You would see that you are able to add your new repo without any errors.

```{
  "url": "https://github.com",
  "token": "<new token>",
  "orgs": [

  ],
  "repos": [
    "xxxxxxx/xxxxxxx",

  ]
}
```
