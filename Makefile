# go-autopcr 构建脚手架
# 硬性约束：禁用 CGO（CGO_ENABLED=0），且不依赖任何使用 CGO 的包，保证跨平台可移植。

MODULE  := github.com/cca2878/go-autopcr
BIN_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X '$(MODULE)/internal/buildinfo.Version=$(VERSION)' \
	-X '$(MODULE)/internal/buildinfo.Commit=$(COMMIT)' \
	-X '$(MODULE)/internal/buildinfo.Date=$(DATE)'

# 全局强制禁用 CGO。
export CGO_ENABLED := 0

.PHONY: all build cli datagen test vet tidy lint check-cgo clean

all: build

build: cli datagen

cli:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/autopcr-cli ./cmd/autopcr-cli

datagen:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/datagen ./cmd/datagen

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# 校验最终二进制未链接 CGO（读取二进制内嵌的构建设置）。
check-cgo: cli
	@echo ">> 校验 CGO_ENABLED=0 ..."
	@if go version -m $(BIN_DIR)/autopcr-cli | grep -q 'CGO_ENABLED=0'; then \
		echo "OK: 未启用 CGO"; \
	else \
		echo "ERROR: 检测到 CGO 或无法确认"; exit 1; \
	fi

# 若安装了 golangci-lint 则运行，否则跳过（不作为硬性依赖）。
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint 未安装，跳过"; \
	fi

clean:
	rm -rf $(BIN_DIR)
