module github.com/dosco/graphjin/conf/v3

go 1.18

require (
	github.com/dosco/graphjin/core/v3 v3.0.32
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	golang.org/x/sync v0.7.0 // indirect
)

replace github.com/dosco/graphjin/core/v3 => ../core
