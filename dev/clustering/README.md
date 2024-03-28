# Text Clustering

This directory contains Python code to cluster text data using sentence embeddings and KMeans clustering.

## Overview

The `cluster.py` script takes in a TSV file with a text field, generates sentence embeddings using the SentenceTransformers library, clusters the embeddings with KMeans, and outputs a TSV file with cluster assignments.

The goal is to group similar text snippets together into a predefined number of clusters.

## Usage

Ensure the required packges are installed:

```
pip install -r requirements.txt
```

The script accepts the following arguments:

| Argument       | Description                                      | Default    |
| -------------- | ------------------------------------------------ | ---------- |
| `--input`      | Path to input TSV file                           | _Required_ |
| `--text_field` | Name of text field in the tsv file to operate on | "text"     |
| `--clusters`   | Number of clusters to generate                   | 4          |
| `--output`     | Path for output TSV file with clusters           | _Optional_ |
| `--model`      | Sentence transformer model to use                | _Optional_ |
| `--silent`     | Whether to hide plots                            | False      |

Example

```
python cluster.py --input data.tsv --text_field chat_message --clusters 5 --output out.tsv
```

## Output

The output TSV file contains the original data plus a new "cluster" column with the assigned cluster IDs per row.

## Code Overview

**Libraries Used**

- pandas - for loading and manipulating data
- SentenceTransformers - generating embeddings
- sklearn - KMeans clustering
- matplotlib - visualization
Hello World
