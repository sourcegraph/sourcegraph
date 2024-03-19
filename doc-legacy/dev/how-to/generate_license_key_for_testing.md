# How to generate a license key for testing and debugging

During development, you can generate a license key locally to test and debug how some features, UI components, alerts, errors, and logs are (or will be) displayed to users depending on their purchased plan.

The steps below are only for development and won't work in production. 

1. Choose the plan you want to test. Available options include `business-0` and `enterprise-1`, among others. You can see the whole list [here](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/licensing/data.go?L3&subtree=true).

2. Run the following command from your terminal in the root directory of the `/sourcegraph/sourcegraph` repository. Replace `business-0` with the name of any other plan that you want to test. This command will use the private key set on the `dev-private` repository in the `site-config.json` file, so you need to have a local clone of this repository.

          go run ./internal/license/generate-license.go -private-key ../dev-private/enterprise/dev/test-license-generation-key.pem -tags=plan:business-0 -users=10 -expires=8784h .

3. Confirm that the license generated has the correct information. For the example above, the output was: 

    ````
    # License info (encoded and signed in license key)
    {
      "t": [
       "plan:business-0"
     ],
     "u": 10,
     "e": "2023-09-07T19:28:06Z"
    }

    # License key
    <the license key> 
    ````

4. Copy and paste the new license key (just the block under "# License key") to the `site-config.json` file in your local clone of `dev-private.`

````
{
  "auth.providers": [
   {
     "type": "builtin",
     "allowSignup": true
   }
 ],
 "licenseKey": "<paste here the newly generated license key> ",
 // ... 
}
````

5. Restart your local instance. 

6. Go to the admin page and click on `Configuration > License` to confirm that the new license is active.
