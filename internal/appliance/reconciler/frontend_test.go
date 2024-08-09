package reconciler

import (
	"fmt"
	"maps"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sourcegraph/sourcegraph/internal/appliance/k8senvtest"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/ingress"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func (suite *ApplianceTestSuite) TestDeployFrontend() {
	for _, tc := range []struct {
		name string
	}{
		{name: "frontend/default"},
		{name: "frontend/with-blobstore"},
		{name: "frontend/with-ingress"},
		{name: "frontend/with-ingress-optional-fields"},
		{name: "frontend/with-overrides"},
	} {
		suite.Run(tc.name, func() {
			namespace := suite.createConfigMapAndAwaitReconciliation(tc.name)
			suite.makeGoldenAssertions(namespace, tc.name)
		})
	}
}

func (suite *ApplianceTestSuite) TestAdoptsHelmProvisionedFrontendResources() {
	namespace, err := k8senvtest.NewRandomNamespace("test-appliance")
	suite.Require().NoError(err)
	_, err = suite.k8sClient.CoreV1().Namespaces().Create(suite.ctx, namespace, metav1.CreateOptions{})
	suite.Require().NoError(err)
	testService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sourcegraph-frontend",
			Namespace: namespace.Name,
			Labels: map[string]string{
				"app": "sourcegraph-frontend",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports:    []corev1.ServicePort{{Name: "http", Port: 30080, TargetPort: intstr.FromString("http")}},
			Selector: map[string]string{"app": "sourcegraph-appliance"},
		},
	}
	_, err = suite.k8sClient.CoreV1().Services(namespace.Name).Create(suite.ctx, &testService, metav1.CreateOptions{})
	suite.Require().NoError(err)

	testIngress := ingress.NewIngress("sourcegraph-frontend", namespace.Name)
	testIngress.Spec.Rules = []netv1.IngressRule{{
		Host: "an-existing-hostname.com",
		IngressRuleValue: netv1.IngressRuleValue{
			HTTP: &netv1.HTTPIngressRuleValue{
				Paths: []netv1.HTTPIngressPath{{
					Path:     "/",
					PathType: pointers.Ptr(netv1.PathTypePrefix),
					Backend: netv1.IngressBackend{
						Service: &netv1.IngressServiceBackend{
							Name: "sourcegraph-frontend",
							Port: netv1.ServiceBackendPort{
								Number: 30081,
							},
						},
					},
				}},
			},
		},
	}}
	ingressClassName := "nginx"
	testIngress.Spec.IngressClassName = &ingressClassName
	_, err = suite.k8sClient.NetworkingV1().Ingresses(namespace.Name).Create(suite.ctx, &testIngress, metav1.CreateOptions{})
	suite.Require().NoError(err)

	cfgMap := suite.newConfigMap(namespace.GetName(), "frontend/with-ingress")
	suite.awaitReconciliation(namespace.GetName(), func() {
		_, err := suite.k8sClient.CoreV1().ConfigMaps(namespace.GetName()).Create(suite.ctx, cfgMap, metav1.CreateOptions{})
		suite.Require().NoError(err)
	})
	suite.makeGoldenAssertions(namespace.GetName(), "frontend/adopt-service")
}

func (suite *ApplianceTestSuite) TestFrontendDeploymentRollsWhenPGSecretsChange() {
	for _, tc := range []struct {
		secret string
	}{
		{secret: pgsqlSecretName},
		{secret: codeInsightsDBSecretName},
		{secret: codeIntelDBSecretName},
	} {
		suite.Run(tc.secret, func() {
			// Create the frontend before the PGSQL secret exists. In general, this
			// might happen, depending on the order of the reconcile loop. If we
			// introduce concurrency to this, we'll have little control over what
			// happens first.
			namespace := suite.createConfigMapAndAwaitReconciliation("frontend/default")

			// Create the PGSQL secret.
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.secret,
				},
				StringData: map[string]string{
					"host":     "example.com",
					"port":     "5432",
					"user":     "alice",
					"password": "letmein",
					"database": "sg",
				},
			}
			_, err := suite.k8sClient.CoreV1().Secrets(namespace).Create(suite.ctx, secret, metav1.CreateOptions{})
			suite.Require().NoError(err)

			// We have to make a config change to trigger the reconcile loop
			suite.awaitReconciliation(namespace, func() {
				cfgMap := suite.newConfigMap(namespace, "frontend/default")
				cfgMap.GetAnnotations()["force-reconcile"] = "1"
				_, err := suite.k8sClient.CoreV1().ConfigMaps(namespace).Update(suite.ctx, cfgMap, metav1.UpdateOptions{})
				suite.Require().NoError(err)
			})

			suite.makeGoldenAssertions(namespace, fmt.Sprintf("frontend/after-create-%s-secret", tc.secret))
		})
	}
}

