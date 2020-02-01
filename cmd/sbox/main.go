package main

import (
	"flag"
	"strings"

	"github.com/tonistiigi/vzor/runk"
	"gvisor.dev/gvisor/pkg/refs"
)

func main() {
	var (
		opt     runk.Opt
		hostnet bool
		mounts  string
	)

	flag.BoolVar(&hostnet, "hostnet", false, "use host-networking")
	flag.BoolVar(&opt.Process.TTY, "tty", false, "use tty")
	flag.StringVar(&mounts, "mounts", "rootfs", "rootfs-overlay")

	flag.Parse()
	if hostnet {
		opt.Network = runk.NetHost
	}
	opt.Mounts = strings.Split(mounts, ",")

	opt.Process.Args = flag.Args()

	// Sets the reference leak check mode. Also set it in config below to
	// propagate it to child processes.
	refs.SetLeakMode(refs.NoLeakChecking) //TODO

	if err := runk.Run(opt); err != nil {
		panic(err)
	}
}
