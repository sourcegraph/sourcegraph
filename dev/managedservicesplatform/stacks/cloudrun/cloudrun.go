package cloudrun

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/exp/maps"

	"github.com/hashicorp/terraform-cdk-go/cdktf"

	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/projectiamcustomrole"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/projectiammember"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/pubsubsubscription"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/pubsubtopic"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/serviceaccountiammember"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/storagebucket"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/storagebucketiammember"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/storagebucketobject"
	postgresql "github.com/sourcegraph/managed-services-platform-cdktf/gen/postgresql/provider"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/postgresql/role"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/random/password"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/sentry/datasentryorganization"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/sentry/datasentryteam"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/sentry/key"
	sentryproject "github.com/sourcegraph/managed-services-platform-cdktf/gen/sentry/project"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/googlesecretsmanager"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/bigquery"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/cloudsql"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/deliverypipeline"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/gsmsecret"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/postgresqllogicalreplication"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/postgresqlroles"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/privatenetwork"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/random"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/redis"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/tfvar"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resourceid"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack/options/cloudflareprovider"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack/options/dynamicvariables"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack/options/googleprovider"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack/options/randomprovider"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack/options/sentryprovider"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/spec"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/cloudrun/internal/builder"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/cloudrun/internal/builder/job"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/cloudrun/internal/builder/service"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/iam"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type CrossStackOutput struct {
	DiagnosticsSecret  *random.Output
	RedisInstanceID    *string
	CloudSQLInstanceID *string
	SentryProject      sentryproject.Project
}

type Variables struct {
	ProjectID string

	IAM iam.CrossStackOutput

	Service spec.ServiceSpec
	// Repository is the source code repository for the image to deploy
	Repository  string
	Image       string
	Environment spec.EnvironmentSpec

	// RolloutPipeline is only non-nil if this environment is part of rollout
	// pipeline. The final environment (IsFinalStage) is where the Cloud Deploy
	// pipeline lives.
	RolloutPipeline *spec.RolloutPipelineConfiguration

	StableGenerate bool

	PreventDestroys bool
}

const StackName = "cloudrun"

const (
	OutputCloudSQLConnectionName = "cloudsql_connection_name"

	// ScaffoldSourceFile is the file to place in the cloudrun Terraform stack
	// directory for upload. We expect this to be generated into the TF dir -
	// it's weird but unfortunately placing the file into bucket object 'content'
	// directly in Terraform seems to mangle it terribly.
	ScaffoldSourceFile = "skaffoldsource.tar.gz"
)

const tfVarKeyResolvedImageTag = "resolved_image_tag"

const SentryOrganization = "sourcegraph"