func (suite *ApplianceTestSuite) TestFrontendDeploymentRollsWhenRedisSecretsChange() {
	for _, tc := range []struct {
		secret string
	}{
		{secret: redisCacheSecretName},
		{secret: redisStoreSecretName},
	} {
		suite.Run(tc.secret, func() {
			// Create the frontend before the PGSQL secret exists. In general, this
			// might happen, depending on the order of the reconcile loop. If we
			// introduce concurrency to this, we'll have little control over what
			// happens first.
			namespace := suite.createConfigMapAndAwaitReconciliation("frontend/default")

			// Create the PGSQL secret.
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.secret,
				},
				StringData: map[string]string{
					"endpoint": "example.com",
				},
			}
			_, err := suite.k8sClient.CoreV1().Secrets(namespace).Create(suite.ctx, secret, metav1.CreateOptions{})
			suite.Require().NoError(err)

			// We have to make a config change to trigger the reconcile loop
			suite.awaitReconciliation(namespace, func() {
				cfgMap := suite.newConfigMap(namespace, "frontend/default")
				cfgMap.GetAnnotations()["force-reconcile"] = "1"
				_, err := suite.k8sClient.CoreV1().ConfigMaps(namespace).Update(suite.ctx, cfgMap, metav1.UpdateOptions{})
				suite.Require().NoError(err)
			})

			suite.makeGoldenAssertions(namespace, fmt.Sprintf("frontend/after-create-%s-secret", tc.secret))
		})
	}
}

type MockObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Data              map[string]string `json:"data,omitempty"`
}

func (m *MockObject) DeepCopyObject() runtime.Object {
	if m == nil {
		return nil
	}
	return &MockObject{
		TypeMeta:   m.TypeMeta,
		ObjectMeta: *m.ObjectMeta.DeepCopy(),
		Data:       maps.Clone(m.Data),
	}
}

func (suite *ApplianceTestSuite) TestMergeK8sObjects() {
	tests := []struct {
		name        string
		existingObj client.Object
		newObject   client.Object
		expected    client.Object
		expectError bool
	}{
		{
			name: "Successful merge",
			existingObj: &MockObject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Data: map[string]string{
					"key1": "value1",
				},
			},
			newObject: &MockObject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Data: map[string]string{
					"key2": "value2",
				},
			},
			expected: &MockObject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expectError: false,
		},
		{
			name: "Merge with overlapping keys",
			existingObj: &MockObject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Data: map[string]string{
					"key1": "value1",
					"key2": "old-value2",
				},
			},
			newObject: &MockObject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Data: map[string]string{
					"key2": "new-value2",
					"key3": "value3",
				},
			},
			expected: &MockObject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Data: map[string]string{
					"key1": "value1",
					"key2": "new-value2",
					"key3": "value3",
				},
			},
			expectError: false,
		},
		{
			name: "Merge with empty new object",
			existingObj: &MockObject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Data: map[string]string{
					"key1": "value1",
				},
			},
			newObject: &MockObject{},
			expected: &MockObject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Data: map[string]string{
					"key1": "value1",
				},
			},
			expectError: false,
		},
		{
			name: "merges annotations",
			existingObj: &MockObject{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"present_and_unchanged": "old1",
						"present_and_changed":   "old2",
					},
				},
			},
			newObject: &MockObject{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"present_and_changed": "new2",
						"new":                 "new3",
					},
				},
			},
			expected: &MockObject{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"present_and_unchanged": "old1",
						"present_and_changed":   "new2",
						"new":                   "new3",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result, err := MergeK8sObjects(tt.existingObj, tt.newObject)
			fmt.Print(result)
			if tt.expectError {
				assert.Error(suite.T(), err)
				assert.Nil(suite.T(), result)
			} else {
				assert.NoError(suite.T(), err)
				assert.Equal(suite.T(), tt.expected, result)
			}
		})
	}
}
