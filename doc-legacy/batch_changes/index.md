# Batch Changes

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
  background-image: url(batch_changes/file-icon.svg);
  margin-right: 0.5em;
  margin-bottom: -0.29em;
}

body.theme-dark .markdown-body ul li:before {
  filter: invert(50%);
}

</style>

<p class="subtitle">Make large-scale code changes across many repositories and code hosts</p>

<p class="lead">
Create a batch change by specifying a search query to get a list of repositories and a script to run in each. You can also <a href="how-tos/creating_changesets_per_project_in_monorepos">create a batch change on a monorepo</a> by specifying which projects to run the script on. The batch change then lets you create changesets (a generic term for pull requests or merge requests) on all affected repositories or projects. Batch Changes allows you to track their progress until they're all merged. You can preview the changes and update them at any time. A batch change can also be used to track and manage manually created changesets.
</p>

<div class="cta-group">
<a class="btn btn-primary" href="quickstart">★ Quickstart</a>
<a class="btn" href="explanations/introduction_to_batch_changes">Introduction to Batch Changes</a>
<a class="btn" href="references/requirements">Requirements</a>
</div>

## Getting started

<div class="getting-started">
  <a href="quickstart" class="btn" alt="Run through the Quickstart guide">
   <span>New to Batch Changes?</span>
   <br>
   Run through the <b>quickstart guide</b> and create a batch change in less than 10 minutes.
  </a>

  <a href="https://www.youtube.com/watch?v=GKyHYqH6ggY" class="btn" alt="Watch the Batch Changes demo video">
   <span>Demo video</span>
   <br>
   Watch the Batch Changes demo video to see what it's capable of.
  </a>

  <a href="explanations/introduction_to_batch_changes" class="btn" alt="Read the Introduction to Batch Changes">
   <span>Introduction to Batch Changes</span>
   <br>
   Find out what Batch Changes is, learn key concepts and see what others use them for.
  </a>
</div>

## Explanations

- [Introduction to Batch Changes](explanations/introduction_to_batch_changes.md)
- [Permissions in Batch Changes](explanations/permissions_in_batch_changes.md)
- [Batch Changes design](explanations/batch_changes_design.md)
- [How `src` executes a batch spec](explanations/how_src_executes_a_batch_spec.md)
- [Re-executing batch specs multiple times](explanations/reexecuting_batch_specs_multiple_times.md)
- [Running batch changes server-side](explanations/server_side.md)

## How-tos

- [Creating a batch change](how-tos/creating_a_batch_change.md)
- [Publishing changesets to the code host](how-tos/publishing_changesets.md)
- [Updating a batch change](how-tos/updating_a_batch_change.md)
- [Viewing batch changes](how-tos/viewing_batch_changes.md)
- [Tracking existing changesets](how-tos/tracking_existing_changesets.md)
- [Closing or deleting a batch change](how-tos/closing_or_deleting_a_batch_change.md)
- [Site admin configuration for Batch Changes](how-tos/site_admin_configuration.md)
- [Configuring credentials for Batch Changes](how-tos/configuring_credentials.md)
- [Handling errored changesets](how-tos/handling_errored_changesets.md)
- [Opting out of batch changes](how-tos/opting_out_of_batch_changes.md)
- [Bulk operations on changesets](how-tos/bulk_operations_on_changesets.md)
- [Using file mounts with server-side execution](how-tos/server_side_file_mounts.md)
- Batch changes in monorepos <span class="badge badge-beta">Beta</span>
  - [Creating changesets per project in monorepos](how-tos/creating_changesets_per_project_in_monorepos.md)
  - <span class="badge badge-beta">Beta</span> [Creating multiple changesets in large repositories](how-tos/creating_multiple_changesets_in_large_repositories.md)

## Tutorials

- [Refactoring Go code using Comby](tutorials/refactor_go_comby.md)
- [Updating Go import statements using Comby](tutorials/updating_go_import_statements.md)
- [Update base images in Dockerfiles](tutorials/update_base_images_in_dockerfiles.md)
- [Search and replace specific terms](tutorials/search_and_replace_specific_terms.md)
- [Examples repository](https://github.com/sourcegraph/batch-change-examples)

## References

- [Requirements](references/requirements.md)
- [Batch spec YAML reference](references/batch_spec_yaml_reference.md)
- [Batch spec templating](references/batch_spec_templating.md)
- [Batch spec cheat sheet](references/batch_spec_cheat_sheet.md)
- [Site admin configuration reference](../admin/config/batch_changes.md)
- [Troubleshooting](references/troubleshooting.md)
- [FAQ](references/faq.md)
- [CLI](../cli/references/batch/index.md)
