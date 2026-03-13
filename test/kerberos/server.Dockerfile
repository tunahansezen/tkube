FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y krb5-kdc krb5-admin-server openssh-server ssh
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

RUN useradd -m -s /bin/bash admin && echo "admin:secret" | chpasswd

# Client bileti üretiliyor ve shared klasöre konuyor
RUN echo '#!/bin/bash\n\
kdb5_util create -s -P mastersecret\n\
kadmin.local -q "addprinc -pw secret admin"\n\
kadmin.local -q "addprinc -randkey host/server.test.local"\n\
kadmin.local -q "ktadd host/server.test.local"\n\
kadmin.local -q "addprinc -randkey host/client.test.local"\n\
kadmin.local -q "ktadd -k /shared/client.keytab host/client.test.local"\n\
krb5kdc\n\
kadmind\n\
/usr/sbin/sshd -D' > /start.sh && chmod +x /start.sh

CMD ["/start.sh"]