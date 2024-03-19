# Search results aggregations use cases and recipes

Here are some common use cases for the [in-search-results aggregations](../explanations/search_results_aggregations.md) powered by code insights. 

## Popular

### What versions of Go are we using most? 
See all the go versions in your codebase (group by capture group)
```sgquery
file:go\.mod$ /go\s*(\d\.\d+)/
```

### What are the open source licenses most used in our codebase? 
See all the licenses included, by frequency (group by capture group)
```sgquery
file:package.json /"license":\s(.*),/
```

### Which repositories use a specific internal library the most?
See which repositories import a library (group by repository) 
```sguqery
from '@sourcegraph/wildcard'
```

### Which directories have the longest eslint ignore file? 
See which files have the most linter override rules within a repository (group by file)
```sgquery
file:^\.eslintignore /.\n/ repo:^github\.com/sourcegraph/sourcegraph$
```

### Who knows most about a library or component? 
See who has added the most uses of a component to a specific repository in the last three months
```sgquery
nodeComponent type:diff select:commit.diff.added repo:sourcegraph/sourcegraph$ after:"3 months ago"
```


## By capture group

### What versions of log4j exist in our codebase?
See all the different subversions of log4j present in your code 
```sgquery
lang:gradle /org\.apache\.logging\.log4j['"].*?(2\.\d+\.\d+)/
```

### What breaks most commonly? 
See what topics most frequently appear in "fix [x]" commit messsages
```sgquery
type:commit repo:^github\.com/sourcegraph/sourcegraph$ after:"5 days ago" /Fix (\\w+)/
```

### What are the most common email addresses we direct users to?
See every email address hardcoded, by frequency
```sgquery
/(\w+)\@sourcegraph\.com/
```

### How can we see all our different tracer calls to remove unnecessary ones or encourage proper usage?
See all your tracer calls to track the growth of, or minimize spend on, tools like Datadog. 
```sgquery
/tracer\.trace\(([\s"'\w@\/:^.#,+-=]+)\)/
```


## By repository

### Which repositories use a specific internal library the most?
See which repositories import a library 
```sguqery
/from\s'\@sourcegraph\/wildcard/
```

### Which teams (repositories) have the most usages of a vulnerable function or library? 
```sgquery
vulnerableFunc(
```

### Which repositories have the longest top-level eslint ignore files? 
See which repositories are using the most linter overrides
```sgquery
file:^\.eslintignore /.\n/
```


## By file

### Which files should we migrate first? 
See which files have the most usage of a library you want to deprecate, such as the log15 library
```sgquery
repo:^github\.com/sourcegraph/sourcegraph$ /log15\.(?:Debug|Info|Warn|Error)/
```

### Which are our biggest package.json files? 
See which repositories have the most scripts or dependencies
```sgquery
file:package\.json ,
```

### Which directories have the longest eslint ignore file? 
See which files have the most linter override rules within a repository
```sgquery
file:^\.eslintignore /.\n/ repo:^github\.com/sourcegraph/sourcegraph$
```


## By author

### Who knows most about a library or component? 
See who has added the most uses of a component to a specific repository in the last three months
```sgquery
nodeComponent type:diff select:commit.diff.added repo:sourcegraph/sourcegraph$ after:"3 months ago"
```

### Who worked on a recent migration? 
See who most often had commits mentioning what you migrated away from (for example: migrating off bootstrap)
```sgquery
bootstrap type:commit r:sourcegraph/sourcegraph$ after:"3 months ago"
```
