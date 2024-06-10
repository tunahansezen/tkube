#!/bin/bash

red_color="\033[0;31m"
color_off="\033[0m"

keep_vagrant=0
skip_vagrant_up=0

node_ips=("192.168.50.10" "192.168.50.20" "192.168.50.30")

script_dir=$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)

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
    shift 1
    ;;
  --skip-vagrant-up)
    skip_vagrant_up=1
    shift 1
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

echo -n "Enter the test number: "
read -r test_no

case $test_no in
  1)
    cd "$script_dir/ubuntu" || exit
    if [ "$skip_vagrant_up" -eq 0 ]; then
      sudo vagrant destroy -f
      sudo rm -rf .vagrant
      sudo vagrant up
    fi
    ssh_known_host "192.168.50.10"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "mkdir -p \$HOME/.tkube/config"
    sshpass -p vagrant scp deployment.yaml vagrant@192.168.50.10:/home/vagrant/.tkube/config/deployment.yaml
    go run ../../main.go install --remote 192.168.50.10
    if [ "$keep_vagrant" -eq 0 ]; then
      sudo vagrant destroy -f
      sudo rm -rf .vagrant
    fi
    cd - || exit
    exit 0
    ;;
  2)
    cd "$script_dir/centos" || exit
    if [ "$skip_vagrant_up" -eq 0 ]; then
      sudo vagrant destroy -f
      sudo rm -rf .vagrant
      sudo vagrant up
    fi
    ssh_known_host "192.168.50.10"
    sshpass -p vagrant ssh vagrant@192.168.50.10 "mkdir -p \$HOME/.tkube/config"
    sshpass -p vagrant scp deployment.yaml vagrant@192.168.50.10:/home/vagrant/.tkube/config/deployment.yaml
    go run ../../main.go install --remote 192.168.50.10
    if [ "$keep_vagrant" -eq 0 ]; then
      sudo vagrant destroy -f
      sudo rm -rf .vagrant
    fi
    cd - || exit
    exit 0
    ;;
esac
echo -e "$red_color""Test not found""$color_off"
exit 1
