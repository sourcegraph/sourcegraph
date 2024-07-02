all:
	cp Dockerfile ../../backend/dist
	docker build  --platform linux/amd64 -t operator-api ../../backend/dist
	docker tag operator-api:latest ${REGISTRY}/operator-api:${VERSION}
	docker push ${REGISTRY}/operator-api:${VERSION}
