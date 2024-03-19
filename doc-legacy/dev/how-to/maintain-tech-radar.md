### Running the tech radar

- The radar is hosted at Thoughtworks.com, we only provide input data
- The input data is in the [tech-radar.csv](https://github.com/sourcegraph/sourcegraph/blob/main/doc/dev/radar/tech-radar.csv) file

### Adding/removing entries

- Modify [tech-radar.csv](https://github.com/sourcegraph/sourcegraph/blob/main/doc/dev/radar/tech-radar.csv)
- Preview the changes by linking your local version of the CSV file using the following address: https://radar.thoughtworks.com/?sheetId=https%3A%2F%2Fraw.githubusercontent.com%2Fsourcegraph%2Fsourcegraph%2{branch}%2Fdoc%2Fdev%2Fradar%2Ftech-radar.csv and replace {branch} wit your branch name
  - For branch "my-radar" the URL is https://radar.thoughtworks.com/?sheetId=https%3A%2F%2Fraw.githubusercontent.com%2Fsourcegraph%2Fsourcegraph%2Fmy-radar%2Fdoc%2Fdev%2Fradar%2Ftech-radar.csv
- Once you merge your branch to the main the latest radar will reflect the changes you've made

We're likely to simplify this process in the future by using `sg`.

