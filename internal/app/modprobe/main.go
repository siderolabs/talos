// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package modprobe implements the kernel modprobe usermode helper: it loads a kernel module on demand.
package modprobe

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/pmorjan/kmod"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

const (
	// pfKthread is the PF_KTHREAD task flag (include/linux/sched.h).
	pfKthread = 0x00200000

	// parametersTimeout bounds the time spent fetching the module parameters from machined,
	// which bounds the time the kernel caller is blocked on top of the module load itself.
	parametersTimeout = 30 * time.Second

	connectRetryInterval = 500 * time.Millisecond
)

// ErrNotAModule is returned when the requested name is not a module shipped with the running kernel.
//
// This is the expected outcome for subsystem aliases (e.g. 'fs-xfs', 'net-pf-10', 'crypto-aes'), which are
// what userspace can trigger via mounts, sockets, etc.: Talos never loads modules via aliases.
var ErrNotAModule = errors.New("not a module name")

// moduleNameRe matches a plain module name.
var moduleNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Main is the entrypoint into /sbin/modprobe as invoked by the kernel usermode helper (request_module()).
//
// The kernel executes the static usermode helper (machined) as `/sbin/modprobe -q -- <module>` and waits
// for the process to exit, so the module is loaded by the time Main returns.
//
// The helper runs as root in the initial namespaces, so it loads the module itself: the module name is
// looked up without resolving aliases (see LookupModule), and the parameters come from the machine config
// (KernelModuleSpec resources read via the machined API) if the module is listed there.
//
// Like `modprobe -q`, a request for a name which is not a module exits with an error silently.
func Main(args []string) {
	err := run(args)
	if err == nil {
		return
	}

	if errors.Is(err, ErrNotAModule) && QuietFromArgs(args) {
		os.Exit(1)
	}

	log.Fatal(err)
}

func run(args []string) error {
	name, err := checkRequest(args)
	if err != nil {
		return err
	}

	lookup, err := lookupRequested(name)
	if err != nil {
		return err
	}

	if lookup == LookupBuiltin {
		// nothing to load
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), parametersTimeout)
	defer cancel()

	parameters, err := fetchParameters(ctx, name)
	if err != nil {
		return fmt.Errorf("module %q was not loaded: %w", name, err)
	}

	if err = loadModule(name, parameters); err != nil {
		return fmt.Errorf("module %q was not loaded: %w", name, err)
	}

	log.Printf("module %q loaded (parameters %q)", name, parameters)

	return nil
}

// checkRequest extracts the module name from the arguments and verifies the request comes from the kernel.
func checkRequest(args []string) (string, error) {
	name, ok := ModuleNameFromArgs(args)
	if !ok {
		return "", fmt.Errorf("no module name in the arguments %q", args)
	}

	if !ValidModuleName(name) {
		return "", fmt.Errorf("refusing to load %q: invalid module name", name)
	}

	if err := VerifyKernelParent(os.Getppid(), "/proc"); err != nil {
		return "", fmt.Errorf("refusing to load %q: %w", name, err)
	}

	return name, nil
}

// lookupRequested looks the requested module up in the modules directory of the running kernel.
func lookupRequested(name string) (Lookup, error) {
	modulesDir := filepath.Join(constants.LibModulesPath, constants.DefaultKernelVersion)

	lookup, err := LookupModule(modulesDir, name)
	if err != nil {
		return LookupNotFound, fmt.Errorf("error looking up %q: %w", name, err)
	}

	if lookup == LookupNotFound {
		return LookupNotFound, fmt.Errorf("refusing to load %q: %w", name, ErrNotAModule)
	}

	return lookup, nil
}

// ModuleNameFromArgs extracts the module name from the modprobe arguments.
//
// The kernel invokes `modprobe -q -- <module>`: options (arguments starting with '-') before '--' are skipped,
// the first argument after '--' or the first non-option argument is the module name.
func ModuleNameFromArgs(args []string) (string, bool) {
	for i := 1; i < len(args); i++ {
		switch {
		case args[i] == "--":
			if i+1 < len(args) && args[i+1] != "" {
				return args[i+1], true
			}

			return "", false
		case strings.HasPrefix(args[i], "-"):
			continue
		case args[i] != "":
			return args[i], true
		}
	}

	return "", false
}

// QuietFromArgs reports whether the quiet option ('-q') is present among the modprobe options.
func QuietFromArgs(args []string) bool {
	for i := 1; i < len(args); i++ {
		if args[i] == "--" {
			return false
		}

		if args[i] == "-q" || args[i] == "--quiet" {
			return true
		}
	}

	return false
}

// ValidModuleName checks that the name is syntactically a kernel module name.
func ValidModuleName(name string) bool {
	return moduleNameRe.MatchString(name)
}

// VerifyKernelParent checks that the process with the given PID is a kernel thread.
//
// The kernel spawns the usermode helper from a kernel thread (kthreadd or a kworker), while any process
// started from userspace has a userspace parent (orphans are reparented to init or a subreaper, never
// to a kernel thread), so a kernel thread as the parent proves the request was issued by the kernel.
func VerifyKernelParent(ppid int, procfsPath string) error {
	if ppid <= 1 {
		return fmt.Errorf("parent PID %d is not a kernel thread", ppid)
	}

	stat, err := os.ReadFile(filepath.Join(procfsPath, strconv.Itoa(ppid), "stat"))
	if err != nil {
		return fmt.Errorf("error reading parent process status: %w", err)
	}

	flags, err := statFlags(stat)
	if err != nil {
		return fmt.Errorf("error parsing parent process status: %w", err)
	}

	if flags&pfKthread == 0 {
		return fmt.Errorf("parent PID %d is not a kernel thread (flags %#x)", ppid, flags)
	}

	return nil
}

