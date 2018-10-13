package services

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/system/conditions"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/system/runner"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/system/runner/containerd"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// CRT implements the Service interface. It serves as the concrete type with the
// required methods.
type CRT struct{}

const crioPolicy = `{
    "default": [
        {
            "type": "insecureAcceptAnything"
        }
    ]
}
`

// ID implements the Service interface.
func (c *CRT) ID(data *userdata.UserData) string {
	switch data.Services.Kubeadm.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		return "docker"
	case constants.ContainerRuntimeCRIO:
		return "crio"
	default:
		return "unknown"
	}
}

// PreFunc implements the Service interface.
func (c *CRT) PreFunc(data *userdata.UserData) error {
	switch data.Services.Kubeadm.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		if err := os.MkdirAll("/var/lib/docker", os.ModeDir); err != nil {
			return fmt.Errorf("failed to create directory /var/lib/docker: %v", err)
		}
	case constants.ContainerRuntimeCRIO:
		if err := os.MkdirAll("/var/run/crio", os.ModeDir); err != nil {
			return fmt.Errorf("failed to create directory /var/run/crio: %v", err)
		}
		if err := os.MkdirAll("/var/lib/containers", os.ModeDir); err != nil {
			return fmt.Errorf("failed to create directory /var/lib/containers: %v", err)
		}
		if err := os.MkdirAll("/var/etc/crio", os.ModeDir); err != nil {
			return fmt.Errorf("failed to create directory /var/etc/crio: %v", err)
		}
		if err := os.MkdirAll("/var/etc/containers", os.ModeDir); err != nil {
			return fmt.Errorf("failed to create directory /var/etc/containers: %v", err)
		}
		if err := ioutil.WriteFile("/var/etc/containers/policy.json", []byte(crioPolicy), 0644); err != nil {
			return fmt.Errorf("failed to write policy.json: %v", err)
		}
		if err := ioutil.WriteFile("/var/etc/crio/seccomp.json", []byte(seccompProfile), 0644); err != nil {
			return fmt.Errorf("failed to write seccomp.json: %v", err)
		}
	}

	return nil
}

// PostFunc implements the Service interface.
func (c *CRT) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// ConditionFunc implements the Service interface.
func (c *CRT) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	return conditions.None()
}

// Start implements the Service interface. nolint: dupl
func (c *CRT) Start(data *userdata.UserData) error {
	// Set the image.
	var (
		image  string
		args   runner.Args
		mounts = []specs.Mount{
			{Type: "cgroup", Destination: "/sys/fs/cgroup", Options: []string{"rbind", "rshared", "rw"}},
			{Type: "bind", Destination: "/etc/kubernetes", Source: "/var/etc/kubernetes", Options: []string{"bind", "rw"}},
			{Type: "bind", Destination: "/etc/cni", Source: "/var/etc/cni", Options: []string{"bind", "rw"}},
			{Type: "bind", Destination: "/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
			{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		}
		env = []string{}
	)

	switch data.Services.Kubeadm.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		image = constants.DockerImage
		args = runner.Args{
			ID: c.ID(data),
			ProcessArgs: []string{"dockerd",
				"--host=unix://" + constants.ContainerRuntimeDockerSocket,
				"--live-restore",
				"--iptables=false",
				"--ip-masq=false",
				"--storage-driver=overlay2",
				"--selinux-enabled=false",
				"--exec-opt=native.cgroupdriver=cgroupfs",
				"--log-opt=max-size=10m",
				"--log-opt=max-file=3",
			},
		}
		dockerMounts := []specs.Mount{
			{Type: "bind", Destination: "/var/lib/docker", Source: "/var/lib/docker", Options: []string{"rbind", "rshared", "rw"}},
		}
		mounts = append(mounts, dockerMounts...)
		env = []string{"DOCKER_NOFILE=1000000"}
	case constants.ContainerRuntimeCRIO:
		// TODO(andrewrynhard): We should use
		// registry.centos.org/projectatomic/cri-o:latest, but a 403 is returned
		// with attempting to pull the image.
		image = constants.CRIOImage
		args = runner.Args{
			ID: c.ID(data),
			ProcessArgs: []string{
				"crio",
				"--conmon=/usr/libexec/crio/conmon",
				"--storage-driver=overlay",
				"--seccomp-profile=/etc/crio/seccomp.json",
				"--registry=docker.io",
			},
		}
		crioMounts := []specs.Mount{
			{Type: "bind", Destination: "/etc/crio", Source: "/var/etc/crio", Options: []string{"bind", "rw"}},
			{Type: "bind", Destination: "/etc/containers", Source: "/var/etc/containers", Options: []string{"bind", "rw"}},
			{Type: "bind", Destination: "/var/lib/containers", Source: "/var/lib/containers", Options: []string{"rbind", "rshared", "rw"}},
		}
		mounts = append(mounts, crioMounts...)
	default:
		return fmt.Errorf("unknown container runtime %q", data.Services.Kubeadm.ContainerRuntime)
	}

	if data.Services.CRT != nil && data.Services.CRT.Image != "" {
		image = data.Services.CRT.Image
	}

	r := containerd.Containerd{}

	return r.Run(
		data,
		args,
		runner.WithContainerImage(image),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*2048)),
			containerd.WithRootfsPropagation("slave"),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithPrivileged,
		),
		runner.WithType(runner.Forever),
		runner.WithEnv(env),
	)
}

