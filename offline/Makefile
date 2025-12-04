SHELL                              = bash -e -o pipefail
VERSION                           ?= $(shell cat ./../VERSION)

TARGET_ARCH			              ?= amd64
DOCKER_VERSION                     =
DOCKER_REPO_KEY                    =
DEFAULT_APT_DOCKER_REPO_KEY     = https://download.docker.com/linux/ubuntu/gpg
DEFAULT_YUM_DOCKER_REPO_KEY     = https://download.docker.com/linux/centos/gpg
DOCKER_REPO_ADDRESS                =
DEFAULT_APT_DOCKER_REPO_ADDRESS = https://download.docker.com/linux/ubuntu ${OS_RELEASE} stable
DEFAULT_YUM_DOCKER_REPO_ADDRESS = https://download.docker.com/linux/centos/7.9/x86_64/stable # fixme static
KUBE_VERSION                       =
KUBE_MAJOR_VERSION                 = $(shell echo $(KUBE_VERSION) | cut -d '.' -f 1,2)
KUBE_REPO_KEY                      =
DEFAULT_APT_KUBE_REPO_KEY       = https://pkgs.k8s.io/core:/stable:/v${KUBE_MAJOR_VERSION}/deb/Release.key
DEFAULT_YUM_KUBE_REPO_KEY       = https://pkgs.k8s.io/core:/stable:/v${KUBE_MAJOR_VERSION}/rpm/repodata/repomd.xml.key
KUBE_REPO_ADDRESS                  =
DEFAULT_APT_KUBE_REPO_ADDRESS   = https://pkgs.k8s.io/core:/stable:/v${KUBE_MAJOR_VERSION}/deb/ /
DEFAULT_YUM_KUBE_REPO_ADDRESS   = https://pkgs.k8s.io/core:/stable:/v${KUBE_MAJOR_VERSION}/rpm/
ETCD_VERSION                       = 3.5.25
ETCD_URL                           = https://github.com/coreos/etcd/releases/download/v${ETCD_VERSION}/etcd-v${ETCD_VERSION}-linux-${TARGET_ARCH}.tar.gz
HELM_VERSION                       = 3.19.2
HELM_URL                           = https://get.helm.sh/helm-v${HELM_VERSION}-linux-${TARGET_ARCH}.tar.gz
EXTRA_DOCKER_BUILD_ARGS            =

DEST_PATH 			     = ./output
OS_ARR                   = ubuntu centos rockylinux
ubuntu_VERSIONS          = 18.04 20.04 22.04 24.04
centos_VERSIONS          = 7
rockylinux_VERSIONS           = 9
define ubuntu_RELEASES
18.04=bionic\
20.04=focal\
22.04=jammy\
24.04=noble
endef
define centos_RELEASES
7=centos
endef
define rockylinux_RELEASES
9=rockylinux
endef

check_semver = $(if $(shell echo $(1) | grep -Eq '^([0-9]+)\.([0-9]+)\.([0-9]+)$$'; echo $$?),, $(error Version '$(1)' is not valid. It should be in the form of MAJOR.MINOR.PATCH))
check_os_name = $(if $(filter $(1), $(OS_ARR)),, $(error OS_NAME '$(1)' is not valid. Valid options are: $(OS_ARR)))
get_versions = $($(1)_VERSIONS)
get_releases = $($(1)_RELEASES)
check_os_version = $(if $(filter $(2), $(call get_versions,$(1))),, $(error OS_VERSION '$(2)' is not valid for OS_NAME '$(1)'. Valid options for '$(1)' are: $(call get_versions,$(1))))
get_release = $(strip $(foreach release,$(call get_releases,$(1)),$(if $(findstring $(2)=,$(release)),$(subst $(2)=,,$(release)))))

KUBE_CALICO_COMPATIBILITY := \
    "1.34=3.31.2" \
    "1.33=3.31.2" \
    "1.32=3.31.2" \
    "1.31=3.28.0" \
    "1.30=3.28.0" \
    "1.29=3.27.3" \
    "1.28=3.27.3" \
    "1.27=3.27.3" \
    "1.26=3.26.4" \
    "1.25=3.26.4" \
    "1.24=3.26.4" \
    "1.23=3.25.2" \
    "1.22=3.24.6" \
    "1.21=3.23" \
    "1.20=3.21" \
    "1.19=3.20" \
    "1.18=3.18" \
    "1.17=3.17"

define get_calico_version
    $(shell \
        found="false"; \
        for entry in $(KUBE_CALICO_COMPATIBILITY); do \
            kube=$$(echo $$entry | cut -d '=' -f 1); \
            calico=$$(echo $$entry | cut -d '=' -f 2); \
            if [ "$$(echo $(1) | cut -d '.' -f 1-2)" = "$$kube" ]; then \
                echo "$$calico"; \
                found="true"; \
                break; \
            fi; \
        done; \
        if [ "$$found" != "true" ]; then \
            echo "No Calico version found for Kubernetes $(1)"; \
            exit 1; \
        fi \
    )
endef

