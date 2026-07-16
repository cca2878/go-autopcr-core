# go-autopcr-core 构建脚手架
# 硬性约束：禁用 CGO（CGO_ENABLED=0），且不依赖任何使用 CGO 的包，保证跨平台可移植。
#
# 本仓是【库】：唯一的可执行产物是 datagen（母数据解包开发工具，只用 internal/ 故必须留在
# 仓内）。命令行外壳已分出去独立仓（autopcr-cli），故这里没有 cli 目标。

BIN_DIR := bin

# 全局强制禁用 CGO。
export CGO_ENABLED := 0

.PHONY: all build datagen test vet tidy lint check-cgo clean

all: build

build: datagen

datagen:
	go build -trimpath -o $(BIN_DIR)/datagen ./cmd/datagen

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# 校验最终二进制未链接 CGO（读取二进制内嵌的构建设置）。挂在 datagen 上——它是本仓唯一的
# 可执行产物，且吃下了 sqlite/lz4 这两个最可能引入 CGO 的依赖，故足以守住这条约束。
check-cgo: datagen
	@echo ">> 校验 CGO_ENABLED=0 ..."
	@if go version -m $(BIN_DIR)/datagen | grep -q 'CGO_ENABLED=0'; then \
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
