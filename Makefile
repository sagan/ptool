.PHONY: help build docker-build docker-build-multi docker-run docker-push docker-test

# 变量定义
APP_NAME := ptool
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags="-w -s -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Docker 相关变量
DOCKER_IMAGE := $(APP_NAME)
DOCKER_TAG := latest

help: ## 显示帮助信息
	@echo "可用命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## 构建二进制文件
	@echo "Building $(APP_NAME)..."
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(APP_NAME) .
	@echo "Build complete: bin/$(APP_NAME)"

docker-build: ## 构建 Docker 镜像
	@echo "Building Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)"
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Docker image built successfully"

docker-build-multi: ## 构建多平台 Docker 镜像
	@echo "Building multi-platform Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)"
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		--push \
		.

docker-run: ## 运行 Docker 容器
	docker run --rm \
		-v ~/.config/ptool:/home/ptool/.config/ptool \
		$(DOCKER_IMAGE):$(DOCKER_TAG) \
		$(ARGS)

docker-push: ## 推送 Docker 镜像
	@echo "Pushing Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)"
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-test: docker-build ## 测试 Docker 镜像
	@echo "Testing Docker image..."
	docker run --rm $(DOCKER_IMAGE):$(DOCKER_TAG) version
