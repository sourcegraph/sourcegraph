# Sourcegraph GraphQL API examples

This page demonstrates a few example GraphQL queries for the [Sourcegraph GraphQL API](index.md). For help with the API, you can use `Ctrl + space` to trigger tooltips. 

<table class="table" style="table-layout: fixed">
	<tr>
		<th>GraphQL query</th>
		<th>Description</th>
		<th>Example use case</th>
	</tr>
	<tr>
  	<td style="width:33%">
  		<a href="https://sourcegraph.com/api/console#%7B%22query%22%3A%22query%20%7B%5Cn%20%20repository(name%3A%20%5C%22github.com%2Fuber%2Freact-map-gl%5C%22)%20%7B%5Cn%20%20%20%20defaultBranch%20%7B%5Cn%20%20%20%20%20%20target%20%7B%5Cn%20%20%20%20%20%20%20%20commit%20%7B%5Cn%20%20%20%20%20%20%20%20%20%20blob(path%3A%20%5C%22README.md%5C%22)%20%7B%5Cn%20%20%20%20%20%20%20%20%20%20%20%20content%5Cn%20%20%20%20%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20%7D%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%7D%5Cn%22%7D">Get the contents of a file on the default branch</a>
  	</td>
  	<td>Returns the file contents.</td>
  	<td>Quickly fetch file contents without cloning a repository or hitting your code host API (which is usually slower or more rate-limited than Sourcegraph).</td>
  </tr>
	<tr>
		<td>
			<a href="https://sourcegraph.com/api/console#%7B%22query%22%3A%22query%20(%24query%3A%20String!)%20%7B%5Cn%20%20search(query%3A%20%24query%2C%20version%3A%20V2)%20%7B%5Cn%20%20%20%20results%20%7B%5Cn%20%20%20%20%20%20results%20%7B%5Cn%20%20%20%20%20%20%20%20__typename%5Cn%20%20%20%20%20%20%20%20...%20on%20FileMatch%20%7B%5Cn%20%20%20%20%20%20%20%20%20%20...FileMatchFields%5Cn%20%20%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20%20%20...%20on%20CommitSearchResult%20%7B%5Cn%20%20%20%20%20%20%20%20%20%20...CommitSearchResultFields%5Cn%20%20%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20%20%20...%20on%20Repository%20%7B%5Cn%20%20%20%20%20%20%20%20%20%20...RepositoryFields%5Cn%20%20%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20limitHit%5Cn%20%20%20%20%20%20cloning%20%7B%5Cn%20%20%20%20%20%20%20%20name%5Cn%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20missing%20%7B%5Cn%20%20%20%20%20%20%20%20name%5Cn%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20timedout%20%7B%5Cn%20%20%20%20%20%20%20%20name%5Cn%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20matchCount%5Cn%20%20%20%20%20%20elapsedMilliseconds%5Cn%20%20%20%20%20%20...SearchResultsAlertFields%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%7D%5Cn%5Cnfragment%20FileMatchFields%20on%20FileMatch%20%7B%5Cn%20%20repository%20%7B%5Cn%20%20%20%20name%5Cn%20%20%20%20url%5Cn%20%20%7D%5Cn%20%20file%20%7B%5Cn%20%20%20%20name%5Cn%20%20%20%20path%5Cn%20%20%20%20url%5Cn%20%20%20%20content%5Cn%20%20%20%20commit%20%7B%5Cn%20%20%20%20%20%20oid%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%20%20lineMatches%20%7B%5Cn%20%20%20%20preview%5Cn%20%20%20%20lineNumber%5Cn%20%20%20%20offsetAndLengths%5Cn%20%20%20%20limitHit%5Cn%20%20%7D%5Cn%7D%5Cn%5Cnfragment%20CommitSearchResultFields%20on%20CommitSearchResult%20%7B%5Cn%20%20messagePreview%20%7B%5Cn%20%20%20%20value%5Cn%20%20%20%20highlights%20%7B%5Cn%20%20%20%20%20%20line%5Cn%20%20%20%20%20%20character%5Cn%20%20%20%20%20%20length%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%20%20diffPreview%20%7B%5Cn%20%20%20%20value%5Cn%20%20%20%20highlights%20%7B%5Cn%20%20%20%20%20%20line%5Cn%20%20%20%20%20%20character%5Cn%20%20%20%20%20%20length%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%20%20label%20%7B%5Cn%20%20%20%20html%5Cn%20%20%7D%5Cn%20%20url%5Cn%20%20matches%20%7B%5Cn%20%20%20%20url%5Cn%20%20%20%20body%20%7B%5Cn%20%20%20%20%20%20html%5Cn%20%20%20%20%20%20text%5Cn%20%20%20%20%7D%5Cn%20%20%20%20highlights%20%7B%5Cn%20%20%20%20%20%20character%5Cn%20%20%20%20%20%20line%5Cn%20%20%20%20%20%20length%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%20%20commit%20%7B%5Cn%20%20%20%20repository%20%7B%5Cn%20%20%20%20%20%20name%5Cn%20%20%20%20%7D%5Cn%20%20%20%20oid%5Cn%20%20%20%20url%5Cn%20%20%20%20subject%5Cn%20%20%20%20author%20%7B%5Cn%20%20%20%20%20%20date%5Cn%20%20%20%20%20%20person%20%7B%5Cn%20%20%20%20%20%20%20%20displayName%5Cn%20%20%20%20%20%20%7D%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%7D%5Cn%5Cnfragment%20RepositoryFields%20on%20Repository%20%7B%5Cn%20%20name%5Cn%20%20url%5Cn%20%20externalURLs%20%7B%5Cn%20%20%20%20serviceType%5Cn%20%20%20%20url%5Cn%20%20%7D%5Cn%20%20label%20%7B%5Cn%20%20%20%20html%5Cn%20%20%7D%5Cn%7D%5Cn%5Cnfragment%20SearchResultsAlertFields%20on%20SearchResults%20%7B%5Cn%20%20alert%20%7B%5Cn%20%20%20%20title%5Cn%20%20%20%20description%5Cn%20%20%20%20proposedQueries%20%7B%5Cn%20%20%20%20%20%20description%5Cn%20%20%20%20%20%20query%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%7D%5Cn%22%2C%22variables%22%3A%22%7B%5Cn%20%20%5C%22query%5C%22%3A%20%5C%22repo%3A%5Egithub.com%2Fgorilla%2Fmux%24%20Router%5C%22%5Cn%7D%22%7D">
				Perform a search query and get results
			</a>
		</td>
		<td>
			Returns the search result metadata (whether or not the search result limit was hit, if the search timed out, etc.) and the actual results of the search query, which can be one of three types: <code>Repository</code>, <code>FileMatch</code>, or <code>CommitSearchResult</code>.
		</td>
		<td>
			Search for a new framework or API that you are using (or have deprecated) and determine all of the repositories that haven't yet been migrated.
		</td>
	</tr>
  <tr>
  	<td>
  		<a href="https://sourcegraph.com/api/console#%7B%22query%22%3A%22query%20%7B%5Cn%20%20repository(name%3A%20%5C%22github.com%2Fuber%2Freact-map-gl%5C%22)%20%7B%5Cn%20%20%20%20comparison(base%3A%20%5C%22be0e126b~3%5C%22%2C%20head%3A%20%5C%22be0e126b%5C%22)%20%7B%5Cn%20%20%20%20%20%20fileDiffs%20%7B%5Cn%20%20%20%20%20%20%20%20nodes%20%7B%5Cn%20%20%20%20%20%20%20%20%20%20oldPath%5Cn%20%20%20%20%20%20%20%20%20%20newPath%5Cn%20%20%20%20%20%20%20%20%20%20hunks%20%7B%5Cn%20%20%20%20%20%20%20%20%20%20%20%20body%5Cn%20%20%20%20%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20%7D%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%7D%5Cn%22%7D">Compare 2 commits</a>
  	</td>
  	<td>Returns a list of changes between 2 commits.</td>
  	<td>Scan diffs between the old and new versions of a deployed service for changes that might indicate an incompatibility (e.g., in a service discovery manifest).</td>
  </tr>
	<tr>
		<td>
			<a href="https://sourcegraph.com/api/console#%7B%22query%22%3A%22%7B%5Cn%20%20repositories(first%3A%201000)%20%7B%5Cn%20%20%20%20nodes%20%7B%5Cn%20%20%20%20%20%20name%5Cn%20%20%20%20%20%20description%5Cn%20%20%20%20%20%20url%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%7D%5Cn%22%2C%22variables%22%3A%22%22%2C%22operationName%22%3Anull%7D">
				List the first 1,000 repositories
			</a>
		</td>
		<td>
			Returns the name, description, and URL of the first 1,000 repositories on the Sourcegraph server (in order of their creation date).
		</td>
		<td>
		</td>
	</tr>
	<tr>
		<td>
			<a href="https://sourcegraph.com/api/console#%7B%22query%22%3A%22query%20ListFiles(%24repoName%3A%20String!)%20%7B%5Cn%20%20repository(name%3A%20%24repoName)%20%7B%5Cn%20%20%20%20commit(rev%3A%20%5C%22HEAD%5C%22)%20%7B%5Cn%20%20%20%20%20%20tree(path%3A%20%5C%22%5C%22%2C%20recursive%3A%20true)%20%7B%5Cn%20%20%20%20%20%20%20%20entries%20%7B%5Cn%20%20%20%20%20%20%20%20%20%20path%5Cn%20%20%20%20%20%20%20%20%20%20isDirectory%5Cn%20%20%20%20%20%20%20%20%20%20url%5Cn%20%20%20%20%20%20%20%20%7D%5Cn%20%20%20%20%20%20%7D%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%7D%5Cn%22%2C%22variables%22%3A%22%7B%5C%22repoName%5C%22%3A%20%5C%22github.com%2Fgorilla%2Fmux%5C%22%7D%22%2C%22operationName%22%3A%22ListFiles%22%7D">
				List all files in a repository
			</a>
		</td>
		<td>
			Returns every file in the repository, its path (relative to the repository root), whether or not it is a directory or plain file, and what the URL path to the file is.
		</td>
		<td>
			List all of the files in each repository of your organization (when combined with the "List the first 1000 repositories" example above) to determine which of your repositories are missing important files like READMEs, LICENSEs, and Dockerfiles.
		</td>
	</tr>
	<tr>
		<td>
			<a href="https://sourcegraph.com/api/console#%7B%22query%22%3A%22query%20ListLanguages(%24repoName%3A%20String!)%20%7B%5Cn%20%20repository(name%3A%20%24repoName)%20%7B%5Cn%20%20%20%20language%5Cn%20%20%20%20commit(rev%3A%20%5C%22HEAD%5C%22)%20%7B%5Cn%20%20%20%20%20%20languages%5Cn%20%20%20%20%7D%5Cn%20%20%7D%5Cn%7D%5Cn%22%2C%22variables%22%3A%22%7B%5C%22repoName%5C%22%3A%20%5C%22github.com%2Fgorilla%2Fmux%5C%22%7D%22%2C%22operationName%22%3A%22ListLanguages%22%7D">
				List the languages used in a repository
			</a>
		</td>
		<td>
			Returns the primary language of the repository as well as a list of all the languages used in the repository.
		</td>
		<td>
			List all of the languages in each repository of your organization (when combined with the "List the first 1000 repositories" example above) to determine how many repos use each language across your entire organization.
		</td>
	</tr>
  	<tr>
		<td>
			<a href="https://sourcegraph.com/api/console#%7B%22query%22%3A%22%23%20Type%20queries%20here%2C%20with%20completion%2C%20validation%2C%20and%20hovers.%5Cn%23%5Cn%23%20Here's%20an%20example%20query%20to%20get%20you%20started%3A%5Cn%5Cnquery%20%7B%5Cn%20%20users(activePeriod%3A%20THIS_MONTH)%20%7B%5Cn%20%20%20%20totalCount%5Cn%20%20%20%20nodes%20%7B%5Cn%20%20%20%20%20%20username%2C%5Cn%20%20%20%20%20%20usageStatistics%20%7B%5Cn%20%20%20%20%20%20%20%20lastActiveTime%5Cn%20%20%20%20%20%20%7D%5Cn%20%20%20%20%5Ct%7D%5Cn%20%20%7D%5Cn%7D%22%7D">
				Current month's active users
			</a>
		</td>
		<td>
			Returns the current month's active users (total count and usernames) and their last active time.
		</td>
		<td>
		</td>
	</tr>

</table>
