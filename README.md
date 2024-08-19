# tkube

## Getting Started

### Prerequisites
Installation node needs following packages:
sshpass

### Installation

#### Debian Package Standalone
```sh
dpkg -i tkube.x.y.z_amd64.deb
```

## Shell Completion

`tkube` supports shell completion for the `bash` shell. To enable shell
Completion you can use the following command on *most* \*nix based system.

```shell
source <(tkube completion bash)
```

If you are running an older bash 3.x shell (default on macOS), then you can try
the following command:

```shell
source /dev/stdin <<<"$(tkube completion bash)"
```

If you which to make `bash` shell completion automatic when you log in to your
account you can use the following command:

```shell
echo 'source <(tkube completion bash)' >>~/.bashrc
```

## Usage and Commands

```shell
Usage:
  tkube [command]

Use "tkube [command] --help" for more information about a command.
```

Help specific to each command can be found by running `tkube <command> -h`.

Tested with:

OS: ubuntu:20.04\
vagrant: 2.4.1\
virtualbox: 7.0.20

## Roadmap

- [x] CentOS support
- [x] Add worker node
- [ ] Do not install required images if exists
- [ ] CentOS versionlock delete before installation
- [ ] Override registry if ISO installation
- [ ] Rocky Linux support
- [ ] Set log retention and size for kube>=1.24 from kubelet config
- [ ] Set max pod count for cluster
- [ ] Set fs.inotify.max_user_watches to enough value
- [ ] Update containerd.io if older version installed
- [ ] Handle containerd.io for kube>=1.24 separately from docker-ce
- [ ] Different network plugin support
    - [ ] Flannel
    - [ ] none
- [ ] Include helmfile package and required plugins as optional
