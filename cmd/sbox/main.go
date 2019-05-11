package main

import (
	"flag"
	"strings"

	"github.com/tonistiigi/vzor/runk"
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

	if err := runk.Run(opt); err != nil {
		panic(err)
	}
}
