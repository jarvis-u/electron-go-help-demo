#!/bin/bash
set -e

#echo "编译Go二进制文件..."
#go build -o /usr/local/bin/hosts-helper ./go-service/cmd/service
cp ../hosts-helper /usr/local/bin/hosts-helper

echo "设置setuid权限..."
sudo chown root:wheel /usr/local/bin/hosts-helper
sudo chmod u+s /usr/local/bin/hosts-helper

echo "部署LaunchDaemon配置..."
sudo cp ../scripts/com.example.hostshelper.plist /Library/LaunchDaemons/
sudo chown root:wheel /Library/LaunchDaemons/com.example.hostshelper.plist

echo "加载服务..."
sudo launchctl load /Library/LaunchDaemons/com.example.hostshelper.plist

echo "✅ 服务部署完成"