# Glossary

If you've encountered a term and aren't sure what it is, start here. This is intended to describe what a thing is, where it comes from, and how it relates to our product or processes.

### [Atom](https://atom.io/)

GitHub's extensible/customizable text editor, which can be adapted to do things like [LSP](#lsp) support.

### [Buildkite](https://buildkite.com/)

Buildkite is the [continuous integration](#ci) tool we use. Our instances are actually connected to [kubernetes](#kubernetes) stuff running on Google Cloud. Buildkite runs builds automatically whenever changes show up in our trees (but only for internal pull requests, not for pull requests from third parties), and produces feedback on whether the pull breaks anything. To access our Buildkite infrastructure, [make a Buildkite account](https://buildkite.com/signup), then ask for access to the [Sourcegraph organization](https://buildkite.com/sourcegraph). Don't create a new organization when the signup process prompts; just give your email address to the person administering it. (Nick Snyder.)

### CI

Continuous Integration; run tests constantly rather than waiting for users to manually initiate them.

### [docker](https://www.docker.com/)

Container/image infrastructure to let you specify sets of things to install in virtualized machines and run, in theory improving reliability with which you can run code on arbitrary machines.

### FTT

Full-Time Teammate. Internal abbreviation used to refer to full-time staff here.

### GHE

[GitHub](#github) Enterprise. GitHub's fancy/expensive services.

### [GitHub](https://github.com/)

Large public source repository. Sourcegraph has a number of useful integrations with GitHub, also we use it for issue tracking and pull requests for many of our projects. You will need to be added to the `sourcegraph/ftts` team to get access to the internal repositories. Note that you can change notification settings, and probably should; the defaults are incredibly spammy. (Unwatch all repositories, watch issues only when pinged or participating.)

Also used to refer to the Sourcegraph integration for GitHub, available as browser extensions for Chrome and Firefox.

### [Goreman](https://github.com/mattn/goreman)

A program to run several processes in parallel, prefixing their output lines with their process names, used to run sets of related processes. A golang clone of [foreman](https://github.com/ddollar/foreman). You may prefer [a longer description of foreman](http://blog.daviddollar.org/2011/05/06/introducing-foreman.html). This is what runs the various Sourcegraph processes and keeps them in touch with each other.

### [GraphQL](http://graphql.org/)

Query language for APIs designed to allow ongoing revisions of APIs without breaking existing code. Used internally as interaction between the Sourcegraph server and other components, such as our UI.

### [Gusto](https://app.gusto.com/login)

Gusto is the third-party service which handles things like paychecks and benefits for most employees.

### [Helm](https://docs.helm.sh/)

Helm is a package manager for [kubernetes](#kubernetes).

### [Iteration](https://docs.google.com/document/d/1W7Stwye0zYX1jjMCCUdmjxDjv1OIBouBdcNyF3DAAWs/edit#)

Sourcegraph does development in roughly-four-week "iterations"; one week of planning, two weeks development, one week testing and bug fixes. (Link is to a Google Docs document describing the process. Ask for access to the doc if you don't have it already.)

### [kubernetes](https://kubernetes.io/)

Container infrastructure, used to let us build images to run that have predictable qualities and can be started and stopped on cloud infrastructure conveniently.

### [Lattice](https://sourcegraph.latticehq.com/)

Lattice is the service used to do review and goal tracking. These are done quarterly.

### [LSIF](https://lsif.dev/)

LSIF is a standard format for persisted code analyzer output. It allows a code viewing client (e.g., an editor or code browser) to provide features like autocomplete, go to definition, find references, and similar, without requiring a language analyzer to perform those computations in real-time. Today, several companies are working to support its growth, including Sourcegraph and GitHub/Microsoft, and the protocol is beginning to be used to power a rapidly growing list of language intelligence tools. Sourcegraph uses LSIF to power it's code intelligence feature.

### [LSP](https://microsoft.github.io/language-server-protocol/)

Language Server Protocol. A Microsoft spec intended to provide an easier interface for interactions between tools which need code intelligence (such as editors, IDEs, and code search tools) and tools which can provide it, such as language analyzers. Sourcegraph maintains resources for implementations and integrations at [langserver.org](https://langserver.org/).

### monorepo

A single monolithic repository holding all of a company's code, as contrasted with separate repositories for separate projects.

### [Node](https://nodejs.org/en/) / NodeJS

JavaScript runtime for using JavaScript code outside of web pages/browsers, based on Chrome's V8 JavaScript engine. Package management is handled by [Yarn](#Yarn). Used internally for running things written in JavaScript.

### [Yarn](https://yarnpkg.com/)

The [Node](#node) package management tool and ecosystem.

### [Phabricator](https://www.phacility.com/phabricator/)

Phabricator is a development workflow tool providing code review, auditing, and related functionality for use with various source control systems. There is a Sourcegraph integration for it.

### [PostgreSQL](https://www.postgresql.org/)

Relational database (SQL-based). Used in Sourcegraph for persistent data that needs to survive new installations, etcetera.

### [redis](https://redis.io/)

In-memory data store/cache, with fancy data structures and queries. Sourcegraph uses this for two data sets; `redis-store` is used for analytics and user sessions, and `redis-cache` is used for even-more transient data.

### [sourcegraph/infrastructure](https://github.com/sourcegraph/infrastructure)

The infrastructure repository is what holds configuration, setup, and tools used for [Sourcegraph Data Center](#sourcegraph-data-center), the public sourcegraph.com site, and other internal developer tools and resources. This repository does not have an issue tracker; file issues against the [main sourcegraph repository](#sourcegraph-sourcegraph).

### [sourcegraph/deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph)

The deploy-sourcegraph repo contains the tools and configuration used for deploying [Sourcegraph Data Center](#sourcegraph-data-center), especially the [Helm](#helm) charts.

### [sourcegraph/issues](https://github.com/sourcegraph/issues)

Public issue tracker. This is where customers and interested third parties would file issues.

### [sourcegraph/sourcegraph](https://github.com/sourcegraph/sourcegraph)

Our main repository, also our primary internal issue tracker.

### [TypeScript](https://www.typescriptlang.org/)

A type-checked variant of JavaScript, originating with Microsoft. Of particular interest, [the TypeScript spec](https://github.com/Microsoft/TypeScript/blob/master/doc/spec.md). Used for UI functionality of the web server which runs a Sourcegraph instance.

### [universal-ctags](https://github.com/universal-ctags/ctags)

A program which reads source files and generates indexes of symbols from them. Used by Sourcegraph for a symbol service. Could also be used by [zoekt](#zoekt) to improve search ranking, but isn't currently.

### [VSCode](https://code.visualstudio.com/)

Microsoft's "Visual Studio Code", a fairly powerful and flexible programming editor, which has [LSP](#lsp) support.

### [webpack](https://webpack.js.org/)

A "bundler" which combines multiple source files (for instance, of javascript or CSS) into single unified files that can be more easily cached. Used in Sourcegraph to improve performance of some of the built-in web interface. Can also do translations, such as converting TypeScript to JavaScript.

### [Zoekt](https://github.com/google/zoekt)

Fast search program with indexing, used in Sourcegraph for some search functionality. We also maintain [a private fork](https://github.com/sourcegraph/zoekt).

### [Zoom](https://zoom.us/)

Video conferencing software. You will need/want to download and install their application to allow joining conferences. Supports computer audio, or you can dial in with a phone.
