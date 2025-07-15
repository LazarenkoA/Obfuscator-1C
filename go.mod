module github.com/LazarenkoA/Obfuscator-1C

go 1.23.4

toolchain go1.24.1

//replace github.com/LazarenkoA/1c-language-parser => ..\1c-language-parser

require (
	github.com/LazarenkoA/1c-language-parser v0.0.0-20250714065051-68c9915c926e
	github.com/google/uuid v1.6.0
	github.com/knetic/govaluate v3.0.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/vinser/maze v0.2.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
