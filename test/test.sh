#!/usr/bin/env bash
set -e

red_color="\033[0;31m"
color_off="\033[0m"

test_id=""
keep_vagrant=0
skip_vagrant_up=0

node_ips=("192.168.50.10" "192.168.50.20" "192.168.50.30")

script_dir=$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)
if [ -z "$VERSION" ]; then
  version=$(cat $script_dir/../VERSION)
else
  version="$VERSION"
fi

ssh_known_host() {
  host="$1"
  port="$2"
  ssh-keygen -f "$HOME"/.ssh/known_hosts -R "$host" >/dev/null 2>&1
  ssh_key=$(ssh-keyscan "$host" 2>/dev/null)
  echo "$ssh_key" >>"$HOME"/.ssh/known_hosts
  if [ -n "$port" ]; then
    ssh-keygen -f "$HOME"/.ssh/known_hosts -R ["$host"]:"$port" >/dev/null 2>&1
    ssh_key=$(ssh-keyscan -H "$host" -p "$port" 2>/dev/null)
    echo "$ssh_key" >>"$HOME"/.ssh/known_hosts
  fi
}

while [ $# -gt 0 ]; do
  case "$1" in
  --keep-vagrant)
    keep_vagrant=1
    shift
    ;;
  --skip-vagrant-up)
    skip_vagrant_up=1
    shift
    ;;
  --test-id=*)
    test_id="${1#*=}"
    shift
    ;;
  --test-id)
    test_id="$2"
    shift 2
    ;;
  *)
    if [ "$1" == "destroy" ]; then
      cd "env1" || exit
      sudo vagrant destroy -f
      sudo rm -rf .vagrant
      cd - || exit
      cd "env2" || exit
      sudo vagrant destroy -f
      sudo rm -rf .vagrant
      cd - || exit
      exit 0
    else
      echo "Unknown command: $1"
      exit 1
    fi
    shift 1
    ;;
  esac
done

if [ -z "$test_id" ]; then
  echo -n "Enter the test id: "
  read -r test_id
fi

