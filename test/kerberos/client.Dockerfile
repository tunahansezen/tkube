FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y krb5-user openssh-server ssh sudo
RUN mkdir /var/run/sshd

RUN echo '[libdefaults]\n\
    default_realm = TEST.LOCAL\n\
    rdns = false\n\
[realms]\n\
    TEST.LOCAL = {\n\
        kdc = server.test.local\n\
        admin_server = server.test.local\n\
    }\n\
[domain_realm]\n\
    .test.local = TEST.LOCAL\n\
    server.test.local = TEST.LOCAL\n\
    client.test.local = TEST.LOCAL' > /etc/krb5.conf

RUN echo "GSSAPIAuthentication yes" >> /etc/ssh/sshd_config && \
    echo "GSSAPICleanupCredentials yes" >> /etc/ssh/sshd_config && \
    echo "GSSAPIStrictAcceptorCheck no" >> /etc/ssh/sshd_config

RUN useradd -m -G sudo -s /bin/bash admin && echo "admin:secret" | chpasswd

# Sunucunun bileti bırakmasını bekler, alır ve SSH'ı başlatır
RUN echo '#!/bin/bash\n\
while [ ! -f /shared/client.keytab ]; do sleep 1; done\n\
cp /shared/client.keytab /etc/krb5.keytab\n\
/usr/sbin/sshd -D' > /start.sh && chmod +x /start.sh

CMD ["/start.sh"]