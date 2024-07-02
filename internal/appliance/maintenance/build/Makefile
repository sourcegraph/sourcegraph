VERSION=v0.0.$$(( $(shell date +%s) - 1703122377 ))

REGISTRY=us-central1-docker.pkg.dev/sg-infra-release-merge-feb2024/merge-workshop-feb2024

.PHONY: all

all:
	make .version VERSION=${VERSION} REGISTRY=${REGISTRY}
	make build VERSION=${VERSION} REGISTRY=${REGISTRY}
	make pkg VERSION=${VERSION} REGISTRY=${REGISTRY}

build:
	@echo "---------------------------------------"
	@echo "Building ${VERSION}..."
	@echo "---------------------------------------"
	make build.api VERSION=${VERSION} REGISTRY=${REGISTRY}
	make build.ui VERSION=${VERSION} REGISTRY=${REGISTRY}
	@echo "---------------------------------------"
	@echo "Build ${VERSION} complete"
	@echo "---------------------------------------"

pkg:
	@echo "---------------------------------------"
	@echo "Packging ${VERSION}..."
	@echo "---------------------------------------"
	make pkg.api VERSION=${VERSION} REGISTRY=${REGISTRY}
	make pkg.ui VERSION=${VERSION} REGISTRY=${REGISTRY}
	make helm VERSION=${VERSION} REGISTRY=${REGISTRY}
	@echo "---------------------------------------"
	@echo "Package ${VERSION} complete"
	@echo "---------------------------------------"

.version:
	@echo "======================================="
	@echo "Building version ${VERSION}"
	@echo "======================================="

build.ui:
	cd ../frontend && pnpm install
	cd ../frontend && BUILD_NUMBER=${VERSION} pnpm run build

pkg.ui:
	make -C ui VERSION=${VERSION} REGISTRY=${REGISTRY}

build.api:
	cd ../backend && go mod download
	cd ../backend && GOOS=linux GOARCH=amd64 go build -o dist/operator-api ./cmd

pkg.api:
	make -C api VERSION=${VERSION} REGISTRY=${REGISTRY}

helm:
	@echo "Updating Helm to version ${VERSION}"
	cat ../helm/values.yaml \
	  | sed -e "s#registry:.*#registry: ${REGISTRY}#" \
	  | sed -e "s/\(.*image:.*\):[^:]*/\1:${VERSION}/" \
	  > ../helm/values.yaml.tmp
	mv ../helm/values.yaml.tmp ../helm/values.yaml
	cat ../helm/Chart.yaml \
	  | sed -e "s/version:.*/version: ${VERSION}/" \
	  | sed -e "s/appVersion:.*/appVersion: \"${VERSION}\"/" \
	  > ../helm/Chart.yaml.tmp
	mv ../helm/Chart.yaml.tmp ../helm/Chart.yaml
