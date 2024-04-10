FROM fedora
WORKDIR /app

ENV OC_URL=https://github.com/openshift/origin/releases/download/v3.11.0/openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit.tar.gz

RUN sudo dnf install -y jq units stress stress-ng procps-ng bc
RUN curl -L "$OC_URL" -o /tmp/oc.tar.gz && \
  mkdir /tmp/oc && \
  tar xf /tmp/oc.tar.gz -C /tmp/oc && \
  mv /tmp/oc/*/oc /usr/local/bin && \
  rm -rf /tmp/oc*


ADD contrib/* /app
ENTRYPOINT bash /app/agent.sh
