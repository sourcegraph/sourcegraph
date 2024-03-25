package container

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/yaml"
)

func ExampleContainer() {
	c, _ := NewContainer("test")

	jc, _ := json.Marshal(c)
	fmt.Println(string(jc))

	yc, _ := yaml.Marshal(c)
	fmt.Println(string(yc))

	// Output:
	// {"name":"test","resources":{"limits":{"cpu":"1","memory":"500Mi"},"requests":{"cpu":"500m","memory":"100Mi"}},"terminationMessagePolicy":"FallbackToLogsOnError","imagePullPolicy":"IfNotPresent","securityContext":{"runAsUser":100,"runAsGroup":101,"readOnlyRootFilesystem":true,"allowPrivilegeEscalation":false}}
	// imagePullPolicy: IfNotPresent
	// name: test
	// resources:
	//   limits:
	//     cpu: "1"
	//     memory: 500Mi
	//   requests:
	//     cpu: 500m
	//     memory: 100Mi
	// securityContext:
	//   allowPrivilegeEscalation: false
	//   readOnlyRootFilesystem: true
	//   runAsGroup: 101
	//   runAsUser: 100
	// terminationMessagePolicy: FallbackToLogsOnError
}

func ExampleWithDefaultLivenessProbe() {
	c, _ := NewContainer("test", WithDefaultLivenessProbe())

	jc, _ := json.Marshal(c)
	fmt.Println(string(jc))

	yc, _ := yaml.Marshal(c)
	fmt.Println(string(yc))

	// Output:
	// {"name":"test","resources":{"limits":{"cpu":"1","memory":"500Mi"},"requests":{"cpu":"500m","memory":"100Mi"}},"livenessProbe":{"httpGet":{"path":"/","port":"test","scheme":"HTTP"},"initialDelaySeconds":60,"timeoutSeconds":5},"terminationMessagePolicy":"FallbackToLogsOnError","imagePullPolicy":"IfNotPresent","securityContext":{"runAsUser":100,"runAsGroup":101,"readOnlyRootFilesystem":true,"allowPrivilegeEscalation":false}}
	// imagePullPolicy: IfNotPresent
	// livenessProbe:
	//   httpGet:
	//     path: /
	//     port: test
	//     scheme: HTTP
	//   initialDelaySeconds: 60
	//   timeoutSeconds: 5
	// name: test
	// resources:
	//   limits:
	//     cpu: "1"
	//     memory: 500Mi
	//   requests:
	//     cpu: 500m
	//     memory: 100Mi
	// securityContext:
	//   allowPrivilegeEscalation: false
	//   readOnlyRootFilesystem: true
	//   runAsGroup: 101
	//   runAsUser: 100
	// terminationMessagePolicy: FallbackToLogsOnError
}

func ExampleWithDefaultReadinessProbe() {
	c, _ := NewContainer("test", WithDefaultReadinessProbe())

	jc, _ := json.Marshal(c)
	fmt.Println(string(jc))

	yc, _ := yaml.Marshal(c)
	fmt.Println(string(yc))

	// Output:
	// {"name":"test","resources":{"limits":{"cpu":"1","memory":"500Mi"},"requests":{"cpu":"500m","memory":"100Mi"}},"readinessProbe":{"httpGet":{"path":"/","port":"test","scheme":"HTTP"},"timeoutSeconds":5,"periodSeconds":5},"terminationMessagePolicy":"FallbackToLogsOnError","imagePullPolicy":"IfNotPresent","securityContext":{"runAsUser":100,"runAsGroup":101,"readOnlyRootFilesystem":true,"allowPrivilegeEscalation":false}}
	// imagePullPolicy: IfNotPresent
	// name: test
	// readinessProbe:
	//   httpGet:
	//     path: /
	//     port: test
	//     scheme: HTTP
	//   periodSeconds: 5
	//   timeoutSeconds: 5
	// resources:
	//   limits:
	//     cpu: "1"
	//     memory: 500Mi
	//   requests:
	//     cpu: 500m
	//     memory: 100Mi
	// securityContext:
	//   allowPrivilegeEscalation: false
	//   readOnlyRootFilesystem: true
	//   runAsGroup: 101
	//   runAsUser: 100
	// terminationMessagePolicy: FallbackToLogsOnError
}
