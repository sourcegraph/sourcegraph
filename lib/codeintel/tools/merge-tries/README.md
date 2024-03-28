A little program to check how many rows we could save by merging tries inside our
`codeintel_scip_symbol_names` table.

Upload an index to your local Sourcegraph instance and run:

```
psql --no-psqlrc --csv --dbname=sourcegraph --command="select * from codeintel_scip_symbol_names where upload_id = (select max(upload_id) from codeintel_scip_symbol_names)" | cargo run --release
```

after uploading the index for `sourcegraph/sourcegraph` you'd see the following output:

```
   Compiling merge-tries v0.1.0 (/Users/creek/work/merge-tries)
    Finished release [optimized] target(s) in 1.96s
     Running `target/release/merge-tries`
from 125334 rows
to   100428 rows
reduction by 19.87%
```
Hello World
