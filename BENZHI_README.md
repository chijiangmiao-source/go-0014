基于 Go 实现的高压蒸汽灭菌热穿透校核 Web 项目，一款后端服务，完成探头数据冻结、暴露段判定、杀灭效力积分与确定性证据定版。

# steam-sterilization-thermal-validation

本 Git 项目来自模型完成任务后的 workspace，不包含嵌套 .git 记录或本地构建产物。

## 本地构建与测试

```bash
go mod download
npm --prefix frontend ci
npm --prefix frontend run build
go build ./...
go test ./...
./run_benzhi_smoke.sh
```

## Docker 构建与运行

```bash
docker build --platform linux/amd64 -t steam-sterilization-thermal-validation:latest .
./build_benzhi_docker.sh steam-sterilization-thermal-validation linux/arm64
docker run --rm -it --platform linux/arm64 steam-sterilization-thermal-validation:latest
./build_benzhi_docker.sh steam-sterilization-thermal-validation linux/amd64
docker run --rm -it --platform linux/amd64 steam-sterilization-thermal-validation:latest
```

构建脚本第二个参数为目标平台，必须分别完成 linux/arm64 和 linux/amd64 构建与容器验证；未提供时按照规范默认使用 linux/amd64。系统 frontend-v2 模板通过 Go 原生交叉编译生成目标架构的 /usr/local/bin/benzhi-app，镜像默认直接运行该入口。
