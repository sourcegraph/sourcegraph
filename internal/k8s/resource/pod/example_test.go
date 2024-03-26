package pod

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/yaml"
)

func ExamplePodTemplate() {
	pt, _ := NewPodTemplate("test", "sourcegraph")

	jpt, _ := json.Marshal(pt)
	fmt.Println(string(jpt))

	ypt, _ := yaml.Marshal(pt)
	fmt.Println(string(ypt))

	// Output:
	// {"metadata":{"creationTimestamp":null},"template":{"metadata":{"name":"test","namespace":"sourcegraph","creationTimestamp":null,"labels":{"app":"test","deploy":"sourcegraph"},"annotations":{"kubectl.kubernetes.io/default-container":"test"}},"spec":{"containers":null,"securityContext":{"runAsUser":100,"runAsGroup":101,"fsGroup":101,"fsGroupChangePolicy":"OnRootMismatch"}}}}
	// metadata:
	//   creationTimestamp: null
	// template:
	//   metadata:
	//     annotations:
	//       kubectl.kubernetes.io/default-container: test
	//     creationTimestamp: null
	//     labels:
	//       app: test
	//       deploy: sourcegraph
	//     name: test
	//     namespace: sourcegraph
	//   spec:
	//     containers: null
	//     securityContext:
	//       fsGroup: 101
	//       fsGroupChangePolicy: OnRootMismatch
	//       runAsGroup: 101
	//       runAsUser: 100
}
