module github.com/tonistiigi/vzor/sbox

go 1.12

require (
	github.com/pkg/errors v0.8.1
	gvisor.googlesource.com/gvisor v0.0.0
)

replace gvisor.googlesource.com/gvisor => github.com/tonistiigi/gvisor v0.0.0-20190503211308-a2a954fbc1ea
