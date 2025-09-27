FROM mcr.microsoft.com/devcontainers/go:2-1.25-bookworm

ENV TASK_VERSION=v3.45.4 \
    GOLANG_CI_LINT_VERSION=v2.4.0 \
    GITHUB_CLI_VERSION=2.74.0 \
    NODE_VERSION=lts \
    SEMANTIC_RELEASE_VERSION=v24.2.9

RUN apt-get update && \
    # Determine architecture
    architecture=$(uname -m); \
    architecture_pretty="arm64"; \
    if [ "$architecture" = "x86_64" ]; then \
    architecture_pretty="amd64"; \
    fi && \
    echo "Installing depenenices for ${architecture}" && \
    # Install networking utilities
    apt-get install -y file iputils-ping dnsutils && \
    # Install Node.js for Spectral
    curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash - && \
    apt-get install -y nodejs && \
    # Install Task
    curl -s https://taskfile.dev/install.sh | sh -s -- -b /usr/local/bin ${TASK_VERSION} && \
    # Install golangci-lint
    curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin ${GOLANG_CI_LINT_VERSION} && \
    # Install GitHub CLI
    curl -fsSL https://github.com/cli/cli/releases/download/v${GITHUB_CLI_VERSION}/gh_${GITHUB_CLI_VERSION}_linux_${architecture_pretty}.tar.gz -o /tmp/gh_${GITHUB_CLI_VERSION}_linux_${architecture_pretty}.tar.gz && \
    tar -xzf /tmp/gh_${GITHUB_CLI_VERSION}_linux_${architecture_pretty}.tar.gz -C /tmp && \
    chmod +x /tmp/gh_${GITHUB_CLI_VERSION}_linux_${architecture_pretty}/bin/gh && \
    mv /tmp/gh_${GITHUB_CLI_VERSION}_linux_${architecture_pretty}/bin/gh /usr/local/bin/gh && \
    # Install semantic-release and plugins
    npm install -g semantic-release@${SEMANTIC_RELEASE_VERSION} \
                   @semantic-release/changelog \
                   @semantic-release/git \
                   @semantic-release/github \
                   conventional-changelog-cli \
                   conventional-changelog-conventionalcommits && \
    # Install js-yaml for converting JSON RPC schema to YAML
    npm i -g js-yaml@4.1.0 && \
    # Install Claude code
    npm i -g @anthropic-ai/claude-code && \
    # Install prettier
    npm i -g prettier && \
    # Clean up
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

USER vscode

RUN go install github.com/inference-gateway/tools/cmd/generator@v0.1.1

# Use Powerlevel10k theme
RUN git clone --depth=1 https://github.com/romkatv/powerlevel10k.git /home/vscode/.powerlevel10k

# Configure Powerlevel10k
RUN echo 'source /home/vscode/.powerlevel10k/powerlevel10k.zsh-theme' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_DISABLE_CONFIGURATION_WIZARD=true' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_LEFT_PROMPT_ELEMENTS=(dir vcs)' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_RIGHT_PROMPT_ELEMENTS=(command_execution_time status)' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_COMMAND_EXECUTION_TIME_THRESHOLD=0' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_COMMAND_EXECUTION_TIME_PRECISION=2' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_COMMAND_EXECUTION_TIME_FORMAT="duration"' >> /home/vscode/.zshrc


# Shell completion
RUN echo "source <(gh completion -s zsh)" >> /home/vscode/.zshrc && \
    echo "source <(task --completion zsh)" >> /home/vscode/.zshrc
