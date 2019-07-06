# The osctl targets build the osctl binaries.

FROM base AS osctl-linux-amd64-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/cmd/osctl
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -a -ldflags "-s -w -linkmode external -extldflags \"-static\" -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osctl-linux-amd64
RUN chmod +x /osctl-linux-amd64

FROM scratch AS osctl-linux-amd64
COPY --from=osctl-linux-amd64-build /osctl-linux-amd64 /osctl-linux-amd64

FROM base AS osctl-darwin-amd64-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/cmd/osctl
RUN GOOS=darwin GOARCH=amd64 go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osctl-darwin-amd64
RUN chmod +x /osctl-darwin-amd64

FROM scratch AS osctl-darwin-amd64
COPY --from=osctl-darwin-amd64-build /osctl-darwin-amd64 /osctl-darwin-amd64
