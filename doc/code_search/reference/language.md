# Sourcegraph search query language

<style>

body.theme-dark img.toggle {
    filter: invert(100%);
}

img.toggle {
    width: 20px;
    height: 20px;
}

.toggle-container {
  border: 1px solid;
  border-radius: 3px;
  display: inline-flex;
  vertical-align: bottom;
}

li.r {
    margin-top:10px !important;
    list-style:none !important;
}

.r {
    border: 0px !important;
    padding: 0px !important;
    margin: 0px !important;
    border-collapse: collapse !important;
    vertical-align: top !important;
    background-color: transparent !important;
}

th.r {
    text-align: left !important;
    padding: 3px !important;
}

td.r {
    text-align: left !important;
    vertical-align: top !important;
    border: 1px solid #aca899 !important;
    padding: 3px !important;
}

.ts {
    border: 0px !important;
    padding: 0px !important;
    margin: 0px !important;
    border-collapse: collapse !important;
    vertical-align: top !important;
    width: 16px !important;
    height: 24px !important;
    background-image: url(../img/div-ts.png);
    background-size: 16px 512px !important;
    min-width: 16px; // prevent narrow screen width from removing lines
}

body.theme-dark .ts {
    background-image: url(../img/div-ts-dark.png);
}

.te {
    border: 0px !important;
    padding: 0px !important;
    margin: 0px !important;
    border-collapse: collapse !important;
    vertical-align: top !important;
    width: 16px !important;
    height: 24px !important;
    background-image: url(../img/div-te.png);
    background-size: 16px 512px !important;
    min-width: 16px; // prevent narrow screen width from removing lines
}

body.theme-dark .te {
    background-image: url(../img/div-te-dark.png);
}

.le {
    border: 0px !important;
    padding: 0px !important;
    margin: 0px !important;
    border-collapse: collapse !important;
    vertical-align: top !important;
    width: 16px !important;
    height: 24px !important;
    background-image: url(../img/div-le.png);
    background-size: 16px 512px !important;
}

body.theme-dark .le {
    background-image: url(../img/div-le-dark.png);
}

.ls {
    border: 0px !important;
    padding: 0px !important;
    margin: 0px !important;
    border-collapse: collapse !important;
    vertical-align: top !important;
    width: 16px !important;
    height: 24px !important;
    background-image: url(../img/div-ls.png);
    background-size: 16px 512px !important;
}

body.theme-dark .ls {
    background-image: url(../img/div-ls-dark.png);
}

.ke {
    border: 0px !important;
    padding: 0px !important;
    margin: 0px !important;
    border-collapse: collapse !important;
    vertical-align: top !important;
    width: 16px !important;
    height: 24px !important;
    background-image: url(../img/div-ke.png);
    background-size: 16px 512px !important;
}

body.theme-dark .ke {
    background-image: url(../img/div-ke-dark.png);
}

.ks {
    border: 0px !important;
    padding: 0px !important;
    margin: 0px !important;
    border-collapse: collapse !important;
    vertical-align: top !important;
    width: 16px !important;
    height: 24px !important;
    background-image: url(../img/div-ks.png);
    background-size: 16px 512px !important;
}

body.theme-dark .ks {
    background-image: url(../img/div-ks-dark.png);
}

.d {
    border: 0px !important;
    padding: 0px !important;
    margin: 0px !important;
    border-collapse: collapse !important;
    vertical-align: top !important;
    min-width: 16px !important;
    height: 24px !important;
    background-image: url(../img/div-d.png);
    background-size: 1024px 512px !important;
}

body.theme-dark .d {
    background-image: url(../img/div-d-dark.png);
}

.e {
    border: 0px !important;
    padding: 0px !important;
    margin: 0px !important;
    border-collapse: collapse !important;
    vertical-align: top !important;
    min-width: 16px !important;
    height: 24px !important;
    background-image: url(../img/div-e.png);
    background-size: 1024px 512px !important;
}

body.theme-dark .e {
    background-image: url(../img/div-e-dark.png);
}

