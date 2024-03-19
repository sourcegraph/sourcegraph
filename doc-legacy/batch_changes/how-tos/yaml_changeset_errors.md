# Handling changeset yaml formatting errors

Sometimes you may encounter `Format YAML` errors when trying to run a changeset. These are often a result of misconfiguration or formatting errors. This document attempts to explain some common yaml errors and their meaning.

### Published value error:

```json
cannot publish a changeset that has a published value set in its changesetTemplate
```
This occurs when you have a `published:` field in the spec and then try to run the Publish bulk action on a batch change. Example:

```json
format-yaml
  commit:
    message: Format all YAML 
  published: false
```

Solution A) Change `published: false` to `published: true` in your spec and re-apply.

Solution B) Remove the `published` field altogether. Doing so will allow you to control the publication state from the UI.
