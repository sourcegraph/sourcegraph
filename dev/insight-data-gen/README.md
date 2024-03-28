# Code Insights Test Data Generator

This utility will help generate test repositories for code insights. This will allow you
to define deterministic data sets, and let the utility handle building the repos.

This is a use at your own discretion utility. If you find value, please use it.

## Usage

Provide a manifest file that describes the data series you want to generate.

```json
{
  "definitions": [
    {
      "repo_path": "path_to_repo",
      "symbols": [
        {
          "symbol": "insights",
          "snapshots": [
            {
              "instant": "2021-05-01T00:00:00Z",
              "count": 5
            },
            {
              "instant": "2021-06-01T00:00:00Z",
              "count": 8
            },
            {
              "instant": "2021-07-01T00:00:00Z",
              "count": 14
            }
          ]
        },
        {
          "symbol": "salami",
          "snapshots": [
            {
              "instant": "2021-05-01T00:00:00Z",
              "count": 5
            },
            {
              "instant": "2021-06-01T00:00:00Z",
              "count": 3
            },
            {
              "instant": "2021-07-01T00:00:00Z",
              "count": 0
            }
          ]
        }
      ]
    }
  ]
}
```

Point the `repo_path` field at a git repository (that is already initialized, and on the primary branch).

Run the tool `./insight-data-gen --manifest=./examples/manifest.json`

The tool will generate commits at each specified timestamp with a file named `files/findme.txt` that will have
occurrences of each symbol in the specified number of counts

```
git checkout 32e5099cfbc70024a98bfb61bbb90b325d3d5659
cat /files/findme.txt

insights
insights
insights
insights
insights
insights
insights
insights
salami
salami
salami
```

### Future Improvements

1. Handle the git repo automatically (init, branch, etc)
2. Launch `src-cli` and mount generated repos to Sourcegraph
3. Gracefully handle duplicate commits
4. Generate the manifest with some help input rather than require the manifest manually created
Hello World