.c {
    padding: 0px 3px !important;
    margin: 0px 0px !important;
    border: 2px solid !important;
    -moz-border-radius: 0.4em !important;
    -webkit-border-radius: 0.4em !important;
    -khtml-border-radius: 0.4em !important;
    border-radius: 0.4em !important;
    white-space: nowrap;
}

body.theme-dark .c {
    background-image: url(../img/div-c-dark.png);
}

</style>


This page provides a visual breakdown of our Search Query Language and a handful
of examples to get you started. It is complementary to our [syntax reference](../reference/queries.md) and illustrates syntax using railroad diagrams instead of
tables.

**How to read railroad diagrams.** Follow the lines in these railroad diagrams from left
to right to see how pieces of syntax combine. When a line splits it means there
are multiple options available. When it is possible to repeat a previous syntax,
you'll see a line lead into a box that looks like this:

<table class="r">
  <tbody>
    <tr class="r">
      <table class="r">
        <tbody>
          <tr class="r">
            <td class="ts"></td>
            <td class="d"> </td>
            <td class="te"></td>
          </tr>
          <tr class="r">
            <td class="ls"></td>
            <td class="d"><code class="c">...</code></td>
            <td class="le"></td>
          </tr>
        </tbody>
      </table>
    </tr>
  </tbody>
</table>

## Basic query

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"><code class="c"><a href="#search-pattern">search pattern</a></code></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c"><a href="#parameter">parameter</a></code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"> </td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">...</code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
      </tr>
    </tbody>
  </table>
</div>