define get_calico_exact_url
	$(shell \
		major_version=$$(echo $(1) | cut -d '.' -f 1); \
		minor_version=$$(echo $(1) | cut -d '.' -f 2); \
		if [ $$major_version -ge 3 ] && [ $$minor_version -ge 24 ]; then \
			echo "https://raw.githubusercontent.com/projectcalico/calico/v$(1)/manifests/calico.yaml"; \
		else \
			echo "https://projectcalico.docs.tigera.io/archive/v$(1)/manifests/calico.yaml"; \
		fi \
    )
endef

check-vars: ## Check required parameters
ifndef OS_NAME
	$(error OS_NAME is undefined)
endif
	@$(call check_os_name,$(OS_NAME))
ifndef OS_VERSION
	$(error OS_VERSION is undefined)
endif
	@$(call check_os_version,$(OS_NAME),$(OS_VERSION))
ifndef DOCKER_VERSION
	$(error DOCKER_VERSION is undefined)
endif
	@$(call check_semver,$(DOCKER_VERSION))
ifndef KUBE_VERSION
	$(error KUBE_VERSION is undefined)
endif
	$(eval OS_RELEASE=$(call get_release,$(OS_NAME),$(OS_VERSION)))
	@if [ -z "$(OS_RELEASE)" ]; then echo "Error: No release found for OS_VERSION '$(OS_VERSION)' in OS_NAME '$(OS_NAME)'"; exit 1; fi

calculate_calico:
	$(eval CALICO_VERSION=$(call get_calico_version,$(KUBE_VERSION)))
	@echo "Calico version for Kubernetes $(KUBE_VERSION): $(CALICO_VERSION)"
	$(eval CALICO_URL=$(call get_calico_exact_url,$(CALICO_VERSION)))
	@echo "Calico URL for Kubernetes $(KUBE_VERSION): $(CALICO_URL)"

calculate_repos:
ifndef DOCKER_REPO_KEY
ifeq ($(OS_NAME), ubuntu)
	$(eval DOCKER_REPO_KEY=${DEFAULT_APT_DOCKER_REPO_KEY})
else
	$(eval DOCKER_REPO_KEY=${DEFAULT_YUM_DOCKER_REPO_KEY})
endif
endif
ifndef DOCKER_REPO_ADDRESS
ifeq ($(OS_NAME), ubuntu)
	$(eval DOCKER_REPO_ADDRESS=${DEFAULT_APT_DOCKER_REPO_ADDRESS})
else
	$(eval DOCKER_REPO_ADDRESS=${DEFAULT_YUM_DOCKER_REPO_ADDRESS})
endif
endif
ifndef KUBE_REPO_KEY
ifeq ($(OS_NAME), ubuntu)
	$(eval KUBE_REPO_KEY=${DEFAULT_APT_KUBE_REPO_KEY})
else
	$(eval KUBE_REPO_KEY=${DEFAULT_YUM_KUBE_REPO_KEY})
endif
endif
ifndef KUBE_REPO_ADDRESS
ifeq ($(OS_NAME), ubuntu)
	$(eval KUBE_REPO_ADDRESS=${DEFAULT_APT_KUBE_REPO_ADDRESS})
else
	$(eval KUBE_REPO_ADDRESS=${DEFAULT_YUM_KUBE_REPO_ADDRESS})
endif
endif

iso: check-vars calculate_repos calculate_calico
	DOCKER_BUILDKIT=1 docker build \
    	  -t tkube:$(VERSION) \
          --platform linux/$(TARGET_ARCH) \
    	  --build-arg OS_NAME=$(OS_NAME) \
    	  --build-arg OS_VERSION=$(OS_VERSION) \
    	  --build-arg OS_RELEASE=$(OS_RELEASE) \
    	  --build-arg VERSION=$(VERSION) \
    	  --build-arg TARGET_ARCH=$(TARGET_ARCH) \
    	  --build-arg DOCKER_VERSION=$(DOCKER_VERSION) \
    	  --build-arg DOCKER_REPO_KEY="$(DOCKER_REPO_KEY)" \
    	  --build-arg DOCKER_REPO_ADDRESS="$(DOCKER_REPO_ADDRESS)" \
    	  --build-arg KUBE_VERSION=$(KUBE_VERSION) \
    	  --build-arg KUBE_MAJOR_VERSION=$(KUBE_MAJOR_VERSION) \
    	  --build-arg KUBE_REPO_KEY="$(KUBE_REPO_KEY)" \
    	  --build-arg KUBE_REPO_ADDRESS="$(KUBE_REPO_ADDRESS)" \
    	  --build-arg CALICO_VERSION=$(CALICO_VERSION) \
    	  --build-arg CALICO_URL=$(CALICO_URL) \
    	  --build-arg ETCD_VERSION=$(ETCD_VERSION) \
    	  --build-arg ETCD_URL=$(ETCD_URL) \
    	  --build-arg HELM_VERSION=$(HELM_VERSION) \
    	  --build-arg HELM_URL=$(HELM_URL) \
    	  --network host \
    	  $(EXTRA_DOCKER_BUILD_ARGS) \
    	  --output $(DEST_PATH) \
    	  -f iso/$(OS_NAME).Dockerfile .
