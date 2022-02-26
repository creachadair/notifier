module github.com/creachadair/notifier

go 1.17

require (
	bitbucket.org/creachadair/shell v0.0.7
	bitbucket.org/creachadair/stringset v0.0.10
	github.com/creachadair/atomicfile v0.2.4
	github.com/creachadair/fileinput v0.1.0
	github.com/creachadair/jrpc2 v0.37.0
	github.com/kr/text v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	golang.org/x/sys v0.0.0-20220224120231-95c6836cb0e7 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

require golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect

replace github.com/creachadair/jrpc2 => ../jrpc2