// statFlags parses the task flags (the 9th field) out of /proc/<pid>/stat contents.
func statFlags(stat []byte) (uint64, error) {
	// the command name (the 2nd field) is enclosed in parentheses and might contain spaces,
	// so the fields are parsed starting from the last ')'
	_, rest, found := bytes.CutLast(stat, []byte(")"))
	if !found {
		return 0, errors.New("malformed stat contents")
	}

	fields := strings.Fields(string(rest))

	// fields[0] is the state (the 3rd field)
	const flagsIdx = 9 - 3

	if len(fields) <= flagsIdx {
		return 0, errors.New("not enough fields in stat contents")
	}

	return strconv.ParseUint(fields[flagsIdx], 10, 64)
}

// Lookup is the outcome of looking up a module name in the modules directory.
type Lookup int

const (
	// LookupNotFound means there's no such module (the name might be an alias, which is not resolved).
	LookupNotFound Lookup = iota
	// LookupBuiltin means the module is built into the kernel.
	LookupBuiltin
	// LookupLoadable means the module is available to be loaded.
	LookupLoadable
)

// LookupModule looks the module name up in modules.builtin and modules.dep of the modules directory.
//
// Aliases (modules.alias) are deliberately not consulted: kernel code requesting another module
// (e.g. a driver loading a helper module while probing a device) uses the module name, while subsystem
// aliases (net-pf-*, fs-*, crypto-*, binfmt-*, ...) are what userspace can trigger via sockets, mounts, etc.
func LookupModule(modulesDir, name string) (Lookup, error) {
	name = NormalizeModuleName(name)

	found, err := scanModulesFile(filepath.Join(modulesDir, "modules.builtin"), name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return LookupNotFound, err
	}

	if found {
		return LookupBuiltin, nil
	}

	found, err = scanModulesFile(filepath.Join(modulesDir, "modules.dep"), name)
	if err != nil {
		return LookupNotFound, err
	}

	if found {
		return LookupLoadable, nil
	}

	return LookupNotFound, nil
}

// scanModulesFile reports whether the file lists a module with the given (normalized) name.
//
// Both modules.builtin ('kernel/fs/xfs/xfs.ko') and modules.dep ('kernel/fs/xfs/xfs.ko: <deps>') start
// each line with the module path.
func scanModulesFile(filePath, name string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}

	defer f.Close() //nolint:errcheck

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		modulePath, _, _ := strings.Cut(scanner.Text(), ":")

		if fields := strings.Fields(modulePath); len(fields) > 0 && moduleNameFromPath(fields[0]) == name {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// moduleNameFromPath converts a module path ('kernel/drivers/md/dm-thin-pool.ko') to the normalized module name ('dm_thin_pool').
func moduleNameFromPath(modulePath string) string {
	base := path.Base(modulePath)

	if idx := strings.Index(base, ".ko"); idx != -1 {
		base = base[:idx]
	}

	return NormalizeModuleName(base)
}

// NormalizeModuleName converts the module name to the form used by the kernel ('-' and '_' are interchangeable).
func NormalizeModuleName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// ParametersFor returns the parameters of the module from the machine config (KernelModuleSpec resources), if listed.
//
// Module names are matched in the normalized form, so the config might list the module as 'dm-thin-pool'
// while the kernel requests 'dm_thin_pool'.
func ParametersFor(specs []*runtime.KernelModuleSpec, name string) []string {
	name = NormalizeModuleName(name)

	for _, spec := range specs {
		if NormalizeModuleName(spec.TypedSpec().Name) == name {
			return spec.TypedSpec().Parameters
		}
	}

	return nil
}

// fetchParameters reads the module parameters from the machine config via the machined API.
func fetchParameters(ctx context.Context, name string) ([]string, error) {
	md := metadata.Pairs()
	authz.SetMetadata(md, role.MakeSet(role.Operator))
	ctx = metadata.NewOutgoingContext(ctx, md)

	c, err := client.New(
		ctx,
		client.WithUnixSocket(constants.MachineSocketPath),
		client.WithGRPCDialOptions(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating machinery client: %w", err)
	}

	defer c.Close() //nolint:errcheck

	var specs safe.List[*runtime.KernelModuleSpec]

	if err = retryUnavailable(ctx, func() error {
		specs, err = safe.StateListAll[*runtime.KernelModuleSpec](ctx, c.COSI)

		return err
	}); err != nil {
		return nil, fmt.Errorf("error listing kernel module specs: %w", err)
	}

	return ParametersFor(slices.Collect(specs.All()), name), nil
}

// loadModule loads the module (and its dependencies) with the parameters.
func loadModule(name string, parameters []string) error {
	// aliases are not resolved (see LookupModule); the status in /proc/modules is ignored so that
	// a module being loaded concurrently (by another helper or by machined) is waited for by the kernel
	// rather than considered loaded while its initialization is still in progress
	manager, err := kmod.New(kmod.SetIgnoreAlias(), kmod.SetIgnoreStatus())
	if err != nil {
		return fmt.Errorf("error initializing kmod manager: %w", err)
	}

	return manager.Load(name, strings.Join(parameters, " "), 0)
}

// retryUnavailable retries the function while machined is unavailable (e.g. still starting up).
func retryUnavailable(ctx context.Context, f func() error) error {
	for {
		err := f()
		if err == nil || status.Code(err) != codes.Unavailable {
			return err
		}

		log.Printf("machined is not available, retrying: %s", err)

		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %w", ctx.Err(), err)
		case <-time.After(connectRetryInterval):
		}
	}
}
