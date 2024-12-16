ARG VERSION=master
ARG GO_VERSION=1.23.4

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION}-alpine as build

RUN apk --no-cache add make ca-certificates
RUN adduser -D dockhand
WORKDIR /src
COPY go.mod go.sum /src/
RUN go mod download
COPY . /src/
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} VERSION=${VERSION} make build
USER dockhand
ENTRYPOINT ["/src/bin/dockhand-secrets-operator"]

FROM --platform=${TARGETPLATFORM} gcr.io/distroless/static as release

COPY --from=build /etc/passwd /etc/group /etc/
COPY --from=build /src/bin/dockhand-secrets-operator /bin/dockhand-secrets-operator
USER dockhand
ENTRYPOINT ["/bin/dockhand-secrets-operator"]
