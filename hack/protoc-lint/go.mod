module github.com/talos-systems/talos-hack-protoc-lint

go 1.17

replace github.com/talos-systems/talos/pkg/machinery => ../../pkg/machinery

require (
	github.com/stretchr/testify v1.7.0
	github.com/talos-systems/talos/pkg/machinery v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.41.0
	google.golang.org/protobuf v1.27.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.0.0-20211020060615-d418f374d309 // indirect
	golang.org/x/sys v0.0.0-20211025201205-69cdffdb9359 // indirect
	golang.org/x/text v0.3.6 // indirect
	google.golang.org/genproto v0.0.0-20211029142109-e255c875f7c7 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)
