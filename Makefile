.PHONY: all build-capturer build-main clean ci

# --- 变量定义 ---

#  export PATH=$PATH:/usr/local/go/bin
PATH := /usr/local/go/bin:$(PATH)
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

# 1. Recorder 输出路径 (必须固定！为了配合 go:embed)
RECORDER_DIR := sdriver/linux/bin
RECORDER_BIN := recorder

RECORDER_OUT := $(RECORDER_DIR)/$(RECORDER_BIN)
# 2. Main 程序输出路径 (可变)
DIST_DIR ?= .
SUFFIX ?= 
MAIN_OUT := $(DIST_DIR)/webscreen$(SUFFIX)$(EXE)

# --- 构建目标 ---

all: build-LinuxRecorder build-main

ci: all

# 构建 capturer
build-LinuxRecorder:
	@echo ">> [1/2] Building LinuxRecorder for $(GOOS)/$(GOARCH)..."
	@echo "   Output Directory (for embed): $(RECORDER_DIR)"
	@mkdir -p $(RECORDER_DIR)
# 	go build -v -ldflags "$(LDFLAGS)" -o "$(RECORDER_XVFB_OUT)" ./linuxRecorder/xvfb
# 	go build -v -ldflags "$(LDFLAGS)" -o "$(RECORDER_XORG_OUT)" ./linuxRecorder/xorg
# 	go build -v -ldflags "$(LDFLAGS)" -o "$(RECORDER_WL_OUT)" ./linuxRecorder/wf-recorder
	go build -v -ldflags "$(LDFLAGS)" -o "$(RECORDER_OUT)" ./linuxRecorder

# 构建主程序
build-main: build-LinuxRecorder
	@echo ">> [2/2] Building Main App for $(GOOS)/$(GOARCH)..."
	@echo "   LDFLAGS: $(LDFLAGS)"
	@echo "   Output: $(MAIN_OUT)"
	@mkdir -p $(dir $(MAIN_OUT))
	go build -v -ldflags "$(LDFLAGS)" -o "$(MAIN_OUT)" .

# 清理
clean:
	rm -f webscreen* $(RECORDER_DIR)/*
	rm -rf dist
