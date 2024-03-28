package storage

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/yaml"
)

func ExampleStorageClass() {
	sc, _ := NewStorageClass("test", "sourcegraph")

	jsc, _ := json.Marshal(sc)
	fmt.Println(string(jsc))

	ysc, _ := yaml.Marshal(sc)
	fmt.Println(string(ysc))

	// Output:
	// {"metadata":{"name":"test","namespace":"sourcegraph","creationTimestamp":null,"labels":{"deploy":"sourcegraph-storage"}},"provisioner":"","reclaimPolicy":"Retain","allowVolumeExpansion":true,"volumeBindingMode":"WaitForFirstConsumer"}
	// allowVolumeExpansion: true
	// metadata:
	//   creationTimestamp: null
	//   labels:
	//     deploy: sourcegraph-storage
	//   name: test
	//   namespace: sourcegraph
	// provisioner: ""
	// reclaimPolicy: Retain
	// volumeBindingMode: WaitForFirstConsumer
}
