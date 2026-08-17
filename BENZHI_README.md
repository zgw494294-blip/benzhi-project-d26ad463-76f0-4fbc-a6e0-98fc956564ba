# BENZHI_README

## 项目说明
- 项目：benzhi-project-d26ad463-76f0-4fbc-a6e0-98fc956564ba
- 项目用途：PatchProof is implemented with draft/active/closed lifecycle management, endpoint reservation, immutable receipts, atomic versioned JSON persistence, CLI commands, tests, documentation, and a bounded temporary-directory smoke workflow. Go 1.22.0 is pinned. Acceptance commands passed.
- Go 工具链：`golang:1.22.0`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/patchproof --help
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-d26ad463-76f0-4fbc-a6e0-98fc956564ba-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-d26ad463-76f0-4fbc-a6e0-98fc956564ba-arm64 linux/arm64
docker run -it benzhi-project-d26ad463-76f0-4fbc-a6e0-98fc956564ba-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/patchproof smoke`
