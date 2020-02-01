package runk

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/abi/linux"
	"gvisor.dev/gvisor/pkg/context"
	"gvisor.dev/gvisor/pkg/cpuid"
	"gvisor.dev/gvisor/pkg/log"
	"gvisor.dev/gvisor/pkg/memutil"
	"gvisor.dev/gvisor/pkg/rand"
	"gvisor.dev/gvisor/pkg/sentry/fs"
	"gvisor.dev/gvisor/pkg/sentry/fs/host"
	"gvisor.dev/gvisor/pkg/sentry/fs/ramfs"
	"gvisor.dev/gvisor/pkg/sentry/kernel"
	"gvisor.dev/gvisor/pkg/sentry/kernel/auth"
	"gvisor.dev/gvisor/pkg/sentry/limits"
	"gvisor.dev/gvisor/pkg/sentry/loader"
	"gvisor.dev/gvisor/pkg/sentry/pgalloc"
	"gvisor.dev/gvisor/pkg/sentry/socket/hostinet"
	slinux "gvisor.dev/gvisor/pkg/sentry/syscalls/linux"
	"gvisor.dev/gvisor/pkg/sentry/time"
	"gvisor.dev/gvisor/pkg/sentry/usage"

	_ "gvisor.dev/gvisor/pkg/sentry/fs/dev"
	_ "gvisor.dev/gvisor/pkg/sentry/fs/proc"
	_ "gvisor.dev/gvisor/pkg/sentry/fs/sys"
	_ "gvisor.dev/gvisor/pkg/sentry/fs/tmpfs"
	_ "gvisor.dev/gvisor/pkg/sentry/fs/tty"
)

type ProcessOpt struct {
	TTY            bool
	Args           []string
	Env            []string
	Stdout, Stderr io.Writer
	Stdin          io.Reader
}

type GVisorPlatform string

const (
	KVM    GVisorPlatform = "kvm"
	Ptrace                = "ptrace"
)

type GVisorOpt struct {
	Platform GVisorPlatform
}

type Network int

const (
	NetNone Network = iota
	NetHost
)

type Opt struct {
	Process ProcessOpt
	Mounts  []string
	Network Network
	GVisor  GVisorOpt
}

