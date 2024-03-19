# Known Issues

Our Known Issues page is designed to keep you informed about any current issues that could impact your use. We update this page frequently as new issues are discovered and old ones are resolved.

---

## v5.1.8 - September 4, 2023

---

- ### Sourcegraph does not recognize GitHub Enterprise Repository Visibility as described [here](https://docs.github.com/en/enterprise-server@3.10/repositories/creating-and-managing-repositories/about-repositories#about-repository-visibility).
    **Status:** [Open](https://github.com/sourcegraph/sourcegraph/pull/54419)

## v5.1.0 - June 28, 2023

---

- ### There is an issue with Sourcegraph instances configured to use explicit permissions using permissions.userMapping in Site configuration, where repository permissions are not enforced. Customers using the explicit permissions API are advised to upgrade to v5.1.1 directly.
    **Status:** [Fixed in v5.1.2](https://github.com/sourcegraph/sourcegraph/pull/54419)

- ### There is an issue with creating and updating existing Bitbucket.org (Cloud) code host connections due to problem with JSON schema validation which prevents the JSON editor from loading and surfaces as an error in the UI.
    **Status:** [Fixed in v5.1.2](https://github.com/sourcegraph/sourcegraph/pull/54496)


=======

## v5.0.1 - April 5, 2023

---

- ### Search results navigation via keyboard disabled by deafult

    **Status:** [Open](https://github.com/sourcegraph/sourcegraph/issues/51340)

- ### Editing a context that starts with a / causes a 404
    **Status:** [Fixed in v5.0.3](https://github.com/sourcegraph/sourcegraph/pull/51196)

## v5.0.0 - March 22, 2023

---

- ### Structural Search Insight shows 0 for most recent data point

    **Status:** [Fixed in 5.0.3](https://github.com/sourcegraph/sourcegraph/issues/50506)


---
