all:
	cp Dockerfile operator-ui.conf.template ../../frontend/dist
	docker build  --platform linux/amd64 -t operator-ui ../../frontend/dist
	docker tag operator-ui:latest ${REGISTRY}/operator-ui:${VERSION}
	docker push ${REGISTRY}/operator-ui:${VERSION}
