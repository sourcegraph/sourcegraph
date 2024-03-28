package configmap

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/yaml"
)

func ExampleConfigMap() {
	cm, _ := NewConfigMap("test", "sourcegraph")

	jcm, _ := json.Marshal(cm)
	fmt.Println(string(jcm))

	ycm, _ := yaml.Marshal(cm)
	fmt.Println(string(ycm))

	// Output:
	// {"metadata":{"name":"test","namespace":"sourcegraph","creationTimestamp":null,"labels":{"deploy":"sourcegraph"}},"immutable":false}
	// immutable: false
	// metadata:
	//   creationTimestamp: null
	//   labels:
	//     deploy: sourcegraph
	//   name: test
	//   namespace: sourcegraph
}
