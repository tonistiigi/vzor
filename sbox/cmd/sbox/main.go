package main

import (
	"flag"

	"github.com/tonistiigi/sbox"
)

func main() {
	var opt sbox.Opt

	flag.BoolVar(&opt.HostNet, "hostnet", false, "use host-networking")
	flag.BoolVar(&opt.TTY, "tty", false, "use tty")
	flag.StringVar(&opt.Mounts, "mounts", "rootfs", "rootfs-overlay")

	flag.Parse()

	opt.Args = flag.Args()

	if err := sbox.Run(opt); err != nil {
		panic(err)
	}
}
