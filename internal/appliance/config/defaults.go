package config

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// NewDefaultConfig
//
// Warning: never extract `ptr.To(thing)` into a package-level variable! If you
// do this, reconciling a config that overrides a default value for that
// pointer, will affect the subsequent _default_ for all future resources
// reconciled.
func NewDefaultConfig() Sourcegraph {
	return Sourcegraph{
		Spec: SourcegraphSpec{
			// Global config
			ImageRepository: "index.docker.io/sourcegraph",

			// Service-specific config
			Blobstore: BlobstoreSpec{
				StandardConfig: StandardConfig{
					PersistentVolumeConfig: PersistentVolumeConfig{
						StorageSize: "100Gi",
					},
				},
			},
			RepoUpdater: RepoUpdaterSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(6060),
				},
			},
			Symbols: SymbolsSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(6060),
					PersistentVolumeConfig: PersistentVolumeConfig{
						StorageSize: "12Gi",
					},
				},
				Replicas: 1,
			},
			GitServer: GitServerSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(6060),
					PersistentVolumeConfig: PersistentVolumeConfig{
						StorageSize: "200Gi",
					},
				},
				Replicas: 1,
			},
			PGSQL: PGSQLSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(9187),
					PersistentVolumeConfig: PersistentVolumeConfig{
						StorageSize: "200Gi",
					},
				},
				DatabaseConnection: &DatabaseConnectionSpec{
					Host:     "pgsql",
					Port:     "5432",
					User:     "sg",
					Password: "password",
					Database: "sg",
				},
			},
			RedisCache: RedisSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(9121),
					PersistentVolumeConfig: PersistentVolumeConfig{
						StorageSize: "100Gi",
					},
				},
			},
			RedisStore: RedisSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(9121),
					PersistentVolumeConfig: PersistentVolumeConfig{
						StorageSize: "100Gi",
					},
				},
			},
			SyntectServer: SyntectServerSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(6060),
				},
				Replicas: 1,
			},
			PreciseCodeIntel: PreciseCodeIntelSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(6060),
				},
				NumWorkers: 4,
				Replicas:   2,
			},
			CodeInsights: CodeDBSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(9187),
					PersistentVolumeConfig: PersistentVolumeConfig{
						StorageSize: "200Gi",
					},
				},
				DatabaseConnection: &DatabaseConnectionSpec{
					Host:     "codeinsights-db",
					Port:     "5432",
					User:     "postgres",
					Password: "password",
					Database: "postgres",
				},
			},
			CodeIntel: CodeDBSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(9187),
					PersistentVolumeConfig: PersistentVolumeConfig{
						StorageSize: "200Gi",
					},
				},
				DatabaseConnection: &DatabaseConnectionSpec{
					Host:     "codeintel-db",
					Port:     "5432",
					User:     "sg",
					Password: "password",
					Database: "sg",
				},
			},
			Prometheus: PrometheusSpec{
				StandardConfig: StandardConfig{
					PersistentVolumeConfig: PersistentVolumeConfig{
						StorageSize: "200Gi",
					},
				},
			},
			Cadvisor: CadvisorSpec{
				StandardConfig: StandardConfig{
					// cadvisor is opt-in due to the privilege requirements
					Disabled:       true,
					PrometheusPort: pointers.Ptr(48080),
				},
			},
			Worker: WorkerSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(6060),
				},
				Replicas: 1,
			},
			Frontend: FrontendSpec{
				StandardConfig: StandardConfig{
					PrometheusPort: pointers.Ptr(6060),
				},
				Replicas: 2,
				Migrator: true,
			},
			Searcher: SearcherSpec{
				StandardConfig: StandardConfig{
					PersistentVolumeConfig: PersistentVolumeConfig{
						StorageSize: "26Gi",
					},
					PrometheusPort: pointers.Ptr(6060),
				},
				Replicas: 1,
			},
			IndexedSearch: IndexedSearchSpec{
				StandardConfig: StandardConfig{
					PersistentVolumeConfig: PersistentVolumeConfig{
						StorageSize: "200Gi",
					},
					PrometheusPort: pointers.Ptr(6070),
				},
				Replicas: 1,
			},

			// Grafana and Jaeger are opt-in
			Grafana: GrafanaSpec{
				StandardConfig: StandardConfig{
					Disabled: true,
				},
			},
			Jaeger: JaegerSpec{
				StandardConfig: StandardConfig{
					Disabled: true,
				},
				Replicas: 1,
			},
		},
	}
}

func GetDefaultImage(sg *Sourcegraph, component string) string {
	return fmt.Sprintf("%s/%s:%s", sg.Spec.ImageRepository, component, sg.Spec.RequestedVersion)
}
