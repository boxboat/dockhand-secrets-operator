ARG VERSION=master
ARG GO_VERSION=1.25.0

FROM --platform=${BUILDPLATFORM} cgr.dev/chainguard/go:latest AS build

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

FROM gcr.io/distroless/static AS release

COPY --from=build /etc/passwd /etc/group /etc/
COPY --from=build /src/bin/dockhand-secrets-operator /bin/dockhand-secrets-operator
USER dockhand
ENTRYPOINT ["/bin/dockhand-secrets-operator"]
