ARG OS_NAME=ubuntu
ARG OS_VERSION=22.04
FROM $OS_NAME:$OS_VERSION AS os
ARG OS_NAME
ARG OS_VERSION
ARG VERSION=0.0.0
ARG TARGET_ARCH=amd64
ARG OS_RELEASE=jammy
ARG DIR=${OS_NAME}-${OS_VERSION}-${TARGET_ARCH}-debs
ARG DEBIAN_FRONTEND=noninteractive
ENV TZ=Etc/UTC
ARG PKGS=.common[],.$OS_NAME[]
ARG MANDATORY_PACKAGES="tzdata apt-transport-https software-properties-common ca-certificates curl wget gnupg dpkg-dev genisoimage"

RUN apt update -qq \
    && apt install -y --no-install-recommends $MANDATORY_PACKAGES

WORKDIR /package
COPY iso/packages.yaml .

COPY --from=mikefarah/yq:4.44.1 /usr/bin/yq /usr/bin/yq
RUN yq eval "${PKGS}" packages.yaml | xargs apt install -y --no-install-recommends

ARG DOCKER_VERSION=20.10.24
ARG DOCKER_REPO_KEY="https://download.docker.com/linux/ubuntu/gpg"
ARG DOCKER_REPO_ADDRESS="https://download.docker.com/linux/ubuntu $OS_RELEASE stable"
RUN curl -fsSL $DOCKER_REPO_KEY | apt-key add -qq - \
    && echo "deb [arch=$TARGET_ARCH] $DOCKER_REPO_ADDRESS" > /etc/apt/sources.list.d/docker.list \
    && apt update -qq \
    && DOCKER_EXACT_VERSION=$(apt list -a docker-ce 2>/dev/null | cut -d '[' -f1 | grep ${DOCKER_VERSION} | head -1 | xargs | cut -d ' ' -f2) \
    && if [ -z "$DOCKER_EXACT_VERSION" ]; then echo "Docker not found with version $DOCKER_VERSION"; exit 1; fi \
    && apt install -y --no-install-recommends docker-ce=$DOCKER_EXACT_VERSION docker-ce-cli=$DOCKER_EXACT_VERSION

ARG KUBE_VERSION=1.30.2
ARG KUBE_MAJOR_VERSION=1.30
ARG KUBE_REPO_KEY="https://pkgs.k8s.io/core:/stable:/v$KUBE_MAJOR_VERSION/deb/Release.key"
ARG KUBE_REPO_ADDRESS="https://pkgs.k8s.io/core:/stable:/v$KUBE_MAJOR_VERSION/deb/ /"
RUN mkdir -p /etc/apt/keyrings \
    && curl -fsSL $KUBE_REPO_KEY | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg \
    && echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] $KUBE_REPO_ADDRESS" > /etc/apt/sources.list.d/kubernetes.list \
    && apt update -qq \
    && KUBE_EXACT_VERSION=$(apt list -a kubelet 2>/dev/null | cut -d '[' -f1 | grep ${KUBE_VERSION} | head -1 | xargs | cut -d ' ' -f2) \
    && if [ -z "$KUBE_EXACT_VERSION" ]; then echo "Kubernetes not found with version $KUBE_VERSION"; exit 1; fi \
    && apt install -y --no-install-recommends kubelet=$KUBE_EXACT_VERSION kubeadm=$KUBE_EXACT_VERSION kubectl=$KUBE_EXACT_VERSION

ARG REPO_DIR=$DIR/repo
RUN dpkg-query -W -f='${binary:Package}=${Version}\n' > packages.list \
    && sort -u packages.list | xargs apt-get install --yes --reinstall --print-uris | awk -F "'" '{print $2}' | grep -v '^$' | sort -u > packages.urls \
    && mkdir -p ${REPO_DIR} \
    && wget -q -x -P ${REPO_DIR} -i packages.urls \
    && cd ${REPO_DIR} \
    && dpkg-scanpackages ./ /dev/null | gzip -9c > ./Packages.gz

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

RUN genisoimage -r -o ${OS_NAME}-${OS_VERSION}_kube-${KUBE_VERSION}_registry-${VERSION}.iso ${DIR}

FROM scratch
COPY --from=os /package/*.iso /
