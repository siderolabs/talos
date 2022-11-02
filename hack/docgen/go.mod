module github.com/siderolabs/talos-hack-docgen

go 1.19

// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
// which are then encoded as a valid YAML blocks with proper indentiation
replace gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20210315173758-8fb30b8e5a5b

require (
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	mvdan.cc/gofumpt v0.1.1
)

require (
	github.com/google/go-cmp v0.5.4 // indirect
	golang.org/x/mod v0.4.0 // indirect
	golang.org/x/tools v0.0.0-20210101214203-2dba1e4ea05c // indirect
)
