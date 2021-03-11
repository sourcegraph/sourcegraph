# Sourcegraph enterprise/lib module

This directory is the root of a separate go module from the primary module rooted at sourcegraph/sourcegraph. This module exists to hold code that we want to reuse outside of the sourcegraph/sourcegraph repo.

Code in this module may import the `lib` project in the root of this repository, but should _not_ import any other module from sourcegraph/sourcegraph or from other Sourcegraph repositories to avoid complicated dependency relationships. Instead consider moving code from elsewhere into this module.
