FROM golang:1.21-alpine3.18@sha256:2c10297a06e630166c6de3c3571590f553332b8aa2be74aa9b850c928cbcc743 AS builder

WORKDIR /go/src/tracking-issue
COPY . .
RUN go mod init tracking-issue
RUN go get ./...
RUN CGO_ENABLED=0 go install .

FROM sourcegraph/alpine-3.14:213466_2023-04-17_5.0-bdda34a71619@sha256:6354a4ff578b685e36c8fbde81f62125ae0011b047fb2cc22d1b0de616b3c59a
COPY --from=builder /go/bin/* /usr/local/bin/
ENTRYPOINT ["tracking-issue"]
