package job

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-cdk-go/cdktf"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/cloudrunv2job"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/cloudrunv2jobiammember"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/cloudschedulerjob"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/serviceaccount"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resourceid"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/spec"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/cloudrun/internal/builder"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type jobBuilder struct {
	env          []*cloudrunv2job.CloudRunV2JobTemplateTemplateContainersEnv
	volumes      []*cloudrunv2job.CloudRunV2JobTemplateTemplateVolumes
	volumeMounts []*cloudrunv2job.CloudRunV2JobTemplateTemplateContainersVolumeMounts
	dependencies []cdktf.ITerraformDependable
}

var _ builder.Builder = (*jobBuilder)(nil)

// NewBuilder returns a builder for a Cloud Run Job, translating env/volumes/etc
// to cloudrunv2job equivalents of resources.
func NewBuilder() builder.Builder {
	return &jobBuilder{}
}

func (b *jobBuilder) Kind() spec.ServiceKind { return spec.ServiceKindJob }

func (b *jobBuilder) AddEnv(key, value string) {
	b.env = append(b.env, &cloudrunv2job.CloudRunV2JobTemplateTemplateContainersEnv{
		Name:  pointers.Ptr(key),
		Value: pointers.Ptr(value),
	})
}

func (b *jobBuilder) AddSecretEnv(key string, secret builder.SecretRef) {
	b.env = append(b.env, &cloudrunv2job.CloudRunV2JobTemplateTemplateContainersEnv{
		Name: pointers.Ptr(key),
		ValueSource: &cloudrunv2job.CloudRunV2JobTemplateTemplateContainersEnvValueSource{
			SecretKeyRef: &cloudrunv2job.CloudRunV2JobTemplateTemplateContainersEnvValueSourceSecretKeyRef{
				Secret:  pointers.Ptr(secret.Name),
				Version: pointers.Ptr(secret.Version),
			},
		},
	})
}

func (b *jobBuilder) AddVolumeMount(name, mountPath string) {
	b.volumeMounts = append(b.volumeMounts, &cloudrunv2job.CloudRunV2JobTemplateTemplateContainersVolumeMounts{
		Name:      pointers.Ptr(name),
		MountPath: pointers.Ptr(mountPath),
	})
}

func (b *jobBuilder) AddSecretVolume(name, mountPath string, secret builder.SecretRef, mode int) {
	b.volumes = append(b.volumes, &cloudrunv2job.CloudRunV2JobTemplateTemplateVolumes{
		Name: pointers.Ptr(name),
		Secret: &cloudrunv2job.CloudRunV2JobTemplateTemplateVolumesSecret{
			Secret: &secret.Name,
			Items: []*cloudrunv2job.CloudRunV2JobTemplateTemplateVolumesSecretItems{{
				Version: pointers.Ptr(secret.Version),
				Mode:    pointers.Ptr(float64(mode)),
				Path:    &mountPath,
			}},
		},
	})
}

// AddDependency ensures that particular Terraform resources are provisioned
// before the Cloud Run resource is created.
func (b *jobBuilder) AddDependency(dep cdktf.ITerraformDependable) {
	b.dependencies = append(b.dependencies, dep)
}

