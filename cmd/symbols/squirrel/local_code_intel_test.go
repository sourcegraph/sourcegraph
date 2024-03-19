package squirrel

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestLocalCodeIntel(t *testing.T) {
	type pathContents struct {
		path     string
		contents string
	}

	tests := []pathContents{
		{
			path: "test.java",
			contents: `
class Foo {

    //             v f1.p def
    //             v f1.p ref
    void f1(String p) {

        //     v f1.x def
        //     v f1.x ref
        //         v f1.p ref
        String x = p;
    }

    //             v f2.p def
    //             v f2.p ref
    void f2(String p) {

        //     v f2.x def
        //     v f2.x ref
        //         v f2.p ref
        String x = p;
    }
}
`,
		}, {
			path: "test.go",
			contents: `
var x = 5

//      v f1.p def
//      v f1.p ref
func f1(p int) {

	//  v f1.x def
	//  v f1.x ref
	var x int

	// v f1.y def
	// v f1.y ref
	_, y := g() // < "_" f1.y def < "_" f1.y ref

	//  v f1.i def
	//  v f1.i ref
	//     v f1.j def
	//     v f1.j ref
	for i, j := range z {

		//          v f1.p ref
		//             v f1.i ref
		//                v f1.j ref
		//                   v f1.x ref
		//                      v f1.y ref
		fmt.Println(p, i, j, x, y)
	}

	//     v f1.x ref
	switch x {
	case 3:
		//  v f1.switch1.x def
		//  v f1.switch1.x ref
		var x int
	}

	select {
	//   v f1.switch2.x def
	//   v f1.switch2.x ref
	case x := <-ch:
	}
}
`,
		}, {
			path: "test.cs",
			contents: `
namespace Foo {
    class Bar {

        //                  v Baz.p def
        //                  v Baz.p ref
        static void Baz(int p) {

            //  v Baz.x def
            //  v Baz.x ref
            int x = 5;

            //                       v Baz.p ref
            //                          v Baz.x ref
            System.Console.WriteLine(p, x);

            //       v Baz.i def
            //       v Baz.i ref
            for (int i = 0; ; ) { }

			//           v Baz.e def
			//           v Baz.e ref
			foreach (int e in es) { }

            //         v Baz.r def
            //         v Baz.r ref
            using (var r = new StringReader("foo")) { }

			try { }
			//               v Baz.e def
			//               v Baz.e ref
			catch (Exception e) { }
        }
    }
}
`,
		}, {
			path: "exception.py",
			contents: `
def f():
	try:
		pass
	#                   v f.e def
	#                   v f.e ref
	except Exception as e:
		#     v f.e ref
		print(e)
`,
		}, {
			path: "test.py",
			contents: `
#     vv f.p1 def
#     vv f.p1 ref
#         vv f.p2 def
#         vv f.p2 ref
#                   vv f.p3 def
#                   vv f.p3 ref
#                               vv f.p4 def
#                               vv f.p4 ref
def f(p1, p2: bool, p3 = False, p4: bool = False):
	#     vv f.p1 ref
	#         vv f.p2 ref
	#             vv f.p3 ref
	#                 vv f.p4 ref
	print(p1, p2, p3, p4)

	x = 5 # < "x" f.x def < "x" f.x ref

	#     v f.x ref
	print(x)

	#   v f.i def
	#   v f.i ref
	for i in range(10):
		#     v f.i ref
		print(i)

	try:
		pass
	#                   v f.e def
	#                   v f.e ref
	except Exception as e:
		#     v f.e ref
		print(e)

	#     v f.j ref
	#           v f.j def
	#           v f.j ref
	print(j for j in range(10))

	#      v f.k ref
	#            v f.k def
	#            v f.k ref
	print([k for k in range(10)])
`,
		}, {
			path: "test.js",
			contents: `
//         vv f.p1 def
//         vv f.p1 ref
//             vv f.p2 def
//             vv f.p2 ref
//                        vv f.p3 def
//                        vv f.p3 ref
const f = (p1, p2 = 3, ...p3) => {
	//          vv f.p1 ref
	//              vv f.p2 ref
	//                  vv f.p3 ref
	console.log(p1, p2, p3)

	//    v f.x def
	//    v f.x ref
	const x = 5

	//       v f.g def
	//       v f.g ref
	function g() {}

	// "g" here should be a reference to the function, but the way locals are modeled isn't sophisticated
	// enough (yet?) to express bindings that also escape their lexical scope.

	//          v f.x ref
	console.log(x, g)

	//       v f.i def
	//       v f.i ref
	for (let i = 0; ; ) {
		//          v f.i ref
		console.log(i)
	}

	try { }
	//     v f.e def
	//     v f.e ref
	catch (e) {
		//          v f.e ref
		console.log(e)
	}
}
`,
		}, {
			path: "test.ts",
			contents: `
//         vv f.p1 def
//         vv f.p1 ref
//                      vv f.p2 def
//                      vv f.p2 ref
//                                 vv f.p3 def
//                                 vv f.p3 ref
const f = (p1?: number, p2 = 3, ...p3) => {
	//          vv f.p1 ref
	//              vv f.p2 ref
	//                  vv f.p3 ref
	console.log(p1, p2, p3)

	//    v f.x def
	//    v f.x ref
	const x: number = 5

	//       v f.g def
	//       v f.g ref
	function g() {}

	// "g" here should be a reference to the function, but the way locals are modeled isn't sophisticated
	// enough (yet?) to express bindings that also escape their lexical scope.

	//          v f.x ref
	console.log(x, g)

	//       v f.i def
	//       v f.i ref
	for (let i = 0; ; ) {
		//          v f.i ref
		console.log(i)
	}

	try { }
	//     v f.e def
	//     v f.e ref
	catch (e) {
		//          v f.e ref
		console.log(e)
	}
}
`,
		}, {
			path: "test.cpp",
			contents: `
//         vv f.p1 def
//         vv f.p1 ref
//                 vv f.p2 def
//                 vv f.p2 ref
//                              vv f.p3 def
//                              vv f.p3 ref
//                                       vv f.p4 def
//                                       vv f.p4 ref
void f(int p1, int p2 = 3, int& p3, int* p4)
{
	//  v f.x def
	//  v f.x ref
    int x;

	//  v f.y def
	//  v f.y ref
    int y = 5;

	//       v f.i def
	//       v f.i ref
    for (int i = 0; ; ) { }

	//       v f.j def
	//       v f.j ref
    for (int j : 3) { }

	//   v f.g def
	//   v f.g ref
	//              v f.a def
	//              v f.a ref
	auto g = [](int a) { };

	//                                   v f.e def
	//                                   v f.e ref
    try { } catch (const std::exception& e) { }
}
`,
		}, {
			path: "test.rb",
			contents: `
//    vv f.p1 def
//    vv f.p1 ref
//        vv f.p2 def
//        vv f.p2 ref
//                 vv f.p3 def
//                 vv f.p3 ref
//                       vv f.p4 def
//                       vv f.p4 ref
def f(p1, p2 = 3, *p3, **p4)
	x = 5 # < "x" f.x ref < "x" f.x def

	#         v f.x2 def
	#         v f.x2 ref
	lambda { |x| 5 }

	#          v f.x3 def
	#          v f.x3 ref
	lambda do |x| 5 end

	#   v f.i def
	#   v f.i ref
	for i in 1..5 do end

	begin
		raise ArgumentError
		#                   v f.e def
		#                   v f.e ref
	rescue ArgumentError => e
		#    v f.e ref
		puts e
	end
end
`,
		},
	}

	for _, test := range tests {
		path := types.RepoCommitPath{Repo: "foo", Commit: "bar", Path: test.path}
		want := collectAnnotations(path, test.contents)
		payload := getLocalCodeIntel(t, path, test.contents)
		got := []annotation{}
		for _, symbol := range payload.Symbols {
			got = append(got, annotation{
				repoCommitPathPoint: types.RepoCommitPathPoint{
					RepoCommitPath: path,
					Point: types.Point{
						Row:    symbol.Def.Row,
						Column: symbol.Def.Column,
					},
				},
				symbol: "(unused)",
				tags:   []string{"def"},
			})

			for _, ref := range symbol.Refs {
				got = append(got, annotation{
					repoCommitPathPoint: types.RepoCommitPathPoint{
						RepoCommitPath: path,
						Point: types.Point{
							Row:    ref.Row,
							Column: ref.Column,
						},
					},
					symbol: "(unused)",
					tags:   []string{"ref"},
				})
			}
		}

		sortAnnotations(want)
		sortAnnotations(got)

		if diff := cmp.Diff(want, got, compareAnnotations); diff != "" {
			t.Fatalf("unexpected annotations (-want +got):\n%s", diff)
		}
	}
}

