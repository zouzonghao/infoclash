#!/bin/bash

# 遇到错误时立即停止脚本执行
set -e

# --- 前端构建 ---
echo "正在构建前端..."
# 进入前端项目目录
cd frontend
# 安装前端依赖
npm install
# 运行前端构建命令
npm run build
# 返回到项目根目录
cd ..
echo "前端构建完成。"

# --- 后端构建 ---
echo "正在构建后端..."
# 复制前端构建产物到后端目录，以便嵌入后端应用
echo "正在复制前端产物到后端..."
cp -R ./frontend/dist ./backend/dist

# 进入后端项目目录，构建Go应用。
cd backend
# 使用Go构建命令，生成可执行文件到上级目录，命名为 infoclash。
# -ldflags="-s -w" 选项用于剥离调试信息和符号表，以减小生成二进制文件的大小。
go build -ldflags="-s -w" -o ../infoclash .
# 返回到项目根目录
cd ..

# 清理复制到后端目录的前端产物，因为它们已经被嵌入到Go二进制文件中了。
rm -rf ./backend/dist
echo "后端构建完成。"

echo "构建成功！请运行应用程序：./infoclash"
