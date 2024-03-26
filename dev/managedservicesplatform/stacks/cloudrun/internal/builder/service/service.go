package service

import (
	"fmt"

	"github.com/hashicorp/terraform-cdk-go/cdktf"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/cloudrunv2service"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/cloudrunv2serviceiammember"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/cloudflare"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/cloudflareorigincert"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/loadbalancer"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/managedcert"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resourceid"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/spec"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/cloudrun/internal/builder"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// builder parameterizes configurable components of the core
// Cloud Run Service. It's particularly useful for strongly typing fields that
// the generated CDKTF library accepts as interface{} types.
type serviceBuilder struct {
	env          []*cloudrunv2service.CloudRunV2ServiceTemplateContainersEnv
	volumes      []*cloudrunv2service.CloudRunV2ServiceTemplateVolumes
	volumeMounts []*cloudrunv2service.CloudRunV2ServiceTemplateContainersVolumeMounts
	dependencies []cdktf.ITerraformDependable
}

var _ builder.Builder = (*serviceBuilder)(nil)

// NewBuilder returns a builder for a Cloud Run Service, translating env/volumes/etc
// to cloudrunv2service equivalents of resources.
func NewBuilder() builder.Builder {
	return &serviceBuilder{}
}

func (b *serviceBuilder) Kind() spec.ServiceKind { return spec.ServiceKindService }

func (b *serviceBuilder) AddEnv(key, value string) {
	b.env = append(b.env, &cloudrunv2service.CloudRunV2ServiceTemplateContainersEnv{
		Name:  pointers.Ptr(key),
		Value: pointers.Ptr(value),
	})
}

func (b *serviceBuilder) AddSecretEnv(key string, secret builder.SecretRef) {
	b.env = append(b.env, &cloudrunv2service.CloudRunV2ServiceTemplateContainersEnv{
		Name: pointers.Ptr(key),
		ValueSource: &cloudrunv2service.CloudRunV2ServiceTemplateContainersEnvValueSource{
			SecretKeyRef: &cloudrunv2service.CloudRunV2ServiceTemplateContainersEnvValueSourceSecretKeyRef{
				Secret:  pointers.Ptr(secret.Name),
				Version: pointers.Ptr(secret.Version),
			},
		},
	})
}

func (b *serviceBuilder) AddVolumeMount(name, mountPath string) {
	b.volumeMounts = append(b.volumeMounts, &cloudrunv2service.CloudRunV2ServiceTemplateContainersVolumeMounts{
		Name:      pointers.Ptr(name),
		MountPath: pointers.Ptr(mountPath),
	})
}

func (b *serviceBuilder) AddSecretVolume(name, mountPath string, secret builder.SecretRef, mode int) {
	b.volumes = append(b.volumes, &cloudrunv2service.CloudRunV2ServiceTemplateVolumes{
		Name: pointers.Ptr(name),
		Secret: &cloudrunv2service.CloudRunV2ServiceTemplateVolumesSecret{
			Secret: &secret.Name,
			Items: []*cloudrunv2service.CloudRunV2ServiceTemplateVolumesSecretItems{{
				Version: pointers.Ptr(secret.Version),
				Mode:    pointers.Ptr(float64(mode)),
				Path:    &mountPath,
			}},
		},
	})
}

// AddDependency ensures that particular Terraform resources are provisioned
// before the Cloud Run resource is created.
func (b *serviceBuilder) AddDependency(dep cdktf.ITerraformDependable) {
	b.dependencies = append(b.dependencies, dep)
}

func (b *serviceBuilder) Build(stack cdktf.TerraformStack, vars builder.Variables) (builder.Resource, error) {
	var vpcAccess *cloudrunv2service.CloudRunV2ServiceTemplateVpcAccess
	var launchStage *string
	if vars.PrivateNetwork != nil {
		// https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/cloud_run_v2_service#example-usage---cloudrunv2-service-directvpc
		// https://cloud.google.com/run/docs/configuring/vpc-direct-vpc
		launchStage = pointers.Ptr("BETA") // Direct VPC is still in beta.
		vpcAccess = &cloudrunv2service.CloudRunV2ServiceTemplateVpcAccess{
			NetworkInterfaces: &[]*cloudrunv2service.CloudRunV2ServiceTemplateVpcAccessNetworkInterfaces{{
				Network:    vars.PrivateNetwork.Network.Id(),
				Subnetwork: vars.PrivateNetwork.Subnetwork.Id(),
			}},
			Egress: pointers.Ptr("ALL_TRAFFIC"),
		}
	}

	// For convenience
	if vars.Environment.Instances.Scaling == nil {
		vars.Environment.Instances.Scaling = &spec.EnvironmentInstancesScalingSpec{}
	}

	var executionEnvironment *string
	if generation := vars.Environment.Instances.Resources.CloudRunGeneration; generation != nil {
		switch *generation {
		case 1:
			executionEnvironment = pointers.Ptr("EXECUTION_ENVIRONMENT_GEN1")
		case 2:
			executionEnvironment = pointers.Ptr("EXECUTION_ENVIRONMENT_GEN2")
		}
	}

	var lifecycle *cdktf.TerraformResourceLifecycle
	if vars.Environment.Deploy.Type == spec.EnvironmentDeployTypeRollout {
		lifecycle = &cdktf.TerraformResourceLifecycle{
			IgnoreChanges: &[]*string{
				// This will be managed by Cloud Deploy releases issued by
				// the service owner, e.g. via their CI.
				pointers.Ptr("template[0].containers[0].image"),
				// These will be set when a revision is created via our Cloud
				// Deploy custom target when a release is deployed.
				pointers.Ptr("client"),
				pointers.Ptr("client_version"),
			},
		}
	}

	name, err := vars.Name()
	if err != nil {
		return nil, err
	}
	svc := cloudrunv2service.NewCloudRunV2Service(stack, pointers.Ptr("cloudrun"), &cloudrunv2service.CloudRunV2ServiceConfig{
		Name:      pointers.Ptr(name),
		Location:  pointers.Ptr(vars.GCPRegion),
		DependsOn: &b.dependencies,
		Lifecycle: lifecycle,

		LaunchStage: launchStage,

		//  Disallows direct traffic from public internet, we have a LB set up for that.
		Ingress: pointers.Ptr("INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"),

		// Send all traffic to the latest revison.
		// This is needed to override changes to traffic configuration from the UI. Otherwise,
		// it's possible that traffic will always be routed to a stale revision after new deployment.
		//
		// https://cloud.google.com/run/docs/rollouts-rollbacks-traffic-migration#send-to-latest
		Traffic: []*cloudrunv2service.CloudRunV2ServiceTraffic{
			{
				Percent: pointers.Float64(100),
				Type:    pointers.Ptr("TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"),
			},
		},

		Template: &cloudrunv2service.CloudRunV2ServiceTemplate{
			// Act under our provisioned service account
			ServiceAccount: pointers.Ptr(vars.ServiceAccount.Email),

			// Connect to VPC connector for talking to other GCP services.
			VpcAccess: vpcAccess,

			// Set a high limit that matches our default Cloudflare zone's
			// timeout:
			//
			//   export CF_API_TOKEN=$(gcloud secrets versions access latest --secret CLOUDFLARE_API_TOKEN --project sourcegraph-secrets)
			//   curl -H "Authorization: Bearer $CF_API_TOKEN" https://api.cloudflare.com/client/v4/zones | jq '.result[]  | select(.name == "sourcegraph.com") | .id'
			//   curl -H "Authorization: Bearer $CF_API_TOKEN" https://api.cloudflare.com/client/v4/zones/$CF_ZONE_ID/settings | jq '.result[] | select(.id == "proxy_read_timeout")'
			//
			// Result should be something like:
			//
			//   {
			//     "id": "proxy_read_timeout",
			//     "value": "300",
			//     "modified_on": "2022-02-08T23:10:35.772888Z",
			//     "editable": true
			//   }
			//
			// The service should implement tighter timeouts on its own if desired.
			Timeout: pointers.Ptr("300s"),

			// Scaling configuration
			MaxInstanceRequestConcurrency: pointers.Float64(
				pointers.Deref(vars.Environment.Instances.Scaling.MaxRequestConcurrency, builder.DefaultMaxConcurrentRequests)),
			Scaling: &cloudrunv2service.CloudRunV2ServiceTemplateScaling{
				MinInstanceCount: pointers.Float64(vars.Environment.Instances.Scaling.MinCount),
				MaxInstanceCount: pointers.Float64(
					pointers.Deref(vars.Environment.Instances.Scaling.MaxCount, builder.DefaultMaxInstances)),
			},
			ExecutionEnvironment: executionEnvironment,

			// Configuration for the single service container.
			Containers: []*cloudrunv2service.CloudRunV2ServiceTemplateContainers{{
				Name:  pointers.Ptr(vars.Service.ID),
				Image: pointers.Ptr(fmt.Sprintf("%s:%s", vars.Image, vars.ResolvedImageTag)),

				Resources: &cloudrunv2service.CloudRunV2ServiceTemplateContainersResources{
					Limits: &vars.ResourceLimits,
				},

				Ports: []*cloudrunv2service.CloudRunV2ServiceTemplateContainersPorts{{
					// ContainerPort is provided to the container as $PORT in Cloud Run
					ContainerPort: pointers.Float64(builder.ServicePort),
					// Name is protocol, supporting 'h2c', 'http1', or nil (http1)
					Name: (*string)(vars.Service.Protocol),
				}},

				Env: b.env,

				// Do healthchecks with authorization based on MSP convention.
				StartupProbe: func() *cloudrunv2service.CloudRunV2ServiceTemplateContainersStartupProbe {
					if !vars.Environment.HealthProbes.UseHealthzProbes() {
						return nil
					}

					return &cloudrunv2service.CloudRunV2ServiceTemplateContainersStartupProbe{
						HttpGet: &cloudrunv2service.CloudRunV2ServiceTemplateContainersStartupProbeHttpGet{
							Path: pointers.Ptr(builder.HealthCheckEndpoint),
							HttpHeaders: []*cloudrunv2service.CloudRunV2ServiceTemplateContainersStartupProbeHttpGetHttpHeaders{{
								Name:  pointers.Ptr("Authorization"),
								Value: pointers.Ptr(fmt.Sprintf("Bearer %s", vars.DiagnosticsSecret.HexValue)),
							}},
						},
						InitialDelaySeconds: pointers.Float64(0),
						TimeoutSeconds:      pointers.Float64(vars.Environment.HealthProbes.GetTimeoutSeconds()),
						PeriodSeconds:       pointers.Float64(vars.Environment.HealthProbes.GetStartupIntervalSeconds()),
						FailureThreshold:    pointers.Float64(3),
					}
				}(),
				LivenessProbe: func() *cloudrunv2service.CloudRunV2ServiceTemplateContainersLivenessProbe {
					if !vars.Environment.HealthProbes.UseHealthzProbes() {
						return nil
					}

					return &cloudrunv2service.CloudRunV2ServiceTemplateContainersLivenessProbe{
						HttpGet: &cloudrunv2service.CloudRunV2ServiceTemplateContainersLivenessProbeHttpGet{
							Path: pointers.Ptr(builder.HealthCheckEndpoint),
							HttpHeaders: []*cloudrunv2service.CloudRunV2ServiceTemplateContainersLivenessProbeHttpGetHttpHeaders{{
								Name:  pointers.Ptr("Authorization"),
								Value: pointers.Ptr(fmt.Sprintf("Bearer %s", vars.DiagnosticsSecret.HexValue)),
							}},
						},
						TimeoutSeconds:   pointers.Float64(vars.Environment.HealthProbes.GetTimeoutSeconds()),
						PeriodSeconds:    pointers.Float64(vars.Environment.HealthProbes.GetLivenessIntervalSeconds()),
						FailureThreshold: pointers.Float64(3),
					}
				}(),

				VolumeMounts: b.volumeMounts,
			}},

			Volumes: b.volumes,
		}})

	// Allow IAM-free access to the service by default - auth should be handled
	// generally by the service itself.
	if vars.Environment.Authentication == nil {
		_ = cloudrunv2serviceiammember.NewCloudRunV2ServiceIamMember(stack, pointers.Ptr("cloudrun-allusers-runinvoker"), &cloudrunv2serviceiammember.CloudRunV2ServiceIamMemberConfig{
			Name:     svc.Name(),
			Location: svc.Location(),
			Project:  &vars.GCPProjectID,
			Member:   pointers.Ptr("allUsers"),
			Role:     pointers.Ptr("roles/run.invoker"),
		})
	} else if pointers.DerefZero(vars.Environment.Authentication.Sourcegraph) {
		_ = cloudrunv2serviceiammember.NewCloudRunV2ServiceIamMember(stack, pointers.Ptr("cloudrun-domain-runinvoker"), &cloudrunv2serviceiammember.CloudRunV2ServiceIamMemberConfig{
			Name:     svc.Name(),
			Location: svc.Location(),
			Project:  &vars.GCPProjectID,
			// https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/cloud_run_v2_service_iam#member/members
			Member: pointers.Ptr("domain:sourcegraph.com"),
			Role:   pointers.Ptr("roles/run.invoker"),
		})
	}

	// Then add whatever the user requested to expose the service publicly
	switch domain := pointers.DerefZero(vars.Environment.Domain); domain.Type {
	case "", spec.EnvironmentDomainTypeNone:
		// do nothing

	case spec.EnvironmentDomainTypeCloudflare:
		// set zero value for convenience
		if domain.Cloudflare == nil {
			return nil, errors.Newf("domain type %q specified but Cloudflare configuration is nil",
				domain.Type)
		}
		if domain.Cloudflare.Subdomain == "" || domain.Cloudflare.Zone == "" {
			return nil, errors.Newf("domain type %q requires 'cloudflare.subdomain' and 'cloudflare.zone' to be set",
				domain.Type)
		}

		// Provision SSL cert
		var sslCertificate loadbalancer.SSLCertificate
		if domain.Cloudflare.ShouldProxy() {
			sslCertificate = cloudflareorigincert.New(stack,
				resourceid.New("cf-origin-cert"),
				cloudflareorigincert.Config{
					ProjectID: vars.GCPProjectID,
				}).Certificate
		} else {
			sslCertificate = managedcert.New(stack,
				resourceid.New("managed-cert"),
				managedcert.Config{
					ProjectID: vars.GCPProjectID,
					Domain:    fmt.Sprintf("%s.%s", domain.Cloudflare.Subdomain, domain.Cloudflare.Zone),
				}).Certificate
		}

		// Create load-balancer pointing to Cloud Run service
		lb, err := loadbalancer.New(stack, resourceid.New("loadbalancer"), loadbalancer.Config{
			ProjectID:         vars.GCPProjectID,
			Region:            vars.GCPRegion,
			TargetService:     svc,
			SSLCertificate:    sslCertificate,
			CloudflareProxied: domain.Cloudflare.ShouldProxy(),
			Production:        vars.Environment.Category.IsProduction(),
			EnableLogging:     pointers.DerefZero(pointers.DerefZero(domain.Networking).LoadBalancerLogging),
		})
		if err != nil {
			return nil, errors.Wrap(err, "loadbalancer.New")
		}

		// Now set up a DNS record in Cloudflare to route to the load balancer
		if _, err := cloudflare.New(stack, resourceid.New("cf"), cloudflare.Config{
			Spec:   *vars.Environment.Domain.Cloudflare,
			Target: *lb,
		}); err != nil {
			return nil, err
		}
	}

	return svc, nil
}