func getLocalCodeIntel(t *testing.T, path types.RepoCommitPath, contents string) *types.LocalCodeIntelPayload {
	readFile := func(ctx context.Context, path types.RepoCommitPath) ([]byte, error) {
		return []byte(contents), nil
	}

	squirrel := New(readFile, nil)
	defer squirrel.Close()

	payload, err := squirrel.LocalCodeIntel(context.Background(), path)
	fatalIfError(t, err)

	return payload
}

type annotation struct {
	repoCommitPathPoint types.RepoCommitPathPoint
	symbol              string
	tags                []string
}

func collectAnnotations(repoCommitPath types.RepoCommitPath, contents string) []annotation {
	annotations := []annotation{}

	lines := strings.Split(contents, "\n")

	// Annotation at the end of the line: < "x" x def
	for i, line := range lines {
		matchess := regexp.MustCompile(`([^<]+)< "([^"]+)" ([a-zA-Z0-9_.-/]+) ([a-z,]+)`).FindAllStringSubmatch(line, -1)
		if matchess == nil {
			continue
		}

		for _, matches := range matchess {
			substr, symbol, tags := matches[2], matches[3], matches[4]

			annotations = append(annotations, annotation{
				repoCommitPathPoint: types.RepoCommitPathPoint{
					RepoCommitPath: repoCommitPath,
					Point: types.Point{
						Row:    i,
						Column: strings.Index(line, substr),
					},
				},
				symbol: symbol,
				tags:   strings.Split(tags, ","),
			})
		}
	}

	// Annotations below source lines
nextSourceLine:
	for sourceLine := 0; ; {
		for annLine := sourceLine + 1; ; annLine++ {
			if annLine >= len(lines) {
				break nextSourceLine
			}

			matches := regexp.MustCompile(`([^^]*)\^+ ([a-zA-Z0-9_.-/]+) ([a-z,]+)`).FindStringSubmatch(lines[annLine])
			if matches == nil {
				sourceLine = annLine
				continue nextSourceLine
			}

			prefix, symbol, tags := matches[1], matches[2], matches[3]

			annotations = append(annotations, annotation{
				repoCommitPathPoint: types.RepoCommitPathPoint{
					RepoCommitPath: repoCommitPath,
					Point: types.Point{
						Row:    sourceLine,
						Column: spacesToColumn(lines[sourceLine], lengthInSpaces(prefix)),
					},
				},
				symbol: symbol,
				tags:   strings.Split(tags, ","),
			})
		}
	}

	// Annotations above source lines
previousSourceLine:
	for sourceLine := len(lines) - 1; ; {
		for annLine := sourceLine - 1; ; annLine-- {
			if annLine < 0 {
				break previousSourceLine
			}

			matches := regexp.MustCompile(`([^v]*)v+ ([a-zA-Z0-9_.-/]+) ([a-z,]+)`).FindStringSubmatch(lines[annLine])
			if matches == nil {
				sourceLine = annLine
				continue previousSourceLine
			}

			prefix, symbol, tags := matches[1], matches[2], matches[3]

			annotations = append(annotations, annotation{
				repoCommitPathPoint: types.RepoCommitPathPoint{
					RepoCommitPath: repoCommitPath,
					Point: types.Point{
						Row:    sourceLine,
						Column: spacesToColumn(lines[sourceLine], lengthInSpaces(prefix)),
					},
				},
				symbol: symbol,
				tags:   strings.Split(tags, ","),
			})
		}
	}

	return annotations
}

