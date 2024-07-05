package templates

import (
	"github.com/lithammer/dedent"
	"text/template"
)

var (
	KeepalivedConf = template.Must(template.New("keepalived.conf").Parse(
		dedent.Dedent(`global_defs {
    router_id LVS_DEVEL
}

vrrp_script check_apiserver {
  script "/etc/keepalived/check_apiserver.sh"
  interval 3
  weight -2
  fall 10
  rise 2
}

vrrp_instance VI_1 {
    state MASTER
    interface {{ .Interface }}
    virtual_router_id {{ .VirtualRouterID }}
    priority {{ .Priority }}
    authentication {
        auth_type PASS
        auth_pass {{ .AuthPass }}
    }
    virtual_ipaddress {
        {{ .VirtualIP }}
    }
    track_script {
        check_apiserver
    }
}
`)))
)
