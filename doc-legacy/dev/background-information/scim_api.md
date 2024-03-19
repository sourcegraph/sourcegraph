# Developing the SCIM API

SCIM is a REST API specification standard for provisioning and deprovisioning users. It can also handle group (de)provisioning, but we don't support that yet.

If you're just getting started with SCIM, watch [this](https://www.youtube.com/watch?v=LBZPPBOdImU) (1.5min) and [this](https://www.youtube.com/watch?v=ID-ApdGu2Hc) (2.5 min) video.

IETF RFCs [7642](https://www.rfc-editor.org/rfc/rfc7642), [7643](https://www.rfc-editor.org/rfc/rfc7643), and [7644](https://www.rfc-editor.org/rfc/rfc7644) are pretty dry, but very useful reference materials.

Our implementation is in [this folder](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/internal/scim).

To try it, follow the [How to use section of our SCIM admin guide](../../admin/scim.md#how-to-use), then see the [manual-testing-with-postman](#manual-testing-with-postman) section below to see it in action. It should take you 10–20 minutes to set up everything and see the full CRUD magic happen.

To inspect the requests and responses, you can use [ngrok](https://ngrok.com/). You can then open your browser at http://127.0.0.1:4040/inspect/http to dive deep into the details.

> WARNING: Running the tests and validators referenced on this page will trigger sending live emails. Be sure to set the environment variables `DISABLE_EMAIL_INVITES=true` or `DEBUG_EMAIL_INVITES_MOCK=true` when running live SCIM API calls if you don't want emails to go out.

## Manual testing with Postman

Postman collection for testing SCIM API locally or through ngrok: [scim_postman_collection.json](scim_postman_collection.json). Steps to use it:

- [Download](https://www.postman.com/) and run Postman
- Go to `File | Import...` and import the JSON.
- You can import it into Postman and run the requests from top to bottom.
- Go to `SCIM | Variables` and set your token and optionally your SCIM base URL, in the format of `https://{sourcegraph URL}/.api/scim/v2`.
- For a quick test, run "User create" and "User delete".
- For a complete test, run all requests from top to bottom. They should all pass.

You can also just use [cURL](https://curl.se/) if you prefer a CLI tool.

## Validators

The creators of major IdPs supply validators that one can use to test their SCIM implementation.

We used three validators when testing our implementation: two for Okta and one for Azure AD.

1. Okta SPEC test – Follow [this guide](https://developer.okta.com/docs/guides/scim-provisioning-integration-prepare/main/#test-your-scim-api) to set it up in five minutes. Tests should be green.
2. Okta CRUD test – Follow [this guide](https://developer.okta.com/docs/guides/scim-provisioning-integration-test/main/) to set up these tests in your Runscope. This also needs access to an Okta application, which you can find [here](https://dev-433675-admin.oktapreview.com/admin/app/dev-433675_k8ssgdevorgsamlscim_1/instance/0oa1l85zn9a0tgzKP0h8/). Log in with shared credentials in 1Password. Tests should be green, except for the last ones that rely on soft deletion which we don't support yet.
3. Azure AD validator – It's [here](https://scimvalidator.microsoft.com). It doesn't have a lot of docs. Just use "Discover my schema", then enter the endpoint (for example, https://sourcegraph.test:3443/search/.api/scim/v2) and the token you have in your settings. It should work right away, and all tests should pass.

## Publishing on Okta

Our integration is [here](https://oinmanager.okta.com/app-integration/1731). To access this page, you will need a bit of a hackery:

1. Log in to the Okta test instance [here](https://dev-433675-admin.oktapreview.com/) with the l/p you find in LastPass
2. Go to Directory → [People](https://dev-433675-admin.oktapreview.com/admin/users), and click on the "Add Person" button
3. Add your own name and email, and click "Save"
4. Still with the admin account, open your new user by clicking on it, go to the "Admin roles" tab, and make your user an "Application Administrator".
5. Save, log out, confirm your email, and log in with your new user. Now you should be able to access the [app integration](https://oinmanager.okta.com/app-integration/1731).

Make changes to it as needed, and submit your changes for review. We haven't tested this process, so this territory is uncharted.
