package templates

import (
	"github.com/lithammer/dedent"
	"text/template"
)

var (
	CheckApiserverSh = template.Must(template.New("check_apiserver.sh").Parse(
		dedent.Dedent(`#!/bin/sh
errorExit() {
  echo "*** $@" 1>&2
  exit 1
}

curl --silent --max-time 2 --insecure https://localhost:6443/ -o /dev/null || errorExit "Error GET https://localhost:6443/"
if ip addr | grep -q {{ .VirtualIP }}; then
    curl --silent --max-time 2 --insecure https://{{ .VirtualIP }}:6443/ -o /dev/null || errorExit "Error GET https://{{ .VirtualIP }}:6443/"
fi
`)))
)
