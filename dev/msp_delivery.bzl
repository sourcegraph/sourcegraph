""" MSP Delivery defs"""

def msp_delivery(name, msp_service_id, gcp_project, repository, gcp_region = "us-central1"):
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
            "MSP_SERVICE_ID": msp_service_id,
            "GCP_PROJECT": gcp_project,
            "GCP_REGION": gcp_region,
            "REPOSITORY": repository,
        },
        tags = ["msp-deliverable"],
    )
