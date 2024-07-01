package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ManagementStateType string

const (
	// ManagementStateManaged denotes that Sourcegraph should be reconciled
	// by the operator.
	ManagementStateManaged ManagementStateType = "Managed"

	// ManagementStateUnmanaged denotes that Sourcegraph should not be reconciled
	// by the operator.
	ManagementStateUnmanaged ManagementStateType = "Unmanaged"
)

type DatabaseConnectionSpec struct {
	Host     string `json:"host,omitempty"`
	Port     string `json:"port,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Database string `json:"database,omitempty"`
}

// BlobstoreSpec defines the desired state of Blobstore.
type BlobstoreSpec struct {
	StandardConfig
}

type CadvisorSpec struct {
	StandardConfig
}

type CodeDBSpec struct {
	StandardConfig

	// Database allows for custom database connection details.
	DatabaseConnection *DatabaseConnectionSpec `json:"database,omitempty"`
}

type IngressSpec struct {
	Annotations      map[string]string `json:"annotations,omitempty"`
	Host             string            `json:"host,omitempty"`
	IngressClassName *string           `json:"ingressClassName,omitempty"`
	TLSSecret        string            `json:"tlsSecret,omitempty"`
}

type FrontendSpec struct {
	StandardConfig

	Migrator bool `json:"migrator,omitempty"`

	// Replicas defines the number of Frontend pod replicas.
	// Default: 2
	Replicas int32 `json:"replicas,omitempty"`

	// Ingress allows for changes to the custom Sourcegraph ingress.
	Ingress *IngressSpec `json:"ingress,omitempty"`
}

// GitServerSpec defines the desired state of GitServer.
type GitServerSpec struct {
	StandardConfig

	// Replicas defines the number of Symbols pod replicas.
	// Default: 1
	Replicas int32 `json:"replicas,omitempty"`

	// SSHSecret is the name of existing secret that contains SSH credentials to clone repositories.
	// This secret generally contains keys such as `id_rsa` (private key) and `known_hosts`.
	SSHSecret string `json:"sshSecret,omitempty"`
}

type GrafanaSpec struct {
	StandardConfig

	// Replicas defines the number of Grafana pod replicas.
	// Default: 1
	Replicas int32 `json:"replicas,omitempty"`

	ExistingConfigMap string `json:"existingConfigMap,omitempty"`
}

type IndexedSearchSpec struct {
	StandardConfig

	Replicas int32 `json:"replicas,omitempty"`
}

type OtelCollectorSpec struct {
	StandardConfig
}

type JaegerSpec struct {
	StandardConfig
}

// PGSQLSpec defines the desired state of the Postgres server.
type PGSQLSpec struct {
	StandardConfig

	// DatabaseConnection allows for custom database connection details.
	DatabaseConnection *DatabaseConnectionSpec `json:"database,omitempty"`
}

type PreciseCodeIntelSpec struct {
	StandardConfig

	NumWorkers int `json:"numWorkers"`

	// Replicas defines the number of Precise Code Intel Worker pod replicas.
	// Default: 2
	Replicas int32 `json:"replicas,omitempty"`
}

type PrometheusSpec struct {
	StandardConfig

	ExistingConfigMap string `json:"existingConfigMap,omitempty"`
	Privileged        bool   `json:"privileged,omitempty"`
}

// RedisSpec defines the desired state of a Redis-based service.
type RedisSpec struct {
	StandardConfig
}

// RepoUpdaterSpec defines the desired state of the Repo Updater service.
type RepoUpdaterSpec struct {
	StandardConfig
}

type SearcherSpec struct {
	StandardConfig

	// Replicas defines the number of Searcher pod replicas.
	// Default: 1
	Replicas int32 `json:"replicas,omitempty"`
}

// SymbolsSpec defines the desired state of the Symbols service.
type SymbolsSpec struct {
	StandardConfig

	// Replicas defines the number of Symbols pod replicas.
	// Default: 1
	Replicas int32 `json:"replicas,omitempty"`
}

// SyntectServerSpec defines the desired state of the Syntect server service.
type SyntectServerSpec struct {
	StandardConfig

	// Replicas defines the number of Syntect Server pod replicas.
	// Default: 1
	Replicas int32 `json:"replicas,omitempty"`
}

type WorkerSpec struct {
	StandardConfig

	// Replicas defines the number of Worker pod replicas.
	// Default: 1
	Replicas int32 `json:"replicas,omitempty"`
}

type StorageClassSpec struct {
	// Name is the name of the storageClass.
	// Default: sourcegraph
	Name *string `json:"name,omitempty"`

	// Create will enable/disable the creation of storageClass.
	// Enable if you have your own existing storage class.
	// Default: false
	Create bool `json:"create,omitempty"`

	// Provisioner is the storageClass provisioner.
	// Default: kubernetes.io/no-provisioner
	Provisioner string `json:"provisioner,omitempty"`

	// Type is the `type` key in storageClass `parameters`.
	Type string `json:"type,omitempty"`

	// Parameters defines any extra parameters of StorageClass.
	Parameters map[string]string `json:"parameters,omitempty"`
}

// SourcegraphSpec defines the desired state of Sourcegraph
type SourcegraphSpec struct {
	// RequestedVersion is the user-requested version of Sourcegraph to deploy.
	RequestedVersion string `json:"requestedVersion,omitempty"`

	// ImageRepository overrides the default image repository.
	ImageRepository string `json:"imageRepository,omitempty"`

	// ManagementState defines if Sourcegraph should be managed by the operator or not.
	// Default is managed.
	ManagementState ManagementStateType `json:"managementState,omitempty"`

	// MaintenancePassword will set the password for the administrator maintenance UI.
	// If no password is set, a random password will be generated and storage in a secret.
	MaintenancePassword string `json:"maintenancePassword,omitempty"`

	// Blobstore defines the desired state of the Blobstore service.
	Blobstore BlobstoreSpec `json:"blobstore,omitempty"`

	Cadvisor CadvisorSpec `json:"cadvisor,omitempty"`

	// CodeInsights defines the desired state of the Code Insights service.
	CodeInsights CodeDBSpec `json:"codeInsights,omitempty"`

	// CodeIntel defines the desired state of the Code Intel service.
	CodeIntel CodeDBSpec `json:"codeIntel,omitempty"`

	// Frontend defines the desired state of the Sourcegraph Frontend.
	Frontend FrontendSpec `json:"frontend,omitempty"`

	// GitServer defines the desired state of the GitServer service.
	GitServer GitServerSpec `json:"gitServer,omitempty"`

	Grafana GrafanaSpec `json:"grafana,omitempty"`

	// IndexedSearch defines the desired state of the Indexed Search service.
	IndexedSearch IndexedSearchSpec `json:"indexedSearch,omitempty"`

	Jaeger JaegerSpec `json:"jaeger,omitempty"`

	OtelCollector OtelCollectorSpec `json:"openTelemetry,omitempty"`

	// PGSQL defines the desired state of the PostgreSQL database.
	PGSQL PGSQLSpec `json:"pgsql,omitempty"`

	// PreciseCodeIntel defines the desired state of the Precise Code Intel service.
	PreciseCodeIntel PreciseCodeIntelSpec `json:"preciseCodeIntel,omitempty"`

	Prometheus PrometheusSpec `json:"prometheus,omitempty"`

	// RedisCache defines the desired state of the Redis cache service.
	RedisCache RedisSpec `json:"redisCache,omitempty"`

	// RedisStore defines the desired state of the Redis store service.
	RedisStore RedisSpec `json:"redisStore,omitempty"`

	// RepoUpdater defines the desired state of the Repo updater service.
	RepoUpdater RepoUpdaterSpec `json:"repoUpdater,omitempty"`

	// Searcher defines the desired state of the Searcher service.
	Searcher SearcherSpec `json:"searcher,omitempty"`

	// Symbols defines the desired state of the Symbols service.
	Symbols SymbolsSpec `json:"symbols,omitempty"`

	// SyntectServer defines the desired state of the Syntect Server service.
	SyntectServer SyntectServerSpec `json:"syntectServer,omitempty"`

	// Worker defines the desired state of the Worker service.
	Worker WorkerSpec `json:"worker,omitempty"`

	// StorageClass defines the desired state a custom storage class.
	// If none is specified, default cluster storage class will be used.
	StorageClass StorageClassSpec `json:"storageClass,omitempty"`
}

// SetupStatus defines the observes status of the setup process.
type SetupStatus struct {
	Progress int32
}

// SourcegraphStatus defines the observed state of Sourcegraph
type SourcegraphStatus struct {
	// CurrentVersion is the version of Sourcegraph currently running.
	CurrentVersion string `json:"currentVersion"`

	// Setup tracks the progress of the setup process.
	Setup SetupStatus `json:"setup,omitempty"`

	// Represents the latest available observations of Sourcegraph's current state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Sourcegraph is the Schema for the Sourcegraph API
type Sourcegraph struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SourcegraphSpec   `json:"spec,omitempty"`
	Status SourcegraphStatus `json:"status,omitempty"`
}

// SourcegraphList contains a list of Sourcegraph
type SourcegraphList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Sourcegraph `json:"items"`
}
