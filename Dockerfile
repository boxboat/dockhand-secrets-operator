ARG VERSION=develop

FROM golang:1.15-alpine as build
ARG VERSION
RUN apk --no-cache add make && \
    mkdir /build
ADD . /build/
WORKDIR /build

RUN make linux-release

FROM gcr.io/distroless/static
MAINTAINER engineering@boxboat.com
ARG VERSION
COPY --from=build /build/release/linux-amd64/${VERSION}/dockhand-secrets-operator /bin/dockhand-secrets-operator
ENTRYPOINT ["/bin/dockhand-secrets-operator"]
