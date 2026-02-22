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

# --- Android 特殊处理 ---
# 解决 Go 1.23+ 在 Android/Termux 编译时对 linkname 检查过严的问题
LDFLAGS := -s -w
ifeq ($(GOOS),android)
	LDFLAGS += -checklinkname=0
endif

# --- 路径配置 ---

# 1. Capturer 输出路径 (必须固定！为了配合 go:embed)
CAPTURER_DIR := sdriver/xvfb/bin
CAPTURER_BIN := capturer_xvfb
CAPTURER_OUT := $(CAPTURER_DIR)/$(CAPTURER_BIN)

# 2. Main 程序输出路径 (可变)
DIST_DIR ?= .
SUFFIX ?= 
MAIN_OUT := $(DIST_DIR)/webscreen$(SUFFIX)$(EXE)

# --- 构建目标 ---

all: build-capturer build-main

ci: all

# 构建 capturer
build-capturer:
	@echo ">> [1/2] Building Capturer for $(GOOS)/$(GOARCH)..."
	@echo "   Fixed Path (for embed): $(CAPTURER_OUT)"
	@mkdir -p $(CAPTURER_DIR)
	go build -v -ldflags "$(LDFLAGS)" -o "$(CAPTURER_OUT)" ./capturer

# 构建主程序
build-main: build-capturer
	@echo ">> [2/2] Building Main App for $(GOOS)/$(GOARCH)..."
	@echo "   LDFLAGS: $(LDFLAGS)"
	@echo "   Output: $(MAIN_OUT)"
	@mkdir -p $(dir $(MAIN_OUT))
	go build -v -ldflags "$(LDFLAGS)" -o "$(MAIN_OUT)" .

# 清理
clean:
	rm -f webscreen* sdriver/xvfb/bin/capturer_xvfb*
	rm -rf dist
