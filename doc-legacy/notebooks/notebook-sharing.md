<style>

.markdown-body h2 {
  margin-top: 2em;
}

.markdown-body ul {
  list-style:none;
  padding-left: 1em;
}

.markdown-body ul li {
  margin: 0.5em 0;
}

.markdown-body ul li:before {
  content: '';
  display: inline-block;
  height: 1.2em;
  width: 1em;
  background-size: contain;
  background-repeat: no-repeat;
  background-image: url(code_monitoring/file-icon.svg);
  margin-right: 0.5em;
  margin-bottom: -0.29em;
}

body.theme-dark .markdown-body ul li:before {
  filter: invert(50%);
}

</style>
# Sharing Notebooks

Notebooks support the following sharing scheme.

## User namespace

### Private Notebooks
This is the default permissions level for all new notebook. Only the creator can view and edit the notebook.

### Public Notebooks
Notebooks can be shared with everyone (public notebooks on [Sourcegraph.com](https://sourcegraph.com) are viewable by anyone and don't require a Sourcegraph account), or with your entire Sourcegraph instance.

## Sourcegraph organization namespace
Find out more about Sourcegraph organizations and how to create and configure them on the [organizations docs page](../admin/organizations.md).

### Private organization Notebooks
Only organization members can view and edit the notebook.

### Public organization Notebooks
In self-hosted and managed Sourcegraph instances, everyone who has access to the instance can view the notebook. On [Sourcegraph.com](https://sourcegraph.com), anyone can view the Notebook. In both cases, only members of the owning Sourcegraph organization can edit the Notebook.

<br>

![](https://storage.googleapis.com/sourcegraph-assets/docs/images/notebooks/notebook_sharing.gif)