func Run(o Opt) error {
	log.SetLevel(log.Warning)

	// Register the global syscall table.
	kernel.RegisterSyscallTable(slinux.AMD64)

	// We initialize the rand package now to make sure /dev/urandom is pre-opened
	// on kernels that do not support getrandom(2).
	if err := rand.Init(); err != nil {
		return errors.Wrap(err, "setting up rand")
	}

	if err := usage.Init(); err != nil {
		return errors.Wrap(err, "error setting up memory usage")
	}

	p, err := newPlatform(o.GVisor.Platform)
	if err != nil {
		return err
	}

	k := &kernel.Kernel{
		Platform: p,
	}

	// Create memory file.
	mf, err := createMemoryFile()
	if err != nil {
		return errors.Wrap(err, "creating memory file")
	}
	k.SetMemoryFile(mf)

	vdso, err := loader.PrepareVDSO(nil, k)
	if err != nil {
		return errors.Wrap(err, "error creating vdso")
	}

	tk, err := kernel.NewTimekeeper(k, vdso.ParamPage.FileRange())
	if err != nil {
		return errors.Wrap(err, "error creating timekeeper")
	}
	tk.SetClocks(time.NewCalibratedClocks())

	networkStack, err := netStack(k, k, o.Network)
	if err != nil {
		return err
	}

	stack, ok := networkStack.(*hostinet.Stack)
	if ok {
		if err := stack.Configure(); err != nil {
			return err
		}
	}

	creds := auth.NewUserCredentials(
		auth.KUID(0),
		auth.KGID(0),
		nil,
		nil,
		auth.NewRootUserNamespace())

	if err = k.Init(kernel.InitKernelArgs{
		FeatureSet:                  cpuid.HostFeatureSet(),
		Timekeeper:                  tk,
		RootUserNamespace:           creds.UserNamespace,
		NetworkStack:                networkStack,
		ApplicationCores:            uint(runtime.NumCPU()),
		Vdso:                        vdso,
		RootUTSNamespace:            kernel.NewUTSNamespace("sbox", "sbox", creds.UserNamespace),
		RootIPCNamespace:            kernel.NewIPCNamespace(creds.UserNamespace),
		RootAbstractSocketNamespace: kernel.NewAbstractSocketNamespace(),
		PIDNamespace:                kernel.NewRootPIDNamespace(creds.UserNamespace),
	}); err != nil {
		return errors.Wrap(err, "error initializing kernel")
	}

	ls, err := limits.NewLinuxLimitSet()
	if err != nil {
		return err
	}

	// Create the process arguments.
	procArgs := kernel.CreateProcessArgs{
		Argv:                    o.Process.Args,
		Envv:                    []string{},
		WorkingDirectory:        "/", // Defaults to '/' if empty.
		Credentials:             creds,
		Umask:                   0022,
		Limits:                  ls,
		MaxSymlinkTraversals:    linux.MaxSymlinkTraversals,
		UTSNamespace:            k.RootUTSNamespace(),
		IPCNamespace:            k.RootIPCNamespace(),
		AbstractSocketNamespace: k.RootAbstractSocketNamespace(),
		ContainerID:             "sbox",
		PIDNamespace:            k.RootPIDNamespace(),
	}
	ctx := procArgs.NewContext(k)

	fdt, err := createFDTable(ctx, k, ls, o.Process.TTY, []int{0, 1, 2})
	if err != nil {
		return errors.Wrap(err, "error importing fds")
	}
	// CreateProcess takes a reference on fdTable if successful. We
	// won't need ours either way.
	procArgs.FDTable = fdt

	rootProcArgs := procArgs
	rootProcArgs.WorkingDirectory = "/"
	rootProcArgs.Credentials = auth.NewRootCredentials(creds.UserNamespace)
	rootProcArgs.Umask = 0022
	rootProcArgs.MaxSymlinkTraversals = linux.MaxSymlinkTraversals

	rootCtx := rootProcArgs.NewContext(k)

	followLinks := uint(linux.MaxSymlinkTraversals)
	mns, err := createMountNamespace(ctx, rootCtx, o.Mounts, &followLinks)
	if err != nil {
		return errors.Wrap(err, "error creating mounts")
	}
	rootProcArgs.MountNamespace = mns

	_, _, err = k.CreateProcess(rootProcArgs)
	if err != nil {
		return errors.Wrap(err, "failed to create init process")
	}

	tg := k.GlobalInit()
	if o.Process.TTY {
		ttyFile, _ := procArgs.FDTable.Get(0)
		defer ttyFile.DecRef()
		ttyfop := ttyFile.FileOperations.(*host.TTYFileOperations)
		// Set the foreground process group on the TTY to the global
		// init process group, since that is what we are about to
		// start running.
		ttyfop.InitForegroundProcessGroup(tg.ProcessGroup())
	}

	if err := k.Start(); err != nil {
		return err
	}

	k.WaitExited()

	return nil
}

func addSubmountOverlay(ctx context.Context, inode *fs.Inode, submounts []string) (*fs.Inode, error) {
	// There is no real filesystem backing this ramfs tree, so we pass in
	// "nil" here.
	msrc := fs.NewNonCachingMountSource(ctx, nil, fs.MountSourceFlags{})
	mountTree, err := ramfs.MakeDirectoryTree(ctx, msrc, submounts)
	if err != nil {
		return nil, errors.Wrap(err, "error creating mount tree")
	}
	overlayInode, err := fs.NewOverlayRoot(ctx, inode, mountTree, fs.MountSourceFlags{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to make mount overlay")
	}
	return overlayInode, err
}

func createMountNamespace(userCtx context.Context, rootCtx context.Context, mounts []string, maxTraversals *uint) (*fs.MountNamespace, error) {
	rootInode, err := createRootMount(rootCtx, mounts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create root mount")
	}

	mns, err := fs.NewMountNamespace(userCtx, rootInode)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create root mount namespace")
	}

	root := mns.Root()
	defer root.DecRef()

	proc, ok := fs.FindFilesystem("proc")
	if !ok {
		panic(fmt.Sprintf("could not find filesystem proc"))
	}
	ctx := rootCtx
	inode, err := proc.Mount(ctx, "none", fs.MountSourceFlags{}, "", nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create mount with source")
	}

	dirent, err := mns.FindInode(ctx, root, root, "/proc", maxTraversals)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find mount destination")
	}
	defer dirent.DecRef()
	if err := mns.Mount(ctx, dirent, inode); err != nil {
		return nil, errors.Wrap(err, "failed to mount at destination")
	}

	return mns, nil
}

