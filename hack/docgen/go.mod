module github.com/siderolabs/talos-hack-docgen

go 1.19

// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
// which are then encoded as a valid YAML blocks with proper indentiation
replace gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20210315173758-8fb30b8e5a5b

require (
	github.com/iancoleman/orderedmap v0.2.0
	github.com/invopop/jsonschema v0.7.0
	github.com/santhosh-tekuri/jsonschema/v5 v5.1.1
	github.com/siderolabs/gen v0.4.3
	gopkg.in/yaml.v3 v3.0.1
	mvdan.cc/gofumpt v0.4.0
)

require (
	github.com/google/go-cmp v0.5.8 // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/tools v0.1.12 // indirect
)