// NewStack instantiates the MSP cloudrun stack, which is currently a pretty
// monolithic stack that encompasses all the core components of an MSP service,
// including networking and dependencies like Redis.
func NewStack(stacks *stack.Set, vars Variables) (crossStackOutput *CrossStackOutput, _ error) {
	stack, locals, err := stacks.New(StackName,
		googleprovider.With(vars.ProjectID),
		cloudflareprovider.With(gsmsecret.DataConfig{
			Secret:    googlesecretsmanager.SecretCloudflareAPIToken,
			ProjectID: googlesecretsmanager.SharedSecretsProjectID,
		}),
		randomprovider.With(),
		dynamicvariables.With(vars.StableGenerate, func() (stack.TFVars, error) {
			if d := vars.Environment.Deploy; d.Type == spec.EnvironmentDeployTypeSubscription {
				resolvedImageTag, err := d.Subscription.ResolveTag(vars.Image)
				return stack.TFVars{tfVarKeyResolvedImageTag: resolvedImageTag}, err
			}
			return nil, nil
		}),
		sentryprovider.With(gsmsecret.DataConfig{
			Secret:    googlesecretsmanager.SecretSentryAuthToken,
			ProjectID: googlesecretsmanager.SharedSecretsProjectID,
		}))
	if err != nil {
		return nil, err
	}

	// Resource locationSpec configuration
	locationSpec := vars.Environment.GetLocationSpec()

	diagnosticsSecret := random.New(stack, resourceid.New("diagnostics-secret"), random.Config{
		ByteLength: 8,
	})

	id := resourceid.New("cloudrun")

	// Set up configuration for the Cloud Run resources
	var cloudRunBuilder builder.Builder
	switch pointers.Deref(vars.Service.Kind, spec.ServiceKindService) {
	case spec.ServiceKindService:
		cloudRunBuilder = service.NewBuilder()
	case spec.ServiceKindJob:
		cloudRunBuilder = job.NewBuilder()
	}

	// Required to enable tracing etc.
	cloudRunBuilder.AddEnv("GOOGLE_CLOUD_PROJECT", vars.ProjectID)

	// Set up secret that service should accept for diagnostics
	// endpoints.
	cloudRunBuilder.AddEnv("DIAGNOSTICS_SECRET", diagnosticsSecret.HexValue)

	// Add the domain as an environment variable.
	dnsName := pointers.DerefZero(vars.Environment.EnvironmentServiceSpec).Domain.GetDNSName()
	if dnsName != "" {
		cloudRunBuilder.AddEnv("EXTERNAL_DNS_NAME", dnsName)
	}

	// Add environment ID env var
	cloudRunBuilder.AddEnv("ENVIRONMENT_ID", vars.Environment.ID)

	// Add user-configured env vars
	if err := addContainerEnvVars(cloudRunBuilder, vars.Environment.Env, vars.Environment.SecretEnv, envVariablesData{
		ProjectID:      vars.ProjectID,
		ServiceDnsName: dnsName,
	}); err != nil {
		return nil, errors.Wrap(err, "add user env vars")
	}

	// Add user-configured secret volumes
	addContainerSecretVolumes(cloudRunBuilder, vars.Environment.SecretVolumes)

	// Determine where to source the image tag from, based on the deploy type.
	var imageTag string
	switch d := vars.Environment.Deploy; d.Type {
	case spec.EnvironmentDeployTypeManual:
		imageTag = d.Manual.GetTag()

	case spec.EnvironmentDeployTypeRollout:
		imageTag = vars.RolloutPipeline.OriginalSpec.GetInitialImageTag()

	case spec.EnvironmentDeployTypeSubscription:
		imageTag = *tfvar.New(stack, id, tfvar.Config{
			VariableKey: tfVarKeyResolvedImageTag,
			Description: "Image tag resolved from subscription to deploy",
		}).StringValue

	default:
		return nil, errors.Newf("unsupported deploy type %q", d.Type)
	}

	// privateNetworkEnabled indicates if privateNetwork has been instantiated
	// before.
	var privateNetworkEnabled bool
	// privateNetwork is only instantiated if used, and is only instantiated
	// once. If called, it always returns a non-nil value.
	privateNetwork := sync.OnceValue(func() *privatenetwork.Output {
		privateNetworkEnabled = true
		return privatenetwork.New(stack, resourceid.New("privatenetwork"), privatenetwork.Config{
			ProjectID: vars.ProjectID,
			ServiceID: vars.Service.ID,
			Region:    locationSpec.GCPRegion,
		})
	})

	// Add MSP env var indicating that the service is running in a Managed
	// Services Platform environment.
	cloudRunBuilder.AddEnv("MSP", "true")

	// For SSL_CERT_DIR, configure right before final build
	sslCertDirs := []string{"/etc/ssl/certs"}

	// redisInstance is only created and non-nil if Redis is configured for the
	// environment.
	// If Redis is configured, populate cross-stack output with Redis ID.
	var redisInstanceID *string
	if vars.Environment.Resources != nil && vars.Environment.Resources.Redis != nil {
		redisInstance, err := redis.New(stack,
			resourceid.New("redis"),
			redis.Config{
				ProjectID: vars.ProjectID,
				Region:    locationSpec.GCPRegion,
				Spec:      *vars.Environment.Resources.Redis,
				Network:   privateNetwork().Network,
			})
		if err != nil {
			return nil, errors.Wrap(err, "failed to render Redis instance")
		}

		redisInstanceID = redisInstance.ID

		// Configure endpoint string.
		cloudRunBuilder.AddEnv("REDIS_ENDPOINT", redisInstance.Endpoint)

		// Mount the custom cert and add it to SSL_CERT_DIR
		caCertVolumeName := "redis-ca-cert"
		cloudRunBuilder.AddSecretVolume(
			caCertVolumeName,
			"redis-ca-cert.pem",
			builder.SecretRef{
				Name:    redisInstance.Certificate.ID,
				Version: redisInstance.Certificate.Version,
			},
			292, // 0444 read-only
		)
		cloudRunBuilder.AddVolumeMount(caCertVolumeName, "/etc/ssl/custom-certs")
		sslCertDirs = append(sslCertDirs, "/etc/ssl/custom-certs")
	}

	var cloudSQLInstanceID *string
	if vars.Environment.Resources != nil && vars.Environment.Resources.PostgreSQL != nil {
		pgSpec := *vars.Environment.Resources.PostgreSQL
		sqlInstance, err := cloudsql.New(stack, resourceid.New("postgresql"), cloudsql.Config{
			ProjectID: vars.ProjectID,
			Region:    locationSpec.GCPRegion,
			Spec:      pgSpec,
			Network:   privateNetwork().Network,

			WorkloadIdentity:       *vars.IAM.CloudRunWorkloadServiceAccount,
			OperatorAccessIdentity: *vars.IAM.OperatorAccessServiceAccount,

			PreventDestroys: vars.PreventDestroys,

			// ServiceNetworkingConnection is required for Cloud SQL to connect
			// to the private network, so we must wait for it to be provisioned.
			// See https://cloud.google.com/sql/docs/mysql/private-ip#network_requirements
			DependsOn: []cdktf.ITerraformDependable{
				privateNetwork().ServiceNetworkingConnection,
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to render Cloud SQL instance")
		}

		cloudSQLInstanceID = sqlInstance.Instance.Id()

		// Add parameters required for authentication
		cloudRunBuilder.AddEnv("PGINSTANCE", *sqlInstance.Instance.ConnectionName())
		cloudRunBuilder.AddEnv("PGUSER", *sqlInstance.WorkloadUser.Name())
		// NOTE: https://pkg.go.dev/cloud.google.com/go/cloudsqlconn#section-readme
		// magically handles certs for us, so we don't need to mount certs in
		// Cloud Run.

		// There are additional runtime configuration we need to apply directly
		// in the PostgreSQL instance. To do this we use a different provider
		// authenticated by the users we just created.
		//
		// Some of the providers are only used if certain configurations are
		// enabled, but we create them all up-front to make teardown scenarios
		// easier to manage.
		pgRuntimeAdminProvider := postgresql.NewPostgresqlProvider(stack,
			id.TerraformID("postgresql_admin_provider"),
			&postgresql.PostgresqlProviderConfig{
				Alias:     pointers.Ptr("postgresql_admin_provider"),
				Scheme:    pointers.Ptr("gcppostgres"),
				Host:      sqlInstance.Instance.ConnectionName(),
				Port:      pointers.Float64(5432),
				Superuser: pointers.Ptr(false),

				Username: sqlInstance.AdminUser.Name(),
				Password: sqlInstance.AdminUser.Password(),
			})
		// Some configurations require impersonating the workload identity, for
		// things like database tables that are likely provisioned by the
		// application.
		pgRuntimeWorkloadUserProvider := postgresql.NewPostgresqlProvider(stack,
			id.TerraformID("postgresql_workloaduser_provider"),
			&postgresql.PostgresqlProviderConfig{
				Alias:     pointers.Ptr("postgresql_workloaduser_provider"),
				Scheme:    pointers.Ptr("gcppostgres"),
				Host:      sqlInstance.Instance.ConnectionName(),
				Port:      pointers.Float64(5432),
				Superuser: pointers.Ptr(false),

				// Impersonate the workload identity
				Username:                        sqlInstance.WorkloadUser.Name(),
				GcpIamImpersonateServiceAccount: &vars.IAM.CloudRunWorkloadServiceAccount.Email,
			})
		// The admin user's cloudsqlsuperuser does not have replication enabled,
		// so we need another user that does have it enabled, because replication
		// permission in roles are not inherited. We use the Postgres provider
		// instead of the Cloud SQL providers in 'cloudsql.New' so that we can
		// enable replication on this user.
		replicationUser := role.NewRole(stack, id.TerraformID("postgresql_replicationuser"), &role.RoleConfig{
			Provider: pgRuntimeAdminProvider,
			Name:     pointers.Ptr("msp-replicationuser"),
			Password: password.NewPassword(stack, id.TerraformID("postgresql_replicationuser_password"), &password.PasswordConfig{
				Length:  pointers.Float64(32),
				Special: pointers.Ptr(false),
			}).Result(),
			Login:       pointers.Ptr(true),
			Replication: pointers.Ptr(true),
		})
		pgRuntimeReplicationProvider := postgresql.NewPostgresqlProvider(stack,
			id.TerraformID("postgresql_replicationuser_provider"),
			&postgresql.PostgresqlProviderConfig{
				Alias:     pointers.Ptr("postgresql_replicationuser_provider"),
				Scheme:    pointers.Ptr("gcppostgres"),
				Host:      sqlInstance.Instance.ConnectionName(),
				Port:      pointers.Float64(5432),
				Superuser: pointers.Ptr(false),

				Username: replicationUser.Name(),
				Password: replicationUser.Password(),
			})

		// Apply runtime configuration
		var publications []postgresqllogicalreplication.PublicationOutput
		if pgSpec.LogicalReplication != nil {
			replication, err := postgresqllogicalreplication.New(stack,
				id.Group("postgresqllogicalreplication"),
				postgresqllogicalreplication.Config{
					AdminPostgreSQLProvider:        pgRuntimeAdminProvider,
					WorkloadUserPostgreSQLProvider: pgRuntimeWorkloadUserProvider,
					ReplicationPostgreSQLProvider:  pgRuntimeReplicationProvider,
					CloudSQL:                       sqlInstance,
					Spec:                           *pgSpec.LogicalReplication,

					DependsOn: []cdktf.ITerraformDependable{
						// Since tables are managed by the application, in the
						// future, we may need to for the application before we
						// provision a publication on tables that may not yet
						// exist. This is currently a circular dependency - the
						// Cloud Run resource does not need logical replication
						// config to start, but we cannot reference the Cloud Run
						// resource the way the codebase is structured now without
						// a bit of trickery or refactoring.
					},
				})
			if err != nil {
				return nil, errors.Wrap(err, "failed to render Cloud SQL PostgreSQL logical replication")
			}
			publications = replication.Publications

			// Make output visible for configuration in consumer tools like
			// Datastream
			for _, pub := range replication.Publications {
				locals.Add(fmt.Sprintf("postgresql_publication_%s_name", pub.Name),
					*pub.PublicationName,
					fmt.Sprintf("Publication name of the %q PostgreSQL publication", pub.Name))
				locals.Add(fmt.Sprintf("postgresql_publication_%s_replicationslot", pub.Name),
					*pub.ReplicationSlotName,
					fmt.Sprintf("Replication slot name of the %q PostgreSQL publication", pub.Name))
				locals.Add(fmt.Sprintf("postgresql_publication_%s_user", pub.Name),
					*pub.User.Name(),
					fmt.Sprintf("User for subscribing to the %q PostgreSQL publication", pub.Name))
				gsmsecret.New(stack,
					id.Group("postgresqllogicalreplication_output").
						Group(pub.Name).
						Group("user_password"),
					gsmsecret.Config{
						ProjectID: vars.ProjectID,
						ID: fmt.Sprintf("POSTGRESQL_PUBLICATION_%s_USER_PASSWORD",
							strings.ToUpper(pub.Name)),
						Value: *pub.User.Password(),
					})
			}
		}
		pgRoles, err := postgresqlroles.New(stack, id.Group("postgresqlroles"), postgresqlroles.Config{
			PostgreSQLProvider: pgRuntimeAdminProvider,
			Databases:          pgSpec.Databases,
			CloudSQL:           sqlInstance,
			Publications:       publications,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to render Cloud SQL PostgreSQL roles")
		}

		// We need the workload superuser role to be granted before Cloud Run
		// can correctly use the database instance
		cloudRunBuilder.AddDependency(pgRoles.WorkloadSuperuserGrant)

		// Add output for connecting to the instance
		locals.Add("cloudsql_connection_name", *sqlInstance.Instance.ConnectionName(),
			"Cloud SQL database connection name")
	}

	// bigqueryDataset is only created and non-nil if BigQuery is configured for
	// the environment.
	if vars.Environment.Resources != nil && vars.Environment.Resources.BigQueryDataset != nil {
		bigqueryDataset, err := bigquery.New(stack, resourceid.New("bigquery"), bigquery.Config{
			DefaultProjectID:       vars.ProjectID,
			ServiceID:              vars.Service.ID,
			WorkloadServiceAccount: vars.IAM.CloudRunWorkloadServiceAccount,
			Spec:                   *vars.Environment.Resources.BigQueryDataset,
			Locations:              locationSpec,
			PreventDestroys:        vars.PreventDestroys,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to render BigQuery dataset")
		}

		// Add parameters required for writing to the correct BigQuery dataset
		cloudRunBuilder.AddEnv("BIGQUERY_PROJECT_ID", bigqueryDataset.ProjectID)
		cloudRunBuilder.AddEnv("BIGQUERY_DATASET_ID", bigqueryDataset.DatasetID)

		// Make sure tables are available before Cloud Run
		for _, t := range bigqueryDataset.Tables {
			cloudRunBuilder.AddDependency(t)
		}
	}

	// Sentry
	var sentryProject sentryproject.Project
	{
		id := id.Group("sentry")
		// Get the Sentry organization
		organization := datasentryorganization.NewDataSentryOrganization(stack, id.TerraformID("organization"), &datasentryorganization.DataSentryOrganizationConfig{
			Slug: pointers.Ptr(SentryOrganization),
		})

		// Get the Sourcegraph team - we don't use individual owner teams
		// because it's hard to tell whether they already exist or not, and
		// it's not really important enough to force operators to create a
		// team by hand. We depend on Opsgenie teams for concrete ownership
		// instead.
		sentryTeam := datasentryteam.NewDataSentryTeam(stack, id.TerraformID("team"), &datasentryteam.DataSentryTeamConfig{
			Organization: organization.Id(),
			Slug:         pointers.Ptr("sourcegraph"),
		})

		// Create the project
		sentryProject = sentryproject.NewProject(stack, id.TerraformID("project"), &sentryproject.ProjectConfig{
			Organization: organization.Id(),
			Name:         pointers.Stringf("%s - %s", vars.Service.GetName(), vars.Environment.ID),
			Slug:         pointers.Stringf("%s-%s", vars.Service.ID, vars.Environment.ID),
			Teams:        &[]*string{sentryTeam.Slug()},
			DefaultRules: pointers.Ptr(false),
		})

		// Create a DSN
		key := key.NewKey(stack, id.TerraformID("dsn"), &key.KeyConfig{
			Organization: organization.Id(),
			Project:      sentryProject.Slug(),
			Name:         pointers.Ptr("Managed Servcies Platform"),
		})

		cloudRunBuilder.AddEnv("SENTRY_DSN", *key.DsnPublic())
	}

	// Finalize output of builder
	cloudRunBuilder.AddEnv("SSL_CERT_DIR", strings.Join(sslCertDirs, ":"))
	cloudRunResource, err := cloudRunBuilder.Build(stack, builder.Variables{
		Service:           vars.Service,
		Image:             vars.Image,
		ImageTag:          imageTag,
		Environment:       vars.Environment,
		GCPProjectID:      vars.ProjectID,
		GCPRegion:         locationSpec.GCPRegion,
		ServiceAccount:    vars.IAM.CloudRunWorkloadServiceAccount,
		DiagnosticsSecret: diagnosticsSecret,
		ResourceLimits:    makeContainerResourceLimits(vars.Environment.Instances.Resources),
		PrivateNetwork: func() *privatenetwork.Output {
			if privateNetworkEnabled {
				return privateNetwork()
			}
			return nil
		}(),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "build Cloud Run resource kind %q", cloudRunBuilder.Kind())
	}

	// We have a rollout pipeline to configure - Cloud Deploy pipeline lives in
	// the final stage of the pipeline.
	if vars.RolloutPipeline.IsFinalStage() {
		id := id.Group("rolloutpipeline")

		// For now, we only use 1 region everywhere, but also note that ALL
		// deployment targets must be in the same location as the delivery
		// pipeline, so if we ever do multi-region we'll need multiple delivery
		// pipelines for each. In particular, see https://registry.terraform.io/providers/hashicorp/google/5.10.0/docs/resources/clouddeploy_delivery_pipeline#target_id:
		//
		// > The location of the Target is inferred to be the same as the location of the DeliveryPipeline that contains this Stage.
		//
		// Updated note: in theory we can change this since we now use a custom
		// rollout target, but in may be good practice to keep separate regions
		// separated.
		var rolloutLocation = locationSpec.GCPRegion

		// stageTargets enumerate stages in order. Cloud Deploy targets are
		// created separately because the TF provider doesn't support Custom
		// Targets yet - TODO document
		var stageTargets []deliverypipeline.Target
		for _, stage := range vars.RolloutPipeline.Stages {
			id := id.Group("stage").Group(stage.EnvironmentID)

			// Our execution service account needs access to this project's
			// resources to deploy releases.
			_ = projectiammember.NewProjectIamMember(stack,
				id.Group("cloudrun_developer").TerraformID("member"),
				&projectiammember.ProjectIamMemberConfig{
					Project: pointers.Ptr(stage.ProjectID),
					Role:    pointers.Ptr("roles/run.developer"),
					Member:  &vars.IAM.CloudDeployExecutionServiceAccount.Member,
				})
			_ = projectiammember.NewProjectIamMember(stack,
				id.Group("service_account_user").TerraformID("member"),
				&projectiammember.ProjectIamMemberConfig{
					Project: pointers.Ptr(stage.ProjectID),
					Role:    pointers.Ptr("roles/iam.serviceAccountUser"),
					Member:  &vars.IAM.CloudDeployExecutionServiceAccount.Member,
				})

			stageTargets = append(stageTargets, deliverypipeline.Target{
				// Name targets with environment+location - this is expected by
				// our Cloud Deploy Custom Target
				ID:        fmt.Sprintf("%s-%s", stage.EnvironmentID, rolloutLocation),
				ProjectID: stage.ProjectID,
			})
		}

		// Now, apply each target in a rollout pipeline. The targets don't need
		// to exist at this point yet, though attempting to use the pipeline
		// before creating targets will fail.
		deliveryPipeline, _ := deliverypipeline.New(stack, id.Group("pipeline"), deliverypipeline.Config{
			Name: fmt.Sprintf("%s-%s-rollout", vars.Service.ID, rolloutLocation),
			Description: fmt.Sprintf("Rollout delivery pipeline for %s",
				vars.Service.GetName()),
			Location: rolloutLocation,

			ServiceID:    vars.Service.ID,
			ServiceImage: vars.Image,
			ExecutionSA:  vars.IAM.CloudDeployExecutionServiceAccount,

			TargetStages: stageTargets,

			Repository: vars.Repository,

			Suspended: pointers.DerefZero(vars.RolloutPipeline.OriginalSpec.Suspended),

			// Make it so that our Cloud Run service is up before we
			// configure the rollout pipeline
			DependsOn: []cdktf.ITerraformDependable{
				cloudRunResource,
			},
		})

		// We also need to synchronize the Skaffold configuration for our custom
		// target, so that we can reference it easily without requiring operators
		// to have the required Skaffold assets for 'gcloud deploy releases create'
		// locally.
		skaffoldBucket := storagebucket.NewStorageBucket(stack, id.Group("skaffold").TerraformID("bucket"), &storagebucket.StorageBucketConfig{
			Name:     pointers.Stringf("%s-cloudrun-skaffold", vars.ProjectID),
			Location: &rolloutLocation,
		})
		_ = storagebucketobject.NewStorageBucketObject(stack, id.Group("skaffold").TerraformID("object"), &storagebucketobject.StorageBucketObjectConfig{
			Name:        pointers.Ptr("source.tar.gz"),
			Bucket:      skaffoldBucket.Name(),
			Source:      pointers.Ptr(ScaffoldSourceFile), // see docstring for hack
			ContentType: pointers.Ptr("application/gzip"),
		})

		// `<pipeline_uid>_clouddeploy` bucket is normally created when the pipeline is first used
		// We manually create it so we can provision IAM access
		pipelineBucket := storagebucket.NewStorageBucket(stack, id.Group("pipeline").TerraformID("bucket"), &storagebucket.StorageBucketConfig{
			Name:     pointers.Stringf("%s_clouddeploy", deliveryPipeline.PipelineID),
			Location: &rolloutLocation,
		})

		// Provision Service Account IAM to create releases
		serviceAccounts := []string{
			vars.IAM.CloudDeployReleaserServiceAccount.Email,
		}
		if sa := pointers.DerefZero(vars.RolloutPipeline.OriginalSpec.ServiceAccount); sa != "" {
			serviceAccounts = append(serviceAccounts, sa)
		}
		addCloudDeployIAM(vars, id, stack, cloudDeployIAMConfig{
			serviceAccounts:    serviceAccounts,
			skaffoldBucketName: skaffoldBucket.Name(),
			pipelineBucketName: pipelineBucket.Name(),
		})

		// Create the Pub/Sub topic that receives notifications for Cloud Deploy events,
		// see https://cloud.google.com/deploy/docs/subscribe-deploy-notifications#available_topics for topic info.
		topic := pubsubtopic.NewPubsubTopic(stack, id.TerraformID("clouddeploy-operations-topic"), &pubsubtopic.PubsubTopicConfig{
			Name: pointers.Ptr("clouddeploy-operations"),
		})

		// Get cloud-relay endpoint from GSM.
		endpoint := gsmsecret.Get(stack, id.Group("cloudrelay-endpoint"), gsmsecret.DataConfig{
			ProjectID: googlesecretsmanager.SharedSecretsProjectID,
			Secret:    googlesecretsmanager.SecretMSPDeployNotificationEndpoint,
		})

		_ = pubsubsubscription.NewPubsubSubscription(stack, id.TerraformID("clouddeploy-operations-sub"), &pubsubsubscription.PubsubSubscriptionConfig{
			Name:  pointers.Ptr("clouddeploy-operations"),
			Topic: topic.Id(),
			PushConfig: &pubsubsubscription.PubsubSubscriptionPushConfig{
				PushEndpoint: &endpoint.Value,
			},
			// Only retain un-acked messages for 1 hour
			// the notifications aren't critical so they can be dropped after
			// a reasonable amount of time.
			MessageRetentionDuration: pointers.Ptr("3600s"),
			// We don't want the subscription to expire if there hasn't been a rollout in 31 days.
			ExpirationPolicy: &pubsubsubscription.PubsubSubscriptionExpirationPolicy{
				Ttl: pointers.Ptr(""),
			},
		})

	}

	// Collect outputs
	locals.Add("cloud_run_resource_name", *cloudRunResource.Name(),
		"Cloud Run resource name")
	locals.Add("cloud_run_location", *cloudRunResource.Location(),
		"Cloud Run resource location")
	return &CrossStackOutput{
		DiagnosticsSecret:  diagnosticsSecret,
		RedisInstanceID:    redisInstanceID,
		CloudSQLInstanceID: cloudSQLInstanceID,
		SentryProject:      sentryProject,
	}, nil
}

type envVariablesData struct {
	ProjectID      string
	ServiceDnsName string
}

func addContainerEnvVars(
	b builder.Builder,
	env map[string]string,
	secretEnv map[string]string,
	varsData envVariablesData,
) error {
	// Apply static env vars
	envKeys := maps.Keys(env)
	slices.Sort(envKeys)
	for _, k := range envKeys {
		tmpl, err := template.New("").Parse(env[k])
		if err != nil {
			return errors.Wrapf(err, "parse env var template: %q", env[k])
		}
		var buf bytes.Buffer
		if err = tmpl.Execute(&buf, varsData); err != nil {
			return errors.Wrapf(err, "execute template: %q", env[k])
		}

		b.AddEnv(k, buf.String())
	}

	// Apply secret env vars
	secretEnvKeys := maps.Keys(secretEnv)
	slices.Sort(secretEnvKeys)
	for _, k := range secretEnvKeys {
		b.AddSecretEnv(k, builder.SecretRef{
			Name:    secretEnv[k],
			Version: "latest",
		})
	}

	return nil
}

// addContainerSecretVolumes adds secret volumes to the container, and mounts
// https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/cloud_run_v2_service#example-usage---cloudrunv2-service-secret
func addContainerSecretVolumes(
	b builder.Builder,
	volumes map[string]spec.EnvironmentSecretVolume,
) {
	keys := maps.Keys(volumes)
	slices.Sort(keys)
	for _, k := range keys {
		v := volumes[k]

		// in secretVolume, we specify the name (filename) of the secret in the volume
		dir, file := filepath.Split(v.MountPath)
		b.AddSecretVolume(k, file,
			builder.SecretRef{
				Name:    v.Secret,
				Version: "latest",
			},
			292, // 0444 read-only
		)
		// then, we mount the secretVolume to the desired path in the container
		b.AddVolumeMount(k, dir)
	}
}

func makeContainerResourceLimits(r spec.EnvironmentInstancesResourcesSpec) map[string]*string {
	return map[string]*string{
		"cpu":    pointers.Ptr(strconv.Itoa(r.CPU)),
		"memory": pointers.Ptr(r.Memory),
	}
}

type cloudDeployIAMConfig struct {
	serviceAccounts    []string
	skaffoldBucketName *string
	pipelineBucketName *string
}

// addCloudDeployIAM needs to be done here rather than the IAM stack as
// the Delivery Pipeline needs to be created first
func addCloudDeployIAM(vars Variables, id resourceid.ID, stack cdktf.TerraformStack, config cloudDeployIAMConfig) {
	// Create custom role to list buckets
	listbuckets := projectiamcustomrole.NewProjectIamCustomRole(stack, id.TerraformID("listbucketsrole"), &projectiamcustomrole.ProjectIamCustomRoleConfig{
		Project:     pointers.Ptr(vars.ProjectID),
		RoleId:      pointers.Ptr("clouddeploy_listbuckets"),
		Title:       pointers.Ptr("Cloud Deploy: List buckets"),
		Permissions: &[]*string{pointers.Ptr("storage.buckets.list")},
	})

	for i, sa := range config.serviceAccounts {
		id := id.Group("%d_serviceaccount", i)
		// Permission to create releases
		_ = projectiammember.NewProjectIamMember(stack, id.TerraformID("releaser"), &projectiammember.ProjectIamMemberConfig{
			Project: pointers.Ptr(vars.ProjectID),
			Role:    pointers.Ptr("roles/clouddeploy.releaser"),
			Member:  pointers.Stringf("serviceAccount:%s", sa),
		})

		// Needs access to `<pipeline_id>_clouddeploy` bucket
		_ = storagebucketiammember.NewStorageBucketIamMember(stack, id.TerraformID("clouddeploy"), &storagebucketiammember.StorageBucketIamMemberConfig{
			Bucket: config.pipelineBucketName,
			Role:   pointers.Ptr("roles/storage.admin"),
			Member: pointers.Stringf("serviceAccount:%s", sa),
		})

		// Needs access to the skaffold source bucket
		_ = storagebucketiammember.NewStorageBucketIamMember(stack, id.TerraformID("skaffold"), &storagebucketiammember.StorageBucketIamMemberConfig{
			Bucket: config.skaffoldBucketName,
			Role:   pointers.Ptr("roles/storage.admin"),
			Member: pointers.Stringf("serviceAccount:%s", sa),
		})

		// // Needs to be able to list buckets
		_ = projectiammember.NewProjectIamMember(stack, id.TerraformID("listbuckets"), &projectiammember.ProjectIamMemberConfig{
			Project: pointers.Ptr(vars.ProjectID),
			Role:    listbuckets.Id(),
			Member:  pointers.Stringf("serviceAccount:%s", sa),
		})

		// Needs to be able to ActAs `clouddeply-executor` SA
		_ = serviceaccountiammember.NewServiceAccountIamMember(stack, id.TerraformID("executor"), &serviceaccountiammember.ServiceAccountIamMemberConfig{
			ServiceAccountId: pointers.Stringf("projects/%s/serviceAccounts/%s", vars.ProjectID, vars.IAM.CloudDeployExecutionServiceAccount.Email),
			Role:             pointers.Ptr("roles/iam.serviceAccountUser"),
			Member:           pointers.Stringf("serviceAccount:%s", sa),
		})
	}
}