At a basic level, a query consists of [search patterns](#search-pattern) and [parameters](#parameter). Typical queries contain one or more space-separated search patterns that describe what to search, and parameters refine searches by filtering results or changing search behavior.

**Example:** `repo:github.com/sourcegraph/sourcegraph file:schema.graphql The result` [↗](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:schema.graphql+The+result&patternType=literal)

## Expression

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="d"></td>
                <td class="d"><code class="c"><a href="#basic-query">basic query</a></code></td>
                <td class="d"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"> </td>
                <td class="d"></td>
              </tr>
              <tr class="r">
                <td class="ks"></td>
                <td class="d"><code class="c">AND</code></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">OR</code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="d"></td>
              </tr>
              <tr class="r">
                <td class="d"><code class="c"><a href="#basic-query">basic query</a></code></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="d"></td>
                <td class="d"> </td>
                <td class="d"></td>
              </tr>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"> </td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">...</code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="le"> </td>
              </tr>
            </tbody>
          </table>
        </td>
      </tr>
    </tbody>
  </table>
</div>

Build query expressions by combining [basic queries](#basic-query) and operators like `AND` or `OR`.
Group expressions with parentheses to build more complex expressions. If there are no balanced parentheses, `AND` operators bind tighter, so `foo or bar and baz` means `foo or (bar and baz)`. You may also use lowercase `and` or `or`.

**Example:** `repo:github.com/sourcegraph/sourcegraph rtr AND newRouter` [↗](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+rtr+AND+newRouter&patternType=literal)


## Search pattern

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="ts"></td>
        <td class="d"><code class="c"><a href="#string">string</a></code></td>
        <td class="te"></td>
      </tr>
      <tr class="r">
        <td class="ls"></td>
        <td class="d"><code class="c"><a href="#quoted-string">quoted string</a></code></td>
        <td class="le"></td>
      </tr>
    </tbody>
  </table>
</div>

A pattern to search. By default the pattern is searched literally. The kind of search may be toggled to change how a pattern matches:
<ul class="r">
    <li class="r"><span class="toggle-container"><img class="toggle" src="../img/regex.png"></span> Perform a [regular expression search](queries.md#regular-expression-search). We support [RE2 syntax](https://golang.org/s/re2syntax). Quoting patterns performs a literal search.<br>
    <strong>Example:</strong> <code>foo.*bar.*baz</code><a href="https://sourcegraph.com/search?q=foo+bar&patternType=regexp"> ↗</a> <code>"foo bar"</code><a href="https://sourcegraph.com/search?q=%22foo+bar%22&patternType=regexp"> ↗</a></li>
    <li class="r"><span class="toggle-container"><img class="toggle" src="../img/brackets.png"></span> Perform a structural search. See our [dedicated documentation](queries.md#structural-search) to learn more about structural sexarch. <br><strong>Example:</strong> <code>fmt.Sprintf(":[format]", :[args])</code><a href="https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+fmt.Sprintf%28%22:%5Bformat%5D%22%2C+:%5Bargs%5D%29&patternType=structural"> ↗</a></li>
</ul>


## Parameter

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="ts"></td>
        <td class="d"><code class="c"><a href="#repo">repo</a></code></td>
        <td class="te"></td>
      </tr>
      <tr class="r">
        <td class="ks"></td>
        <td class="d"><code class="c"><a href="#file">file</a></code></td>
        <td class="ke"></td>
      </tr>
      <tr class="r">
        <td class="ks"></td>
        <td class="d"><code class="c"><a href="#content">content</a></code></td>
        <td class="ke"></td>
      </tr>
      <tr class="r">
        <td class="ks"></td>
        <td class="d"><code class="c"><a href="#language">language</a></code></td>
        <td class="ke"></td>
      </tr>
      <tr class="r">
        <td class="ks"></td>
        <td class="d"><code class="c"><a href="#type">type</a></code></td>
        <td class="ke"></td>
      </tr>
      <tr class="r">
        <td class="ks"></td>
        <td class="d"><code class="c"><a href="#case">case</a></code></td>
        <td class="ke"></td>
      </tr>
      <tr class="r">
        <td class="ks"></td>
        <td class="d"><code class="c"><a href="#fork">fork</a></code></td>
        <td class="ke"></td>
      </tr>
      <tr class="r">
        <td class="ks"></td>
        <td class="d"><code class="c"><a href="#archived">archived</a></code></td>
        <td class="ke"></td>
      </tr>
      <tr class="r">
        <td class="ks"></td>
        <td class="d"><code class="c"><a href="#repogroup">repogroup</a></code></td>
        <td class="ke"></td>
        <tr class="r">
          <td class="ks"></td>
          <td class="d"><code class="c"><a href="#repo-has-file">repo has file</a></code></td>
          <td class="ke"></td>
        </tr>
        <tr class="r">
          <td class="ks"></td>
          <td class="d"><code class="c"><a href="#repo-has-commit-after">repo has commit after</a></code></td>
          <td class="ke"></td>
        </tr>
        <tr class="r">
          <td class="ks"></td>
          <td class="d"><code class="c"><a href="#count">count</a></code></td>
          <td class="ke"></td>
        </tr>
        <tr class="r">
          <td class="ks"></td>
          <td class="d"><code class="c"><a href="#timeout">timeout</a></code></td>
          <td class="ke"></td>
        </tr>
        <tr class="r">
          <td class="ks"></td>
          <td class="d"><code class="c"><a href="#visibility">visibility</a></code></td>
          <td class="ke"></td>
        </tr>
        <tr class="r">
          <td class="ls"></td>
          <td class="d"><code class="c"><a href="#pattern-type">pattern type</a></code></td>
          <td class="le"></td>
        </tr>
      </tr>
    </tbody>
  </table>
</div>

Search parameters allow you to filter search results or modify search behavior.

### Repo

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ks"></td>
                <td class="d"><code class="c">–</code></td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="ke"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">NOT</code></td>
                <td class="d">&nbsp;</td>
                <td class="d"><code class="c"><a href="#whitespace">whitespace</a></code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"><code class="c">repo:</code></td>
                <td class="te">
                </td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d">
                  <table class="r">
                    <tbody>
                      <tr class="r">
                        <td class="d"><code class="c">r:</code></td>
                        <td class="d"></td>
                        <td class="d"></td>
                      </tr>
                    </tbody>
                  </table>
                </td>
                <td class="le">
                </td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><code class="c"><a href="#regular-expression">regular expression</a></code></td>
        <td class="d">
          <td class="d">
            <table class="r">
              <tbody>
                <tr class="r">
                  <td class="ts"></td>
                  <td class="d">&nbsp;</td>
                  <td class="d">
                  <td class="d">
                  <td class="te"></td>
                </tr>
                <tr class="r">
                  <td class="ks"></td>
                  <td class="d"><code class="c">@<a href="#revision">revision</a></code></td>
                  <td class="d">
                  <td class="d">
                  <td class="ke"></td>
                </tr>
                <tr class="r">
                  <td class="ls"></td>
                  <td class="d"><code class="c"><a href="#whitespace">whitespace</a></code></td>
                  <td class="d">
                  <td class="d"><code class="c">rev:<a href="#revision">revision</a></code></td>
                  <td class="le"></td>
              </tr>
              </tbody>
            </table>
          </td>
      </tr>
    </tbody>
  </table>
</div>

Search repositories that match the regular expression.
A `-` before `repo` excludes the repository. By default
the repository will be searched at the `HEAD` commit of the default
branch. You can optionally change the [revision](#revision).

**Example:** `repo:gorilla/mux testroute` [↗](https://sourcegraph.com/search?q=repo:gorilla/mux+testroute&patternType=regexp) `-repo:gorilla/mux testroute` [↗](https://sourcegraph.com/search?q=-repo:gorilla/mux+testroute&patternType=regexp)

### Revision

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"><code class="c">branch name</code></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ks"></td>
                <td class="d"><code class="c">commit hash</code></td>
                <td class="ke"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">git tag</code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d">
          <td class="d">
            <table class="r">
              <tbody>
                <tr class="r">
                  <td class="ts"></td>
                  <td class="d">&nbsp;</td>
                  <td class="te"></td>
                </tr>
                <tr class="r">
                  <td class="ls"></td>
                  <td class="d">
                    <table class="r">
                      <tbody>
                        <tr class="r">
                          <td class="d"><code class="c">:<a href="#revision">revision</a></code></td>
                          <td class="d">
                            <table class="r">
                              <tbody>
                                <tr class="r">
                                  <td class="ts"></td>
                                  <td class="d">&nbsp;</td>
                                  <td class="te"></td>
                                </tr>
                                <tr class="r">
                                  <td class="ls"></td>
                                  <td class="d"><code class="c">...</code></td>
                                  <td class="le"></td>
                                </tr>
                              </tbody>
                            </table>
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </td>
                  <td class="le"></td>
                </tr>
              </tbody>
            </table>
          </td>
      </tr>
    </tbody>
  </table>
</div>

Search a repository at a given revision. For example, a branch name, commit hash, or git tag.

**Example:** `repo:^github\.com/gorilla/mux$@948bec34 testroute` [↗](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/gorilla/mux%24%40948bec34+testroute&patternType=literal) or `repo:^github\.com/gorilla/mux$ rev:v1.8.0 testroute` [↗](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/gorilla/mux+rev:v1.8.0+testroute&patternType=literal)

You can search multiple revisions by separating the revisions with `:`. Specify `HEAD` for the default branch.

**Example:** `repo:^github\.com/gorilla/mux$@v1.7.4:v1.4.0 testing.T` [↗](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/gorilla/mux%24%40v1.7.4:v1.4.0+testing.T&patternType=literal) or `repo:^github\.com/gorilla/mux$ rev:v1.7.4:v1.4.0 testing.T` [↗](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/gorilla/mux%24+rev:v1.7.4:v1.4.0+testing.T&patternType=literal)

### File

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ks"></td>
                <td class="d"><code class="c">–</code></td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="ke"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">NOT</code></td>
                <td class="d">&nbsp;</td>
                <td class="d"><code class="c"><a href="#whitespace">whitespace</a></code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"><code class="c">file:</code></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d">
                  <table class="r">
                    <tbody>
                      <tr class="r">
                        <td class="d"><code class="c">f:</code></td>
                        <td class="d"></td>
                        <td class="d"></td>
                      </tr>
                    </tbody>
                  </table>
                </td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><code class="c"><a href="#regular-expression">regular expression</a></code></td>
        <td class="d"></td>
      </tr>
    </tbody>
  </table>
</div>

Search files whose full path matches the regular expression. A `-` before `file`
excludes the file from being searched.

**Example:** `file:\.js$ httptest` [↗](https://sourcegraph.com/search?q=file:%5C.js%24+httptest&patternType=regexp) `file:\.js$ -file:test http` [↗](https://sourcegraph.com/search?q=file:%5C.js%24+-file:test+http&patternType=regexp)

### Language

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="ts"></td>
        <td class="d"><code class="c">language:</code></td>
        <td class="te">
        </td>
      </tr>
      <tr class="r">
        <td class="ks"></td>
        <td class="d"><code class="c">lang:</code></td>
        <td class="ke"></td>
      </tr>
      <tr class="r">
        <td class="ls"></td>
        <td class="d"><code class="c">l:</code></td>
        <td class="le"></td>
      </tr>
    </tbody>
  </table>
</div>

Only search files in the specified programming language, like `typescript` or
`python`.

**Example:** `lang:typescript encoding` [↗](https://sourcegraph.com/search?q=lang:typescript+encoding&patternType=regexp)

### Content

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"></td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ks"></td>
                <td class="d"><code class="c">–</code></td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="ke"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">NOT</code></td>
                <td class="d">&nbsp;</td>
                <td class="d"><code class="c"><a href="#whitespace">whitespace</a></code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><code class="c">content:</code></td>
        <td class="d"></td>
        <td class="d"><code class="c"><a href="#quoted-string">quoted string</a></code></td>
        <td class="d">
      </tr>
    </tbody>
  </table>
</div>

Set the search pattern to search using a dedicated parameter. Useful, for
example, when searching literally for a string like `repo:my-repo` that may
conflict with the syntax of parameters in this Sourcegraph language.

**Example:** `repo:sourcegraph content:"repo:sourcegraph"` [↗](https://sourcegraph.com/search?q=repo:sourcegraph+content:%22repo:sourcegraph%22&patternType=literal)

### Type

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"></td>
        <td class="d"><code class="c">type:</code></td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="d">
                  <table class="r">
                    <tbody>
                      <tr class="r">
                        <td class="ts"></td>
                        <td class="d"><code class="c">symbol</code></td>
                        <td class="te">
                        </td>
                      </tr>
                      <tr class="r">
                        <td class="ks"></td>
                        <td class="d"><code class="c">repo</code></td>
                        <td class="ke"></td>
                      </tr>
                      <tr class="r">
                        <td class="ks"></td>
                        <td class="d"><code class="c">path</code></td>
                        <td class="ke"></td>
                      </tr>
                      <tr class="r">
                        <td class="ks"></td>
                        <td class="d"><code class="c">file</code></td>
                        <td class="le"></td>
                      </tr>
                    </tbody>
                  </table>
                </td>
                <td class="d"></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="d">
                  <table class="r">
                    <tbody>
                      <tr class="r">
                        <td class="ks"></td>
                        <td class="d"><code class="c">commit</code></td>
                        <td class="te"></td>
                      </tr>
                      <tr class="r">
                        <td class="ls"></td>
                        <td class="d"><code class="c">diff</code></td>
                        <td class="le">
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </td>
                <td class="d"><code class="c"><a href="#commit-parameter">commit parameter</a></code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
      </tr>
    </tbody>
  </table>
</div>

Set whether the search pattern should perform a search of a certain type.
Notable search types are symbol, commit, and diff searches.

**Example:** `type:symbol path` [↗](https://sourcegraph.com/search?q=type:symbol+path) `type:commit author:nick` [↗](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph%24+type:commit+author:nick&patternType=regexp)

### Case

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"></td>
        <td class="d"><code class="c">case:</code></td>
        <td class="d"></td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"><code class="c">yes</code></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">no</code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
      </tr>
    </tbody>
  </table>
</div>

Set whether the search pattern should be treated case-sensitively. This is
synonymous with the <span class="toggle-container"><img class="toggle" src=../img/case.png></span> toggle button.

**Example:** `OPEN_FILE case:yes` [↗](https://sourcegraph.com/search?q=OPEN_FILE+case:yes)


### Fork

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"></td>
        <td class="d"><code class="c">fork:</code></td>
        <td class="d"></td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts">
                  <td class="d"><code class="c">yes</code></td>
                  <td class="te">
              </tr>
              <tr class="r">
                <td class="ks">
                  <td class="d"><code class="c">no</code></td>
                  <td class="ke">
              </tr>
              <tr class="r">
                <td class="ls">
                  <td class="d"><code class="c">only</code></td>
                  <td class="le">
              </tr>
            </tbody>
          </table>
          </td>
      </tr>
    </tbody>
  </table>
</div>

Set to `yes` if repository forks should be included or `only` if only forks
should be searched. Respository forks are excluded by default.

**Example:** `fork:yes repo:sourcegraph` [↗](https://sourcegraph.com/search?q=fork:yes+repo:sourcegraph&patternType=regexp)

### Archived

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"></td>
        <td class="d"><code class="c">archived:</code></td>
        <td class="d"></td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts">
                  <td class="d"><code class="c">yes</code></td>
                  <td class="te">
              </tr>
              <tr class="r">
                <td class="ks">
                  <td class="d"><code class="c">no</code></td>
                  <td class="ke">
              </tr>
              <tr class="r">
                <td class="ls">
                  <td class="d"><code class="c">only</code></td>
                  <td class="le">
              </tr>
            </tbody>
          </table>
          </td>
      </tr>
    </tbody>
  </table>
</div>

Set to `yes` if archived repositories should be included or `only` if only
archives should be searched. Archived repositories are excluded by default.

**Example:** `archived:only repo:sourcegraph` [↗](https://sourcegraph.com/search?q=archived:only+repo:sourcegraph&patternType=regexp)

### Repo group

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="ts"></td>
        <td class="d"><code class="c">repogroup:</code></td>
        <td class="te">
        </td>
      </tr>
      <tr class="r">
        <td class="ls"></td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="d"><code class="c">g:</code></td>
                <td class="d"></td>
                <td class="d"></td>
                <td class="d"></td>
                <td class="d"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="le">
        </td>
      </tr>
    </tbody>
  </table>
</div>

Only include results from the named group of repositories (defined by the server
admin). Same as using [repo](#repo) that matches all of the group’s
repositories. Use [repo](#repo) unless you know that the group
exists.

**Example:** `repogroup:go-gh-100 helm` [↗](https://sourcegraph.com/search?q=repogroup:go-gh-100+helm&patternType=literal)  – searches the top 100 Go repositories on GitHub, ranked by stars.

### Repo has file

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ks"></td>
                <td class="d"><code class="c">–</code></td>
                <td class="d">&nbsp;</td>
                <td class="d">&nbsp;</td>
                <td class="ke"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">NOT</code></td>
                <td class="d">&nbsp;</td>
                <td class="d"><code class="c"><a href="#whitespace">whitespace</a></code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><table class="r">
            <tbody>
              <tr class="r">
                <td class="d"></td>
                <td class="d"><code class="c">repohasfile:</code></td>
                <td class="d">
                </td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><code class="c"><a href="#regular-expression">regular expression</a></code></td>
        <td class="d">
      </tr>
    </tbody>
  </table>
</div>

Only include results from repositories that contain a matching file. This
keyword is a pure filter, so it requires at least one other search term in the
query. Note: this filter currently only works on text matches and file path
matches.

**Example:** `repohasfile:\.py file:Dockerfile$ pip` [↗](https://sourcegraph.com/search?q=repohasfile:%5C.py+file:Dockerfile%24+pip+repo:sourcegraph+&patternType=regexp)

### Repo has commit after

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"><table class="r">
            <tbody>
              <tr class="r">
                <td class="d"></td>
                <td class="d"><code class="c">repohascommitafter:</code></td>
                <td class="d">
                </td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><code class="c"><a href="#quoted-string">quoted string</a></code></td>
        <td class="d">
      </tr>
    </tbody>
  </table>
</div>

Filter out stale repositories that don’t contain commits past the specified time
frame. This parameter is experimental.

**Example:** `repo:github\.com/sourcegraph repohascommitafter:"1 week ago"` [↗](https://sourcegraph.com/search?q=context:global+repo:github%5C.com/sourcegraph+repohascommitafter:%221+week+ago%22&patternType=literal)

### Count

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"><table class="r">
            <tbody>
              <tr class="r">
                <td class="d"></td>
                <td class="d"><code class="c">count:</code></td>
                <td class="d">
                </td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><code class="c">number</code></td>
        <td class="d">
      </tr>
    </tbody>
  </table>
</div>

Retrieve at least N results. By default, Sourcegraph stops searching early and
returns if it finds a full page of results. This is desirable for most
interactive searches. To wait for all results, or to see results beyond the
first page, use the count: keyword with a larger N.

**Example:** `count:1000 function` [↗](https://sourcegraph.com/search?q=count:1000+repo:sourcegraph/sourcegraph%24+function&patternType=regexp)

### Timeout

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"><table class="r">
            <tbody>
              <tr class="r">
                <td class="d"></td>
                <td class="d"><code class="c">timeout:</code></td>
                <td class="d">
                </td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><code class="c">time value</code></td>
        <td class="d">
      </tr>
    </tbody>
  </table>
</div>

Set a search timeout. The time value is a string like 10s or 100ms, which is
parsed by the Go time
package's [ParseDuration](https://golang.org/pkg/time/#ParseDuration).
By default the timeout is set to 10 seconds, and the search will optimize for
returning results as soon as possible. The timeout value cannot be set longer
than 1 minute.

**Example:** `timeout:15s count:10000 func` [↗](https://sourcegraph.com/search?q=repo:%5Egithub.com/sourcegraph/+timeout:15s+func+count:10000)  – sets a longer timeout for a search that contains _a lot_ of results.

### Visibility

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"></td>
        <td class="d"><code class="c">visibility:</code></td>
        <td class="d"></td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts">
                  <td class="d"><code class="c">any</code></td>
                  <td class="te">
              </tr>
              <tr class="r">
                <td class="ks">
                  <td class="d"><code class="c">public</code></td>
                  <td class="ke">
              </tr>
              <tr class="r">
                <td class="ls">
                  <td class="d"><code class="c">private</code></td>
                  <td class="le">
              </tr>
            </tbody>
          </table>
          </td>
      </tr>
    </tbody>
  </table>
</div>

Filter results to only public or private repositories. The default is to include
both private and public repositories.

**Example:** `type:repo visibility:public` [↗](https://sourcegraph.com/search?q=type:repo+visibility:public&patternType=regexp)

### Pattern type

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"></td>
        <td class="d"><code class="c">patterntype:</code></td>
        <td class="d"></td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts">
                  <td class="d"><code class="c">literal</code></td>
                  <td class="te">
              </tr>
              <tr class="r">
                <td class="ks">
                  <td class="d"><code class="c">regexp</code></td>
                  <td class="ke">
              </tr>
              <tr class="r">
                <td class="ls">
                  <td class="d"><code class="c">structural</code></td>
                  <td class="le">
              </tr>
            </tbody>
          </table>
          </td>
      </tr>
    </tbody>
  </table>
</div>

Set whether the pattern should run a literal search, regular expression search,
or a structural search pattern. This parameter is available as a command-line and
accessibility option, and synonymous with the visual [search pattern](#search-pattern) toggles.
in [search pattern](#search-pattern).

## Regular expression

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"><code class="c"><a href="#string">string</a></code></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c"><a href="#quoted-string">quoted string</a></code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
      </tr>
    </tbody>
  </table>
</div>

A string that is interpreted as a <a href="https://golang.org/s/re2syntax">RE2</a> regular expression.

## String

<div name="r">
  <table>
  <tbody>
  <tr class="r">
    <td class="d"></td>
    <td class="d"><code class="c">string</code></td>
    <td class="d"></td>
  </tr>
    </tbody>
  </table>
</div>

An unquoted string is any contiguous sequence of characters not containing whitespace.


## Quoted string

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="ts"></td>
        <td class="d"><code class="c">"any string"</code></td>
        <td class="te"></td>
      </tr>
      <tr class="r">
        <td class="ls"></td>
        <td class="d"><code class="c">'any string'</code></td>
        <td class="le"></td>
      </tr>
    </tbody>
  </table>
</div>

Any string, including whitespace, may be quoted with single `'` or double `"`
quotes. Quotes can be escaped with `\`.

## Commit parameter

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"><code class="c"><a href="#author">author</a></code></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ks"></td>
                <td class="d"><code class="c"><a href="#before">before</a></code></td>
                <td class="ke"></td>
              </tr>
              <tr class="r">
                <td class="ks"></td>
                <td class="d"><code class="c"><a href="#after">after</a></code></td>
                <td class="ke"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c"><a href="#message">message</a></code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">...</code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
      </tr>
    </tbody>
  </table>
</div>

Set parameters that apply only to commit and diff searches.

### Author

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="d"></td>
                <td class="d"><code class="c">author:</code></td>
                <td class="d"></td>
                <td class="d"><code class="c"><a href="#regular expression">regular expression</a></code></td>
                <td class="d"></td>
              </tr>
            </tbody>
          </table>
        </td>
      </tr>
    </tbody>
  </table>
</div>

Include commits or diffs that are authored by the user.

### Before

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"><code class="c">before:</code></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d">
                  <table class="r">
                    <tbody>
                      <tr class="r">
                        <td class="d"><code class="c">until:</code></td>
                        <td class="d"></td>
                      </tr>
                    </tbody>
                  </table>
                </td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><code class="c"><a href="#quoted-string">quoted string</a></code></td>
        <td class="d"></td>
      </tr>
    </tbody>
  </table>
</div>

Include results which have a commit date before the specified time frame.

**Example:** `before:"last thursday"` [↗](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph%24+type:diff+author:nick+before:%22last+thursday%22&patternType=regexp) `before:"november 1 2019"` [↗](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph$+type:diff+author:nick+before:%22november+1+2019%22)

### After

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"><table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"><code class="c">after:</code></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d">
                  <table class="r">
                    <tbody>
                      <tr class="r">
                        <td class="d"><code class="c">since:</code></td>
                      </tr>
                    </tbody>
                  </table>
                </td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d"><code class="c"><a href="#quoted-string">quoted string</a></code></td>
        <td class="d"></td>
      </tr>
    </tbody>
  </table>
</div>

Include results which have a commit date before the specified time frame.

**Example:** `after:"6 weeks ago"` [↗](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph$+type:diff+author:nick+after:%226+weeks+ago%22) `after:"november 1 2019"` [↗](https://sourcegraph.com/search?q=repo:sourcegraph/sourcegraph$+type:diff+author:nick+after:%22november+1+2019%22)

### Message

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d"><table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"><code class="c">message:</code></td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ks"></td>
                <td class="d"><code class="c">msg:</code></td>
                <td class="ke"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">m:</code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
          <td class="d"><code class="c"><a href="#quoted-string">quoted string</a></code></td>
          <td class="d"></td>
        </td>
      </tr>
    </tbody>
  </table>
</div>

Include results which have commit messages containing the string.

**Example:** `type:commit message:"testing"` [↗](https://sourcegraph.com/search?q=type:commit+message:%22testing%22+repo:sourcegraph/sourcegraph%24+&patternType=regexp)

## Whitespace

<div name="r">
  <table class="r">
    <tbody>
      <tr class="r">
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="d"></td>
                <td class="d"><code class="c">space</code></td>
                <td class="d"></td>
              </tr>
            </tbody>
          </table>
        </td>
        <td class="d">
          <table class="r">
            <tbody>
              <tr class="r">
                <td class="ts"></td>
                <td class="d"> </td>
                <td class="te"></td>
              </tr>
              <tr class="r">
                <td class="ls"></td>
                <td class="d"><code class="c">...</code></td>
                <td class="le"></td>
              </tr>
            </tbody>
          </table>
        </td>
      </tr>
    </tbody>
  </table>
</div>


<br>
<sub>Attribution: The railroad diagrams use assets generated by the <a href="https://github.com/h2database/h2database">H2 Database Engine project</a> and licensed under MPL 2.0.</sub>
