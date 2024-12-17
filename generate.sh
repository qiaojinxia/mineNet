#!/bin/bash

# 设置变量
PLUGIN_NAME="protoc-gen-msg-type"
PROTO_ROOT="./proto"  # proto文件的根目录

# 编译插件
echo "Building plugin..."
go build -o $PLUGIN_NAME proto_generate.go

# 查找所有proto文件
PROTO_FILES=$(find $PROTO_ROOT -name "*.proto")

# 生成代码
echo "Generating code..."
protoc --plugin=protoc-gen-msg-type=./$PLUGIN_NAME \
       --msg-type-out=. \
       --proto_path=$PROTO_ROOT \
       "$PROTO_FILES"


# 检查结果
# shellcheck disable=SC2181
if [ $? -eq 0 ]; then
    echo "Code generation successful"
    echo "Generated files:"
    ls -l msgtype.go
    mv ./msg-type.go ./proto/
else
    echo "Code generation failed"
    exit 1
fi