const seccompProfile = `{
	"defaultAction": "SCMP_ACT_ERRNO",
	"archMap": [
		{
			"architecture": "SCMP_ARCH_X86_64",
			"subArchitectures": [
				"SCMP_ARCH_X86",
				"SCMP_ARCH_X32"
			]
		},
		{
			"architecture": "SCMP_ARCH_AARCH64",
			"subArchitectures": [
				"SCMP_ARCH_ARM"
			]
		},
		{
			"architecture": "SCMP_ARCH_MIPS64",
			"subArchitectures": [
				"SCMP_ARCH_MIPS",
				"SCMP_ARCH_MIPS64N32"
			]
		},
		{
			"architecture": "SCMP_ARCH_MIPS64N32",
			"subArchitectures": [
				"SCMP_ARCH_MIPS",
				"SCMP_ARCH_MIPS64"
			]
		},
		{
			"architecture": "SCMP_ARCH_MIPSEL64",
			"subArchitectures": [
				"SCMP_ARCH_MIPSEL",
				"SCMP_ARCH_MIPSEL64N32"
			]
		},
		{
			"architecture": "SCMP_ARCH_MIPSEL64N32",
			"subArchitectures": [
				"SCMP_ARCH_MIPSEL",
				"SCMP_ARCH_MIPSEL64"
			]
		},
		{
			"architecture": "SCMP_ARCH_S390X",
			"subArchitectures": [
				"SCMP_ARCH_S390"
			]
		}
	],
	"syscalls": [
		{
			"names": [
				"accept",
				"accept4",
				"access",
				"adjtimex",
				"alarm",
				"bind",
				"brk",
				"capget",
				"capset",
				"chdir",
				"chmod",
				"chown",
				"chown32",
				"clock_getres",
				"clock_gettime",
				"clock_nanosleep",
				"close",
				"connect",
				"copy_file_range",
				"creat",
				"dup",
				"dup2",
				"dup3",
				"epoll_create",
				"epoll_create1",
				"epoll_ctl",
				"epoll_ctl_old",
				"epoll_pwait",
				"epoll_wait",
				"epoll_wait_old",
				"eventfd",
				"eventfd2",
				"execve",
				"execveat",
				"exit",
				"exit_group",
				"faccessat",
				"fadvise64",
				"fadvise64_64",
				"fallocate",
				"fanotify_mark",
				"fchdir",
				"fchmod",
				"fchmodat",
				"fchown",
				"fchown32",
				"fchownat",
				"fcntl",
				"fcntl64",
				"fdatasync",
				"fgetxattr",
				"flistxattr",
				"flock",
				"fork",
				"fremovexattr",
				"fsetxattr",
				"fstat",
				"fstat64",
				"fstatat64",
				"fstatfs",
				"fstatfs64",
				"fsync",
				"ftruncate",
				"ftruncate64",
				"futex",
				"futimesat",
				"getcpu",
				"getcwd",
				"getdents",
				"getdents64",
				"getegid",
				"getegid32",
				"geteuid",
				"geteuid32",
				"getgid",
				"getgid32",
				"getgroups",
				"getgroups32",
				"getitimer",
				"getpeername",
				"getpgid",
				"getpgrp",
				"getpid",
				"getppid",
				"getpriority",
				"getrandom",
				"getresgid",
				"getresgid32",
				"getresuid",
				"getresuid32",
				"getrlimit",
				"get_robust_list",
				"getrusage",
				"getsid",
				"getsockname",
				"getsockopt",
				"get_thread_area",
				"gettid",
				"gettimeofday",
				"getuid",
				"getuid32",
				"getxattr",
				"inotify_add_watch",
				"inotify_init",
				"inotify_init1",
				"inotify_rm_watch",
				"io_cancel",
				"ioctl",
				"io_destroy",
				"io_getevents",
				"ioprio_get",
				"ioprio_set",
				"io_setup",
				"io_submit",
				"ipc",
				"kill",
				"lchown",
				"lchown32",
				"lgetxattr",
				"link",
				"linkat",
				"listen",
				"listxattr",
				"llistxattr",
				"_llseek",
				"lremovexattr",
				"lseek",
				"lsetxattr",
				"lstat",
				"lstat64",
				"madvise",
				"memfd_create",
				"mincore",
				"mkdir",
				"mkdirat",
				"mknod",
				"mknodat",
				"mlock",
				"mlock2",
				"mlockall",
				"mmap",
				"mmap2",
				"mprotect",
				"mq_getsetattr",
				"mq_notify",
				"mq_open",
				"mq_timedreceive",
				"mq_timedsend",
				"mq_unlink",
				"mremap",
				"msgctl",
				"msgget",
				"msgrcv",
				"msgsnd",
				"msync",
				"munlock",
				"munlockall",
				"munmap",
				"nanosleep",
				"newfstatat",
				"_newselect",
				"open",
				"openat",
				"pause",
				"pipe",
				"pipe2",
				"poll",
				"ppoll",
				"prctl",
				"pread64",
				"preadv",
				"preadv2",
				"prlimit64",
				"pselect6",
				"pwrite64",
				"pwritev",
				"pwritev2",
				"read",
				"readahead",
				"readlink",
				"readlinkat",
				"readv",
				"recv",
				"recvfrom",
				"recvmmsg",
				"recvmsg",
				"remap_file_pages",
				"removexattr",
				"rename",
				"renameat",
				"renameat2",
				"restart_syscall",
				"rmdir",
				"rt_sigaction",
				"rt_sigpending",
				"rt_sigprocmask",
				"rt_sigqueueinfo",
				"rt_sigreturn",
				"rt_sigsuspend",
				"rt_sigtimedwait",
				"rt_tgsigqueueinfo",
				"sched_getaffinity",
				"sched_getattr",
				"sched_getparam",
				"sched_get_priority_max",
				"sched_get_priority_min",
				"sched_getscheduler",
				"sched_rr_get_interval",
				"sched_setaffinity",
				"sched_setattr",
				"sched_setparam",
				"sched_setscheduler",
				"sched_yield",
				"seccomp",
				"select",
				"semctl",
				"semget",
				"semop",
				"semtimedop",
				"send",
				"sendfile",
				"sendfile64",
				"sendmmsg",
				"sendmsg",
				"sendto",
				"setfsgid",
				"setfsgid32",
				"setfsuid",
				"setfsuid32",
				"setgid",
				"setgid32",
				"setgroups",
				"setgroups32",
				"setitimer",
				"setpgid",
				"setpriority",
				"setregid",
				"setregid32",
				"setresgid",
				"setresgid32",
				"setresuid",
				"setresuid32",
				"setreuid",
				"setreuid32",
				"setrlimit",
				"set_robust_list",
				"setsid",
				"setsockopt",
				"set_thread_area",
				"set_tid_address",
				"setuid",
				"setuid32",
				"setxattr",
				"shmat",
				"shmctl",
				"shmdt",
				"shmget",
				"shutdown",
				"sigaltstack",
				"signalfd",
				"signalfd4",
				"sigreturn",
				"socket",
				"socketcall",
				"socketpair",
				"splice",
				"stat",
				"stat64",
				"statfs",
				"statfs64",
				"symlink",
				"symlinkat",
				"sync",
				"sync_file_range",
				"syncfs",
				"sysinfo",
				"syslog",
				"tee",
				"tgkill",
				"time",
				"timer_create",
				"timer_delete",
				"timerfd_create",
				"timerfd_gettime",
				"timerfd_settime",
				"timer_getoverrun",
				"timer_gettime",
				"timer_settime",
				"times",
				"tkill",
				"truncate",
				"truncate64",
				"ugetrlimit",
				"umask",
				"uname",
				"unlink",
				"unlinkat",
				"utime",
				"utimensat",
				"utimes",
				"vfork",
				"vmsplice",
				"wait4",
				"waitid",
				"waitpid",
				"write",
				"writev",
				"mount",
				"umount2",
				"reboot",
				"name_to_handle_at",
				"unshare"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {},
			"excludes": {}
		},
		{
			"names": [
				"personality"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [
				{
					"index": 0,
					"value": 0,
					"valueTwo": 0,
					"op": "SCMP_CMP_EQ"
				}
			],
			"comment": "",
			"includes": {},
			"excludes": {}
		},
		{
			"names": [
				"personality"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [
				{
					"index": 0,
					"value": 8,
					"valueTwo": 0,
					"op": "SCMP_CMP_EQ"
				}
			],
			"comment": "",
			"includes": {},
			"excludes": {}
		},
		{
			"names": [
				"personality"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [
				{
					"index": 0,
					"value": 131072,
					"valueTwo": 0,
					"op": "SCMP_CMP_EQ"
				}
			],
			"comment": "",
			"includes": {},
			"excludes": {}
		},
		{
			"names": [
				"personality"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [
				{
					"index": 0,
					"value": 131080,
					"valueTwo": 0,
					"op": "SCMP_CMP_EQ"
				}
			],
			"comment": "",
			"includes": {},
			"excludes": {}
		},
		{
			"names": [
				"personality"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [
				{
					"index": 0,
					"value": 4294967295,
					"valueTwo": 0,
					"op": "SCMP_CMP_EQ"
				}
			],
			"comment": "",
			"includes": {},
			"excludes": {}
		},
		{
			"names": [
				"sync_file_range2"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"arches": [
					"ppc64le"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"arm_fadvise64_64",
				"arm_sync_file_range",
				"sync_file_range2",
				"breakpoint",
				"cacheflush",
				"set_tls"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"arches": [
					"arm",
					"arm64"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"arch_prctl"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"arches": [
					"amd64",
					"x32"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"modify_ldt"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"arches": [
					"amd64",
					"x32",
					"x86"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"s390_pci_mmio_read",
				"s390_pci_mmio_write",
				"s390_runtime_instr"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"arches": [
					"s390",
					"s390x"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"open_by_handle_at"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"caps": [
					"CAP_DAC_READ_SEARCH"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"bpf",
				"clone",
				"fanotify_init",
				"lookup_dcookie",
				"mount",
				"name_to_handle_at",
				"perf_event_open",
				"quotactl",
				"setdomainname",
				"sethostname",
				"setns",
				"umount",
				"umount2",
				"unshare"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"caps": [
					"CAP_SYS_ADMIN"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"clone"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [
				{
					"index": 0,
					"value": 2080505856,
					"valueTwo": 0,
					"op": "SCMP_CMP_MASKED_EQ"
				}
			],
			"comment": "",
			"includes": {},
			"excludes": {
				"caps": [
					"CAP_SYS_ADMIN"
				],
				"arches": [
					"s390",
					"s390x"
				]
			}
		},
		{
			"names": [
				"clone"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [
				{
					"index": 1,
					"value": 2080505856,
					"valueTwo": 0,
					"op": "SCMP_CMP_MASKED_EQ"
				}
			],
			"comment": "s390 parameter ordering for clone is different",
			"includes": {
				"arches": [
					"s390",
					"s390x"
				]
			},
			"excludes": {
				"caps": [
					"CAP_SYS_ADMIN"
				]
			}
		},
		{
			"names": [
				"reboot"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"caps": [
					"CAP_SYS_BOOT"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"chroot"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"caps": [
					"CAP_SYS_CHROOT"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"delete_module",
				"init_module",
				"finit_module",
				"query_module"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"caps": [
					"CAP_SYS_MODULE"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"acct"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"caps": [
					"CAP_SYS_PACCT"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"kcmp",
				"process_vm_readv",
				"process_vm_writev",
				"ptrace"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"caps": [
					"CAP_SYS_PTRACE"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"iopl",
				"ioperm"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"caps": [
					"CAP_SYS_RAWIO"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"settimeofday",
				"stime",
				"clock_settime"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"caps": [
					"CAP_SYS_TIME"
				]
			},
			"excludes": {}
		},
		{
			"names": [
				"vhangup"
			],
			"action": "SCMP_ACT_ALLOW",
			"args": [],
			"comment": "",
			"includes": {
				"caps": [
					"CAP_SYS_TTY_CONFIG"
				]
			},
			"excludes": {}
		}
	]
}
`
