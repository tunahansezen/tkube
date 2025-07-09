```shell
make iso OS_NAME=ubuntu OS_VERSION=22.04 DOCKER_VERSION=20.10.24 KUBE_VERSION=1.30.2
```

```shell
make iso OS_NAME=centos OS_VERSION=7 DOCKER_VERSION=20.10.24 KUBE_VERSION=1.30.2
```

```shell
make iso OS_NAME=rockylinux OS_VERSION=9 DOCKER_VERSION=20.10.24 KUBE_VERSION=1.30.2
```

```shell
make iso OS_NAME=ubuntu OS_VERSION=18.04 DOCKER_VERSION=20.10.24 KUBE_VERSION=1.18.3 \
  KUBE_REPO_KEY="https://customrepo.com.tr/repository/external-raw/gpg-keys/kube.gpg" \
  KUBE_REPO_ADDRESS="https://customrepo.com.tr/repository/kubernetes kubernetes-xenial main" \
  EXTRA_DOCKER_BUILD_ARGS="--add-host=customrepo.com.tr:192.168.1.101"
```

```shell
make iso OS_NAME=ubuntu OS_VERSION=18.04 DOCKER_VERSION=19.03.9 KUBE_VERSION=1.18.3 \
  ETCD_VERSION=3.3.4 \
  HELM_VERSION=3.5.4 \
  CALICO_VERSION=3.17.2 \
  CALICO_URL=https://sebarepo.argela.com.tr/repository/argela-raw/calico/calico-3.17.2.yaml \
  KUBE_REPO_KEY="https://sebarepo.argela.com.tr/repository/sepon-raw/gpg-keys/seba-apt.gpg" \
  KUBE_REPO_ADDRESS="https://sebarepo.argela.com.tr/repository/seba-apt-stable stable main" \
  EXTRA_DOCKER_BUILD_ARGS="--add-host=sebarepo.argela.com.tr:192.168.31.202"
```
