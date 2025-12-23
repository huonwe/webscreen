.PHONY: all build-capturer build-main clean ci

# --- 变量定义 ---

# 获取当前目标 OS/ARCH
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Windows 下添加 .exe 后缀
EXE :=
ifeq ($(GOOS),windows)
	EXE := .exe
endif

# --- 路径配置 ---

# 1. Capturer 输出路径 (必须固定！为了配合 go:embed)
# 无论是在本地还是 CI，它都必须生成到源代码树的这个位置
CAPTURER_DIR := sdriver/xvfb/bin
CAPTURER_BIN := capturer_xvfb
CAPTURER_OUT := $(CAPTURER_DIR)/$(CAPTURER_BIN)

# 2. Main 程序输出路径 (可变)
# 默认为当前目录，CI 时会传入 dist
DIST_DIR ?= .
SUFFIX ?= 
MAIN_OUT := $(DIST_DIR)/webscreen$(SUFFIX)$(EXE)

# --- 构建目标 ---

# 默认目标
all: build-capturer build-main

# CI 入口
ci: all

# 构建 capturer
# 注意：即使是交叉编译，我们也将结果放在源码目录中，因为 embed 需要它
build-capturer:
	@echo ">> [1/2] Building Capturer for $(GOOS)/$(GOARCH)..."
	@echo "   Fixed Path (for embed): $(CAPTURER_OUT)"
	@mkdir -p $(CAPTURER_DIR)
	go build -v -o "$(CAPTURER_OUT)" ./capturer

# 构建主程序
# 依赖 build-capturer，确保 embed 文件存在
build-main: build-capturer
	@echo ">> [2/2] Building Main App for $(GOOS)/$(GOARCH)..."
	@echo "   Output: $(MAIN_OUT)"
	@mkdir -p $(dir $(MAIN_OUT))
	go build -v -o "$(MAIN_OUT)" .

# 清理
clean:
	rm -f webscreen* sdriver/xvfb/bin/capturer_xvfb*
	rm -rf dist