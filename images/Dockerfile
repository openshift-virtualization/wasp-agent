FROM fedora
WORKDIR /app

RUN sudo dnf install -y cargo jq units stress stress-ng
RUN  curl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o /usr/local/bin/kubectl && chmod a+x /usr/local/bin/kubectl
#ADD Cargo.* /app
#ADD src/ /app/src/
#RUN cargo build
#ENTRYPOINT cargo run

ADD contrib/* /app
ENTRYPOINT bash /app/agent.sh
