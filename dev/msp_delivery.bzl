""" MSP Delivery defs"""

def msp_delivery(name, gcp_delivery_pipeline, gcp_project, repository, gcp_region = "us-central1"):
    native.sh_binary(
        name = name,
        srcs = ["//dev/ci:msp_deploy.sh"],
        args = [
            "$(location :candidate_push)",
            "$(location //dev/tools:gcloud)",
        ],
        data = [
            ":candidate_push",
            "//dev/tools:gcloud",
        ],
        env = {
            "GCP_DELIVERY_PIPELINE": gcp_delivery_pipeline,
            "GCP_PROJECT": gcp_project,
            "GCP_REGION": gcp_region,
            "REPOSITORY": repository,
        },
        tags = ["msp-deliverable"],
    )
