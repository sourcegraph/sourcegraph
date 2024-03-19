# How to diagnose an `Unknown Error` during login to your Sourcegraph instance

This document will attempt to identify a common reason for an `Unknown Error` when attempting to login using a username and password to your organizations Sourcegraph instance.

## Prerequisites

* This document assumes that Sourcegraph is set up and you are trying to login via an existing username and password
* For the step to check the site config, you will need to ask your organizations site-admin to check for you (this may not be required) 

## Steps to identify and remedy

### Symptoms

1. You are attempting to login with your username and password and see an `Unknown Error` message above the username and password boxes 

2. This is often a result of an `http/https` protocol mismatch between what is in the site configuration for `externalURL`, and what the user is trying to log in to

	- For example, if the protocol in the site configuration is set to `https://sourcegraph.your_instance_name.com` 
	- And the user is trying to login at the `http://sourcegraph.your_instance_name.com`, then this error can be displayed

### Things to check

1. Check your Browsers URL bar for a message such as `Not Secure` indicating that you are trying to authenticate via `http` instead of `https`
	- For example, on a Chrome browser you would see the message on the top left hand corner of the URL bar

2. If you were trying to login with `http` protocol, try again with `https` instead

3. If you are still having issues, you can ask a site-admin to double check the `externalURL` configured in the Site configuration so you have the correct login landing page
	- See [Configuring the external URL](https://docs.sourcegraph.com/admin/url)


## Further resources

* [Sourcegraph - Site configuration](https://docs.sourcegraph.com/admin/config/site_config)