func createRootMount(ctx context.Context, mounts []string) (*fs.Inode, error) {
	// First construct the filesystem from the spec.Root.
	mf := fs.MountSourceFlags{ReadOnly: false}

	var (
		rootInode, prevInode *fs.Inode
		err                  error
	)

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	host, ok := fs.FindFilesystem("whitelistfs")
	if !ok {
		panic(fmt.Sprintf("could not find filesystem host"))
	}
	for i, m := range mounts {
		if !filepath.IsAbs(m) {
			m = filepath.Join(wd, m)
		}
		rootInode, err = host.Mount(ctx, "", mf, "root="+m, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate root mount point")
		}
		if i != 0 {
			rootInode, err = fs.NewOverlayRoot(ctx, rootInode, prevInode, fs.MountSourceFlags{})
			if err != nil {
				return nil, errors.Wrap(err, "failed to make mount overlay")
			}
		}
		prevInode = rootInode
	}

	submounts := []string{"/dev", "/sys", "/proc", "/tmp"}
	rootInode, err = addSubmountOverlay(ctx, rootInode, submounts)
	if err != nil {
		return nil, errors.Wrap(err, "error adding submount overlay")
	}

	tmpfs, ok := fs.FindFilesystem("tmpfs")
	if !ok {
		panic(fmt.Sprintf("could not find filesystem tmpfs"))
	}

	upper, err := tmpfs.Mount(ctx, "upper", fs.MountSourceFlags{}, "", nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create tmpfs overlay")
	}
	rootInode, err = fs.NewOverlayRoot(ctx, upper, rootInode, fs.MountSourceFlags{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to make mount overlay")
	}

	return rootInode, nil
}

func createFDTable(ctx context.Context, k *kernel.Kernel, l *limits.LimitSet, console bool, stdioFDs []int) (*kernel.FDTable, error) {
	if len(stdioFDs) != 3 {
		return nil, errors.Errorf("stdioFDs should contain exactly 3 FDs (stdin, stdout, and stderr), but %d FDs received", len(stdioFDs))
	}

	fdm := k.NewFDTable()
	defer fdm.DecRef()
	mounter := fs.FileOwnerFromContext(ctx)

	// Maps sandbox FD to host FD.
	fdMap := map[int]int{
		0: stdioFDs[0],
		1: stdioFDs[1],
		2: stdioFDs[2],
	}

	var ttyFile *fs.File
	for appFD, hostFD := range fdMap {
		var appFile *fs.File

		if console && appFD < 3 {
			// Import the file as a host TTY file.
			if ttyFile == nil {
				var err error
				appFile, err = host.ImportFile(ctx, hostFD, mounter, true /* isTTY */)
				if err != nil {
					return nil, err
				}
				defer appFile.DecRef()

				// Remember this in the TTY file, as we will
				// use it for the other stdio FDs.
				ttyFile = appFile
			} else {
				// Re-use the existing TTY file, as all three
				// stdio FDs must point to the same fs.File in
				// order to share TTY state, specifically the
				// foreground process group id.
				appFile = ttyFile
			}
		} else {
			// Import the file as a regular host file.
			var err error
			appFile, err = host.ImportFile(ctx, hostFD, mounter, false /* isTTY */)
			if err != nil {
				return nil, err
			}
			defer appFile.DecRef()
		}

		// Add the file to the FD map.
		if err := fdm.NewFDAt(ctx, int32(appFD), appFile, kernel.FDFlags{}); err != nil {
			return nil, err
		}
	}

	fdm.IncRef()
	return fdm, nil
}

func createMemoryFile() (*pgalloc.MemoryFile, error) {
	const memfileName = "runsc-memory"
	memfd, err := memutil.CreateMemFD(memfileName, 0)
	if err != nil {
		return nil, errors.Wrap(err, "error creating memfd")
	}
	memfile := os.NewFile(uintptr(memfd), memfileName)
	mf, err := pgalloc.NewMemoryFile(memfile, pgalloc.MemoryFileOpts{})
	if err != nil {
		memfile.Close()
		return nil, errors.Wrap(err, "error creating pgalloc.MemoryFile")
	}
	return mf, nil
}
