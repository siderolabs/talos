module github.com/siderolabs/ukify

go 1.20

replace github.com/siderolabs/talos/pkg/machinery => ../../pkg/machinery

require (
	github.com/foxboron/go-uefi v0.0.0-20230218004016-d1bb9a12f92c
	github.com/google/go-tpm v0.9.0
	github.com/google/go-tpm-tools v0.3.13-0.20230620182252-4639ecce2aba
	github.com/saferwall/pe v1.4.4
	github.com/siderolabs/crypto v0.4.0
	github.com/siderolabs/go-procfs v0.1.1
	github.com/siderolabs/talos v1.4.5
	github.com/siderolabs/talos/pkg/machinery v1.5.0-alpha.0
)

require (
	github.com/containerd/go-cni v1.1.9 // indirect
	github.com/containernetworking/cni v1.1.2 // indirect
	github.com/edsrzf/mmap-go v1.1.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/afero v1.9.3 // indirect
	go.mozilla.org/pkcs7 v0.0.0-20210826202110-33d05740a352 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230530153820-e85fd2cbaebc // indirect
	google.golang.org/grpc v1.56.1 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
)
