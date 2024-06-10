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
