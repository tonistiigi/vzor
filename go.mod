module github.com/tonistiigi/vzor

go 1.12

require (
	github.com/pkg/errors v0.8.1
	golang.org/x/sync v0.0.0-20190423024810-112230192c58 // indirect
	golang.org/x/sys v0.0.0-20190502175342-a43fa875dd82 // indirect
	gvisor.googlesource.com/gvisor v0.0.0
)

replace gvisor.googlesource.com/gvisor => github.com/tonistiigi/gvisor v0.0.0-20190503211308-a2a954fbc1ea
