ARG OS_NAME=ubuntu
ARG OS_VERSION=22.04
FROM $OS_NAME:$OS_VERSION as os
ARG OS_NAME
ARG OS_VERSION
ARG VERSION=0.0.0
ARG TARGET_ARCH=amd64
ARG OS_RELEASE=jammy
ARG DIR=${OS_NAME}-${OS_VERSION}-${TARGET_ARCH}-rpms
ARG DEBIAN_FRONTEND=noninteractive
ENV TZ=Etc/UTC
ARG PKGS=.common[],.$OS_NAME[]
ARG MANDATORY_PACKAGES="tzdata ca-certificates curl wget gnupg2 yum-utils createrepo mkisofs epel-release"

RUN yum install -q -y $MANDATORY_PACKAGES

WORKDIR /package
COPY iso/packages.yaml .

COPY --from=mikefarah/yq:4.44.1 /usr/bin/yq /usr/bin/yq
RUN yq eval "${PKGS}" packages.yaml | xargs yum install -q -y

ARG DOCKER_VERSION=20.10.24
ARG DOCKER_REPO_KEY="https://download.docker.com/linux/ubuntu/gpg"
ARG DOCKER_REPO_ADDRESS="https://download.docker.com/linux/ubuntu $OS_RELEASE stable"
ARG DOCKER_GPG_PATH="/etc/pki/rpm-gpg/docker.gpg"
RUN curl -fsSL $DOCKER_REPO_KEY | tee ${DOCKER_GPG_PATH} >/dev/null \
    && rpm --import ${DOCKER_GPG_PATH} \
    && echo -e "[docker]\nname=docker\nbaseurl=${DOCKER_REPO_ADDRESS}\nenabled=1\ngpgcheck=1\ngpgkey=file://${DOCKER_GPG_PATH}\n" > /etc/yum.repos.d/docker.repo \
    && yum makecache -y \
    && DOCKER_EXACT_VERSION=$(yum list docker-ce --showduplicates 2>/dev/null | grep ${DOCKER_VERSION} | tail -1 | xargs | cut -d ' ' -f2 | cut -d ':' -f2 | cut -d '-' -f1) \
    && if [ -z "$DOCKER_EXACT_VERSION" ]; then echo "Docker not found with version $DOCKER_VERSION"; exit 1; fi \
    && yum install -y -q -y docker-ce-$DOCKER_EXACT_VERSION docker-ce-cli-$DOCKER_EXACT_VERSION

ARG KUBE_VERSION=1.30.2
ARG KUBE_MAJOR_VERSION=1.30
ARG KUBE_REPO_KEY="https://pkgs.k8s.io/core:/stable:/v$KUBE_MAJOR_VERSION/deb/Release.key"
ARG KUBE_REPO_ADDRESS="https://pkgs.k8s.io/core:/stable:/v$KUBE_MAJOR_VERSION/deb/ /"
ARG KUBE_GPG_PATH="/etc/pki/rpm-gpg/kube.gpg"
RUN curl -fsSL $KUBE_REPO_KEY | tee ${KUBE_GPG_PATH} >/dev/null \
    && rpm --import ${KUBE_GPG_PATH} \
    && echo -e "[kube]\nname=kube\nbaseurl=${KUBE_REPO_ADDRESS}\nenabled=1\ngpgcheck=1\ngpgkey=file://${KUBE_GPG_PATH}\n" > /etc/yum.repos.d/kube.repo \
    && yum makecache -y \
    && KUBE_EXACT_VERSION=$(yum list kubelet --showduplicates 2>/dev/null | grep ${KUBE_VERSION} | tail -1 | xargs | cut -d ' ' -f2 | cut -d ':' -f2 | cut -d '-' -f1) \
    && if [ -z "$KUBE_EXACT_VERSION" ]; then echo "Kubernetes not found with version $KUBE_VERSION"; exit 1; fi \
    && yum install -y -q -y kubelet-$KUBE_EXACT_VERSION kubeadm-$KUBE_EXACT_VERSION kubectl-$KUBE_EXACT_VERSION

ARG REPO_DIR=$DIR/repo
RUN rpm -qa > packages.list \
    && mkdir -p ${REPO_DIR} \
    && sort -u packages.list | xargs repoquery --location >> packages.urls \
    && wget -P ${REPO_DIR} -i packages.urls \
    && createrepo -d ${REPO_DIR}

ARG KUBE_IMAGE_REGISTRY=registry.k8s.io
ARG SKOPEO_VERSION=1.14.4
RUN wget https://github.com/lework/skopeo-binary/releases/download/v${SKOPEO_VERSION}/skopeo-linux-${TARGET_ARCH} \
    && mv skopeo-linux-${TARGET_ARCH} /usr/bin/skopeo \
    && chmod +x /usr/bin/skopeo
COPY iso/skopeo-default-policy.json /etc/containers/policy.json
RUN mkdir -p ${DIR}/kubernetes/images \
    && kubeadm config images list --kubernetes-version $KUBE_VERSION --image-repository $KUBE_IMAGE_REGISTRY \
    | while read -r img; do img_name=$(echo "$img" | sed "s/$KUBE_IMAGE_REGISTRY\///g" | sed 's/[:\/]/_/g'); \
    skopeo copy docker://$img docker-archive:${DIR}/kubernetes/images/${img_name}.tar:$img; done

ARG CALICO_VERSION=3.28.0
ARG CALICO_URL=https://raw.githubusercontent.com/projectcalico/calico/v$CALICO_VERSION/manifests/calico.yaml
ARG CALICO_IMAGE_REGISTRY=docker.io
RUN mkdir -p ${DIR}/calico/images \
    && wget -nc -qO ${DIR}/calico/calico-${CALICO_VERSION}.yaml $CALICO_URL --no-check-certificate \
    && yq e '.spec.template.spec.containers[].image, .spec.template.spec.initContainers[].image' \
    ${DIR}/calico/calico-${CALICO_VERSION}.yaml | sort | uniq | grep -v "\---" \
    | while read -r img; do img_name=$(echo "$img" | sed "s/$CALICO_IMAGE_REGISTRY\///g" | sed 's/[:\/]/_/g'); \
    skopeo copy docker://$img docker-archive:${DIR}/calico/images/${img_name}.tar:$img; done

ARG ETCD_VERSION=3.5.14
ARG ETCD_URL="https://github.com/coreos/etcd/releases/download/v${ETCD_VERSION}/etcd-v${ETCD_VERSION}-linux-${TARGET_ARCH}.tar.gz"
RUN mkdir -p ${DIR}/etcd \
    && wget -nc -q -P ${DIR}/etcd $ETCD_URL

ARG HELM_VERSION=3.13.3
ARG HELM_URL="https://get.helm.sh/helm-v${HELM_VERSION}-linux-${TARGET_ARCH}.tar.gz"
RUN mkdir -p ${DIR}/helm \
    && wget -nc -q -P ${DIR}/helm $HELM_URL

RUN echo "kubernetes: $KUBE_VERSION" >> ${DIR}/versions \
    && echo "docker: $DOCKER_VERSION" >> ${DIR}/versions \
    && echo "calico: $CALICO_VERSION" >> ${DIR}/versions \
    && echo "etcd: $ETCD_VERSION" >> ${DIR}/versions \
    && echo "helm: $HELM_VERSION" >> ${DIR}/versions

RUN mkisofs -r -o ${OS_NAME}-${OS_VERSION}_kube-${KUBE_VERSION}_registry-${VERSION}.iso ${DIR}

FROM scratch
COPY --from=os /package/*.iso /
