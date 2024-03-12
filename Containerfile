FROM fedora
WORKDIR /app

RUN sudo dnf install -y jq units stress stress-ng procps-ng
RUN  curl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o /usr/local/bin/kubectl && chmod a+x /usr/local/bin/kubectl

ADD contrib/* /app
ENTRYPOINT bash /app/agent.sh