case $test_id in
  ubuntu22-online-single)
    cd "$script_dir" || exit
    if [ "$skip_vagrant_up" -eq 0 ]; then
      sudo N=1 vagrant destroy -f
      sudo rm -rf .vagrant
      sudo N=1 IMAGE_NAME="bento/ubuntu-22.04" vagrant up
    fi
    ssh_known_host "192.168.50.10"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "mkdir -p \$HOME/.tkube/config"
    sshpass -p vagrant scp config/deployment-ubuntu-single.yaml vagrant@192.168.50.10:/home/vagrant/.tkube/config/deployment.yaml
    go run ../main.go install --remote 192.168.50.10
    if [ "$keep_vagrant" -eq 0 ]; then
      sudo N=1 IMAGE_NAME="bento/ubuntu-22.04" vagrant destroy -f
      sudo rm -rf .vagrant
    fi
    cd - || exit
    exit 0
    ;;
  ubuntu22-online)
    cd "$script_dir" || exit
    if [ "$skip_vagrant_up" -eq 0 ]; then
      sudo N=3 vagrant destroy -f
      sudo rm -rf .vagrant
      sudo N=3 IMAGE_NAME="bento/ubuntu-22.04" vagrant up
    fi
    ssh_known_host "192.168.50.10"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "mkdir -p \$HOME/.tkube/config"
    sshpass -p vagrant scp config/deployment-ubuntu.yaml vagrant@192.168.50.10:/home/vagrant/.tkube/config/deployment.yaml
    go run ../main.go install --remote 192.168.50.10
    if [ "$keep_vagrant" -eq 0 ]; then
      sudo N=3 IMAGE_NAME="bento/ubuntu-22.04" vagrant destroy -f
      sudo rm -rf .vagrant
    fi
    cd - || exit
    exit 0
    ;;
  centos7-online)
    cd "$script_dir" || exit
    if [ "$skip_vagrant_up" -eq 0 ]; then
      sudo N=3 vagrant destroy -f
      sudo rm -rf .vagrant
      sudo N=3 IMAGE_NAME="bento/centos-7.9" vagrant up
    fi
    ssh_known_host "192.168.50.10"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "mkdir -p \$HOME/.tkube/config"
    sshpass -p vagrant scp config/deployment-centos.yaml vagrant@192.168.50.10:/home/vagrant/.tkube/config/deployment.yaml
    sshpass -p vagrant ssh vagrant@192.168.50.10 "sudo sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-*"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "sudo sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*"
    ssh_known_host "192.168.50.20"
    sshpass -p vagrant ssh vagrant@192.168.50.20 "sudo sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-*"
    sshpass -p vagrant ssh vagrant@192.168.50.20 "sudo sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*"
    ssh_known_host "192.168.50.30"
    sshpass -p vagrant ssh vagrant@192.168.50.30 "sudo sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-*"
    sshpass -p vagrant ssh vagrant@192.168.50.30 "sudo sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*"
    go run ../main.go install --remote 192.168.50.10
    if [ "$keep_vagrant" -eq 0 ]; then
      sudo vagrant destroy -f
      sudo N=3 rm -rf .vagrant
    fi
    cd - || exit
    exit 0
    ;;
  rocky9-online-single)
    cd "$script_dir" || exit
    if [ "$skip_vagrant_up" -eq 0 ]; then
      sudo N=1 vagrant destroy -f
      sudo rm -rf .vagrant
      sudo N=1 IMAGE_NAME="bento/rockylinux-9" vagrant up
    fi
    ssh_known_host "192.168.50.10"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "mkdir -p \$HOME/.tkube/config"
    sshpass -p vagrant scp config/deployment-rocky-single.yaml vagrant@192.168.50.10:/home/vagrant/.tkube/config/deployment.yaml
    go run ../main.go install --remote 192.168.50.10
    if [ "$keep_vagrant" -eq 0 ]; then
      sudo N=1 IMAGE_NAME="bento/rockylinux-9" vagrant destroy -f
      sudo rm -rf .vagrant
    fi
    cd - || exit
    exit 0
    ;;
  rocky9-offline-single)
    cd "$script_dir" || exit
    if [ "$skip_vagrant_up" -eq 0 ]; then
      sudo N=1 vagrant destroy -f
      sudo rm -rf .vagrant
      sudo N=1 IMAGE_NAME="bento/rockylinux-9" vagrant up
    fi
    ssh_known_host "192.168.50.10"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "mkdir -p \$HOME/.tkube/config"
    sshpass -p vagrant scp config/deployment-rocky-single.yaml vagrant@192.168.50.10:/home/vagrant/.tkube/config/deployment.yaml
    sshpass -p vagrant scp disable-internet-access.sh vagrant@192.168.50.10:/home/vagrant/
    sshpass -p vagrant ssh vagrant@192.168.50.10 "./disable-internet-access.sh"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "[ -d /etc/yum.repos.d ] && sudo mv /etc/yum.repos.d /etc/yum.repos.d.old && sudo mkdir /etc/yum.repos.d"
    sshpass -p vagrant scp ../offline/output/rockylinux-9_kube-1.30.2_registry-${version}.iso vagrant@192.168.50.10:/home/vagrant/
    go run ../main.go install --remote 192.168.50.10 --iso /home/vagrant/rockylinux-9_kube-1.30.2_registry-${version}.iso --debug
    if [ "$keep_vagrant" -eq 0 ]; then
      sudo N=1 vagrant destroy -f
      sudo rm -rf .vagrant
    fi
    cd - || exit
    exit 0
    ;;
  ubuntu22-offline) # ubuntu-22.04 kube-1.30.2
    cd "$script_dir" || exit
    if [ "$skip_vagrant_up" -eq 0 ]; then
      sudo N=3 vagrant destroy -f
      sudo rm -rf .vagrant
      sudo N=3 IMAGE_NAME="bento/ubuntu-22.04" vagrant up
    fi
    ssh_known_host "192.168.50.10"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "mkdir -p \$HOME/.tkube/config"
    sshpass -p vagrant scp config/deployment-ubuntu.yaml vagrant@192.168.50.10:/home/vagrant/.tkube/config/deployment.yaml
    sshpass -p vagrant scp disable-internet-access.sh vagrant@192.168.50.10:/home/vagrant/
    sshpass -p vagrant ssh vagrant@192.168.50.10 "./disable-internet-access.sh"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "[ -f /etc/apt/sources.list ] && sudo mv /etc/apt/sources.list /etc/apt/sources.list.old"
    ssh_known_host "192.168.50.20"
    sshpass -p vagrant scp disable-internet-access.sh vagrant@192.168.50.20:/home/vagrant/
    sshpass -p vagrant ssh vagrant@192.168.50.20 "./disable-internet-access.sh"
    sshpass -p vagrant ssh vagrant@192.168.50.20 "[ -f /etc/apt/sources.list ] && sudo mv /etc/apt/sources.list /etc/apt/sources.list.old"
    ssh_known_host "192.168.50.30"
    sshpass -p vagrant scp disable-internet-access.sh vagrant@192.168.50.30:/home/vagrant/
    sshpass -p vagrant ssh vagrant@192.168.50.30 "./disable-internet-access.sh"
    sshpass -p vagrant ssh vagrant@192.168.50.30 "[ -f /etc/apt/sources.list ] && sudo mv /etc/apt/sources.list /etc/apt/sources.list.old"
    sshpass -p vagrant scp ../offline/output/ubuntu-22.04_kube-1.30.2_registry-${version}.iso vagrant@192.168.50.10:/home/vagrant/
    go run ../main.go install --remote 192.168.50.10 --iso /home/vagrant/ubuntu-22.04_kube-1.30.2_registry-${version}.iso
    if [ "$keep_vagrant" -eq 0 ]; then
      sudo N=3 vagrant destroy -f
      sudo rm -rf .vagrant
    fi
    cd - || exit
    exit 0
    ;;
  ubuntu18-offline-kube18) # ubuntu-18.04 kube-1.18.3
    cd "$script_dir" || exit
    if [ "$skip_vagrant_up" -eq 0 ]; then
      sudo N=3 vagrant destroy -f
      sudo rm -rf .vagrant
      sudo N=3 IMAGE_NAME="bento/ubuntu-18.04" vagrant up
    fi
    ssh_known_host "192.168.50.10"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "mkdir -p \$HOME/.tkube/config"
    sshpass -p vagrant scp config/deployment-ubuntu.yaml vagrant@192.168.50.10:/home/vagrant/.tkube/config/deployment.yaml
    sshpass -p vagrant scp disable-internet-access.sh vagrant@192.168.50.10:/home/vagrant/
    sshpass -p vagrant ssh vagrant@192.168.50.10 "./disable-internet-access.sh"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "[ -f /etc/apt/sources.list ] && sudo mv /etc/apt/sources.list /etc/apt/sources.list.old"
    ssh_known_host "192.168.50.20"
    sshpass -p vagrant scp disable-internet-access.sh vagrant@192.168.50.20:/home/vagrant/
    sshpass -p vagrant ssh vagrant@192.168.50.20 "./disable-internet-access.sh"
    sshpass -p vagrant ssh vagrant@192.168.50.20 "[ -f /etc/apt/sources.list ] && sudo mv /etc/apt/sources.list /etc/apt/sources.list.old"
    ssh_known_host "192.168.50.30"
    sshpass -p vagrant scp disable-internet-access.sh vagrant@192.168.50.30:/home/vagrant/
    sshpass -p vagrant ssh vagrant@192.168.50.30 "./disable-internet-access.sh"
    sshpass -p vagrant ssh vagrant@192.168.50.30 "[ -f /etc/apt/sources.list ] && sudo mv /etc/apt/sources.list /etc/apt/sources.list.old"
    sshpass -p vagrant scp ../offline/output/ubuntu-18.04_kube-1.18.3_registry-${version}.iso vagrant@192.168.50.10:/home/vagrant/
    go run ../main.go install --remote 192.168.50.10 --iso /home/vagrant/ubuntu-18.04_kube-1.18.3_registry-${version}.iso
    if [ "$keep_vagrant" -eq 0 ]; then
      sudo N=3 vagrant destroy -f
      sudo rm -rf .vagrant
    fi
    cd - || exit
    exit 0
    ;;
  centos7-offline) # centos-7.9 kube-1.30.2
    cd "$script_dir" || exit
    if [ "$skip_vagrant_up" -eq 0 ]; then
      sudo N=3 vagrant destroy -f
      sudo rm -rf .vagrant
      sudo N=3 IMAGE_NAME="bento/centos-7.9" vagrant up
    fi
    ssh_known_host "192.168.50.10"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "mkdir -p \$HOME/.tkube/config"
    sshpass -p vagrant scp config/deployment-centos.yaml vagrant@192.168.50.10:/home/vagrant/.tkube/config/deployment.yaml
    sshpass -p vagrant scp disable-internet-access.sh vagrant@192.168.50.10:/home/vagrant/
    sshpass -p vagrant ssh vagrant@192.168.50.10 "./disable-internet-access.sh"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "[ -d /etc/yum.repos.d ] && sudo mv /etc/yum.repos.d /etc/yum.repos.d.old && sudo mkdir /etc/yum.repos.d"
    ssh_known_host "192.168.50.20"
    sshpass -p vagrant scp disable-internet-access.sh vagrant@192.168.50.20:/home/vagrant/
    sshpass -p vagrant ssh vagrant@192.168.50.20 "./disable-internet-access.sh"
    sshpass -p vagrant ssh vagrant@192.168.50.20 "[ -d /etc/yum.repos.d ] && sudo mv /etc/yum.repos.d /etc/yum.repos.d.old && sudo mkdir /etc/yum.repos.d"
    ssh_known_host "192.168.50.30"
    sshpass -p vagrant scp disable-internet-access.sh vagrant@192.168.50.30:/home/vagrant/
    sshpass -p vagrant ssh vagrant@192.168.50.30 "./disable-internet-access.sh"
    sshpass -p vagrant ssh vagrant@192.168.50.30 "[ -d /etc/yum.repos.d ] && sudo mv /etc/yum.repos.d /etc/yum.repos.d.old && sudo mkdir /etc/yum.repos.d"
    sshpass -p vagrant scp ../offline/output/centos-7_kube-1.30.2_registry-${version}.iso vagrant@192.168.50.10:/home/vagrant/
    go run ../main.go install --remote 192.168.50.10 --iso /home/vagrant/centos-7_kube-1.30.2_registry-${version}.iso
    if [ "$keep_vagrant" -eq 0 ]; then
      sudo N=3 vagrant destroy -f
      sudo rm -rf .vagrant
    fi
    cd - || exit
    exit 0
    ;;
  ubuntu22-worker)
    cd "$script_dir" || exit
    if [ "$skip_vagrant_up" -eq 0 ]; then
      sudo N=4 vagrant destroy -f
      sudo rm -rf .vagrant
      sudo N=4 IMAGE_NAME="bento/ubuntu-22.04" vagrant up
    fi
    ssh_known_host "192.168.50.10"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "mkdir -p \$HOME/.tkube/config"
    sshpass -p vagrant scp config/deployment-ubuntu-workers.yaml vagrant@192.168.50.10:/home/vagrant/.tkube/config/deployment.yaml
    go run ../main.go install --remote 192.168.50.10 --skip-workers
    if [ "$keep_vagrant" -eq 0 ]; then
      sudo N=4 IMAGE_NAME="bento/ubuntu-22.04" vagrant destroy -f
      sudo rm -rf .vagrant
    fi
    cd - || exit
    exit 0
    ;;
esac
echo -e "$red_color""Test not found""$color_off"
exit 1