func (b *jobBuilder) Build(stack cdktf.TerraformStack, vars builder.Variables) (builder.Resource, error) {
	var vpcAccess *cloudrunv2job.CloudRunV2JobTemplateTemplateVpcAccess
	var launchStage *string
	if vars.PrivateNetwork != nil {
		// https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/cloud_run_v2_service#example-usage---cloudrunv2-service-directvpc
		// https://cloud.google.com/run/docs/configuring/vpc-direct-vpc
		launchStage = pointers.Ptr("BETA") // Direct VPC is still in beta.
		vpcAccess = &cloudrunv2job.CloudRunV2JobTemplateTemplateVpcAccess{
			NetworkInterfaces: &[]*cloudrunv2job.CloudRunV2JobTemplateTemplateVpcAccessNetworkInterfaces{{
				Network:    vars.PrivateNetwork.Network.Id(),
				Subnetwork: vars.PrivateNetwork.Subnetwork.Id(),
			}},
			Egress: pointers.Ptr("ALL_TRAFFIC"),
		}
	}

	name, err := vars.Name()
	if err != nil {
		return nil, err
	}

	job := cloudrunv2job.NewCloudRunV2Job(stack, pointers.Ptr("cloudrun"), &cloudrunv2job.CloudRunV2JobConfig{
		Name:      pointers.Ptr(name),
		Location:  pointers.Ptr(vars.GCPRegion),
		DependsOn: &b.dependencies,

		LaunchStage: launchStage,

		Template: &cloudrunv2job.CloudRunV2JobTemplate{
			TaskCount: pointers.Ptr(float64(1)),

			Template: &cloudrunv2job.CloudRunV2JobTemplateTemplate{
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

				// Configuration for the single service container.
				Containers: []*cloudrunv2job.CloudRunV2JobTemplateTemplateContainers{{
					Name:  pointers.Ptr(vars.Service.ID),
					Image: pointers.Ptr(fmt.Sprintf("%s:%s", vars.Image, vars.ResolvedImageTag)),

					Resources: &cloudrunv2job.CloudRunV2JobTemplateTemplateContainersResources{
						Limits: &vars.ResourceLimits,
					},

					Ports: []*cloudrunv2job.CloudRunV2JobTemplateTemplateContainersPorts{{
						// ContainerPort is provided to the container as $PORT in Cloud Run
						ContainerPort: pointers.Float64(builder.ServicePort),
						// Name is protocol, supporting 'h2c', 'http1', or nil (http1)
						Name: (*string)(vars.Service.Protocol),
					}},

					Env: b.env,

					VolumeMounts: b.volumeMounts,

					// Job does not support probes
				}},

				Volumes: b.volumes,
			},
		}})

	if schedule := pointers.DerefZero(vars.Environment.EnvironmentJobSpec).Schedule; schedule != nil {
		invoker := serviceaccount.New(stack, resourceid.New("job_invoker"), serviceaccount.Config{
			ProjectID:   vars.GCPProjectID,
			AccountID:   fmt.Sprintf("%s-job-sa", vars.Service.ID),
			DisplayName: fmt.Sprintf("%s Job-Invoker Service Account", vars.Service.GetName()),
		})

		invokerMember := cloudrunv2jobiammember.NewCloudRunV2JobIamMember(stack, pointers.Ptr("cloudrun_scheduler_job_invoker"), &cloudrunv2jobiammember.CloudRunV2JobIamMemberConfig{
			Name:     job.Name(),
			Location: job.Location(),
			Project:  &vars.GCPProjectID,
			Member:   &invoker.Member,
			Role:     pointers.Ptr("roles/run.invoker"),
		})

		_ = cloudschedulerjob.NewCloudSchedulerJob(stack, pointers.Ptr("job_scheduler"), &cloudschedulerjob.CloudSchedulerJobConfig{
			Name:            job.Name(),
			Schedule:        pointers.Ptr(schedule.Cron),
			TimeZone:        pointers.Ptr("Etc/UTC"),
			AttemptDeadline: pointers.Ptr(fmt.Sprintf("%ds", pointers.Deref(schedule.Deadline, 320))),
			Region:          &vars.GCPRegion,
			HttpTarget: &cloudschedulerjob.CloudSchedulerJobHttpTarget{
				HttpMethod: pointers.Ptr(http.MethodPost),
				Uri: pointers.Ptr(fmt.Sprintf("https://%s-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/%s/jobs/%s:run",
					*job.Location(), vars.GCPProjectID, *job.Name())),

				Headers: &map[string]*string{
					"User-Agent": pointers.Ptr("MSP-Google-Cloud-Scheduler"),
				},

				OauthToken: &cloudschedulerjob.CloudSchedulerJobHttpTargetOauthToken{
					ServiceAccountEmail: &invoker.Email,
				},
			},
			DependsOn: &[]cdktf.ITerraformDependable{invokerMember},
		})
	}

	return job, nil
}