func sortAnnotations(annotations []annotation) {
	sort.Slice(annotations, func(i, j int) bool {
		rowi := annotations[i].repoCommitPathPoint.Point.Row
		rowj := annotations[j].repoCommitPathPoint.Point.Row
		coli := annotations[i].repoCommitPathPoint.Point.Column
		colj := annotations[j].repoCommitPathPoint.Point.Column
		tagsi := annotations[i].tags
		tagsj := annotations[j].tags
		if rowi != rowj {
			return rowi < rowj
		} else if coli != colj {
			return coli < colj
		} else {
			for i := range min(len(tagsi), len(tagsj)) {
				if tagsi[i] != tagsj[i] {
					return tagsi[i] < tagsj[i]
				}
			}
			return len(tagsi) < len(tagsj)
		}
	})
}

var compareAnnotations = cmp.Comparer(func(a, b annotation) bool {
	if a.repoCommitPathPoint.RepoCommitPath != b.repoCommitPathPoint.RepoCommitPath {
		return false
	}
	if a.repoCommitPathPoint.Point.Row != b.repoCommitPathPoint.Point.Row {
		return false
	}
	if a.repoCommitPathPoint.Point.Column != b.repoCommitPathPoint.Point.Column {
		return false
	}
	if len(a.tags) != len(b.tags) {
		return false
	}
	for i := range a.tags {
		if a.tags[i] != b.tags[i] {
			return false
		}
	}
	return true
})
