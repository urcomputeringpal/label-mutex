# https://github.com/microsoft/vscode-dev-containers/tree/master/containers/go

FROM mcr.microsoft.com/devcontainers/go:1.21-bullseye@sha256:30f84595e36b3d5d67b12f1795f0e78777f4913cfc08d23d8cf51a281d3cdf14

RUN apt-get update -y && \
    apt-get install -y curl apt-transport-https ca-certificates gnupg && \
    echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list && \
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key --keyring /usr/share/keyrings/cloud.google.gpg add - && \
    apt-get update -y && apt-get install -y google-cloud-sdk

RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-$(uname -m).zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    sudo ./aws/install
