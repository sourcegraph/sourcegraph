# Keyword Search

<p class="subtitle">Learn how Cody makes use to Keyword Search to gather context.</p>

Keyword search is the traditional approach to text search. It splits content into terms and builds a mapping from terms to documents. At query time, it extracts terms from the query and uses the mapping to retrieve your documents.

Both Cody chat and completions use Keyword Search. It comes out of the box without any additional setup. While powerful, this method can be used as a fallback solution when a codebase lacks embeddings with enhanced context quality and decreased latency over time.

## Keyword Search vs Embeddings

Embeddings search over your entire repo set. While Cody with Keyword Search only searches your local VS Code workspace.

While setting things up, Keyword Search as the default experience is a cost-effective and time-saving solution. Embeddings need to be produced and managed. It's a time-consuming process.

The codebase is divided into 20-30 line chunks, each run through an embedding service. The results must then be stored on an accessible machine for local Cody clients.

For an enterprise admin who has set up Cody with a Code Search instance, developers on their local machines can seamlessly access it. However, indexing a codebase to produce embeddings can take minutes or even hours for users with Cody solely on their local machine.

Cody employs keyword-based context to ensure a good user experience until embeddings become available. When they are accessible, Cody will use them, thereby enhancing the quality of responses.
