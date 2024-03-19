# JVM dependencies integration with Sourcegraph

You can use Sourcegraph with JVM dependencies from any Maven repository, including Maven Central, Sonatype, or Artifactory.

This integration makes it possible to search and navigate through the source code of the JDK (Java standard library) or published Java, Scala, and Kotlin libraries (for example, [`com.google.guava:guava:27.0-jre`](https://sourcegraph.com/maven/com.google.guava/guava@v27.0-jre/-/blob/com/google/common/util/concurrent/Futures.java?L35)).

Feature | Supported?
------- | ----------
[Repository syncing](#repository-syncing) | ✅
[Repository permissions](#repository-syncing) | ❌
[Multiple JVM dependencies code hosts](#multiple-jvm-dependencies-code-hosts) | ❌

## Setup

See the "[JVM dependencies](../admin/external_service/jvm.md)" documentation.

## Repository syncing

Site admins can [add JVM packages to Sourcegraph](../admin/external_service/jvm.md#repository-syncing).

## Repository permissions

⚠️ JVM dependency repositories are visible by all users of the Sourcegraph instance.

## Multiple JVM dependencies code hosts

⚠️ It's only possible to create one JVM dependency code host for each Sourcegraph instance.

See the issue [sourcegraph#32461](https://github.com/sourcegraph/sourcegraph/issues/32461) for more details about this limitation. In most situations, it's possible to work around this limitation by configurating multiple Maven repositories to the same JVM dependency code host.
