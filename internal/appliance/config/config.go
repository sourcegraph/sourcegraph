package config

import corev1 "k8s.io/api/core/v1"

type StandardComponent interface {
	Disableable
	GetContainerConfig() map[string]ContainerConfig
	GetPersistentVolumeConfig() PersistentVolumeConfig
	GetPodTemplateConfig() PodTemplateConfig
	GetServiceAccountAnnotations() map[string]string
	GetPrometheusPort() *int
}

type Disableable interface {
	IsDisabled() bool
}

type StandardConfig struct {
	Disabled                  *bool                      `json:"disabled,omitempty"`
	ContainerConfig           map[string]ContainerConfig `json:"containerConfig,omitempty"`
	PersistentVolumeConfig    PersistentVolumeConfig     `json:"persistentVolumeConfig,omitempty"`
	PodTemplateConfig         PodTemplateConfig          `json:"podTemplateConfig,omitempty"`
	PrometheusPort            *int                       `json:"prometheusPort,omitempty"`
	ServiceAccountAnnotations map[string]string          `json:"serviceAccountAnnotations,omitempty"`
}

type ContainerConfig struct {
	Image string `json:"image,omitempty"`

	// Set BestEffortQOS=true to configure a container without resource limits
	// or requests. This can be useful for local development.
	// We need this flag to disambiguate between Resources being null because
	// the admin is not overriding defaults, or because they do not want to
	// configure resources.
	// https://kubernetes.io/docs/tasks/configure-pod-container/quality-service-pod/
	BestEffortQOS bool                         `json:"bestEffortQOS,omitempty"`
	Resources     *corev1.ResourceRequirements `json:"resources,omitempty"`

	EnvVars map[string]string `json:"envVars,omitempty"`
}

type PersistentVolumeConfig struct {
	StorageSize      string  `json:"storageSize,omitempty"`
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// PodTemplateConfig is a config that applies to all Pod templates produced by a Service. If this needs
// to differ between pod templates, split another service definition.
type PodTemplateConfig struct {
	Affinity         *corev1.Affinity              `json:"affinity,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeSelector     map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations      []corev1.Toleration           `json:"tolerations,omitempty"`
}

func (c StandardConfig) IsDisabled() bool                               { return c.Disabled != nil && *c.Disabled }
func (c StandardConfig) GetContainerConfig() map[string]ContainerConfig { return c.ContainerConfig }
func (c StandardConfig) GetPersistentVolumeConfig() PersistentVolumeConfig {
	return c.PersistentVolumeConfig
}
func (c StandardConfig) GetPodTemplateConfig() PodTemplateConfig { return c.PodTemplateConfig }
func (c StandardConfig) GetPrometheusPort() *int                 { return c.PrometheusPort }
func (c StandardConfig) GetServiceAccountAnnotations() map[string]string {
	return c.ServiceAccountAnnotations
}
