# go-autopcr-core 构建脚手架
# 硬性约束：禁用 CGO（CGO_ENABLED=0），且不依赖任何使用 CGO 的包，保证跨平台可移植。
#
# 本仓是【纯库】，无任何可执行产物：命令行外壳在 autopcr-cli 仓，母数据解包已内化进
# internal/client/masterdata（登录与 RefreshMasterdata 共用 Manager.EnsureDB）。

BIN_DIR := bin

# CGO 校验用的临时测试二进制（见 check-cgo）。
CGO_PROBE := $(BIN_DIR)/cgocheck.test

# 内嵌 rainbow 的落地路径（见 rainbow target）。
RAINBOW_DST := internal/client/rainbow.json.gz

# 全局强制禁用 CGO。
export CGO_ENABLED := 0

.PHONY: all build test vet tidy lint check-cgo clean rainbow

all: build

# 库没有可链接的产物，build 即「所有包在 CGO_ENABLED=0 下可编译」——这正是库该保证的那
# 一半（消费者最终怎么链接是消费者的事）。依赖若需要 CGO，这里就会失败。
build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# 校验 CGO 确实禁用（读取二进制内嵌的构建设置）。本仓无可执行产物，故编译一个【测试
# 二进制】来读：它是真产物（连链接期问题一并暴露，强于 go build ./...），且 masterdata
# 恰是链接 sqlite 的地方——sqlite 与 lz4 是最可能引入 CGO 的两个依赖。
check-cgo:
	@echo ">> 校验 CGO_ENABLED=0 ..."
	@mkdir -p $(BIN_DIR)
	@go test -c -o $(CGO_PROBE) ./internal/client/masterdata
	@if go version -m $(CGO_PROBE) | grep -q 'CGO_ENABLED=0'; then \
		echo "OK: 未启用 CGO"; \
		rm -f $(CGO_PROBE); \
	else \
		echo "ERROR: 检测到 CGO 或无法确认"; rm -f $(CGO_PROBE); exit 1; \
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

# 重新生成内嵌的 rainbow.json.gz：minify + gzip 源文件，落到 RAINBOW_DST。
# 用法：make rainbow SRC=/path/to/rainbow.json
rainbow:
	@if [ -z "$(SRC)" ]; then echo "用法: make rainbow SRC=/path/to/rainbow.json"; exit 1; fi
	go run internal/client/rainbow_gen.go $(SRC) $(RAINBOW_DST)
