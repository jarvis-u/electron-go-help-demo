package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)

const socketPath = "/var/run/com.example.hostshelper.sock"

func verifyClient(conn *net.UnixConn) bool {
	// 简化验证：始终返回true（实际项目需实现真实验证）
	return true
}

func handleRequest(conn net.Conn) {
	defer conn.Close()
	// 读取命令
	buf := make([]byte, 1)
	_, err := conn.Read(buf)
	if err != nil {
		return
	}

	switch buf[0] {
	case 'g': // 获取hosts内容
		content, _ := os.ReadFile("/etc/hosts")
		conn.Write(content)
	case 'u': // 更新hosts
		// 读取内容长度
		lenBuf := make([]byte, 4)
		conn.Read(lenBuf)
		length := binary.BigEndian.Uint32(lenBuf)

		// 读取内容
		content := make([]byte, length)
		conn.Read(content)

		// 写入/etc/hosts（需要sudo权限）
		os.WriteFile("/etc/hosts", content, 0644)
	}
}

func main() {
	// 清理可能存在的旧socket文件
	os.Remove(socketPath)

	// 创建Unix socket监听
	listener, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: socketPath,
		Net:  "unix",
	})
	if err != nil {
		fmt.Printf("Socket creation failed: %v\n", err)
		os.Exit(1)
	}

	// 设置socket文件权限为0666
	if err := os.Chmod(socketPath, 0666); err != nil {
		fmt.Printf("Failed to set socket permissions: %v\n", err)
		os.Exit(1)
	}
	if err != nil {
		fmt.Printf("Socket creation failed: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(socketPath)

	fmt.Println("Hosts helper service started. Listening on", socketPath)
	fmt.Println("Hosts helper服务已启动，PID:", os.Getpid())

	// 处理信号，优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		listener.Close()
	}()

	// 主循环接受连接
	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			fmt.Println("Accept error:", err)
			continue
		}
		fmt.Println("收到新连接")

		// 验证客户端身份
		if !verifyClient(conn) {
			conn.Close()
			fmt.Println("Client verification failed")
			continue
		}

		go handleRequest(conn)
	}
}

func runInstall() error {
	// 获取当前可执行文件绝对路径
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	absPath, err := filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("获取绝对路径失败: %w", err)
	}

	// 复制到目标位置
	targetPath := "/usr/local/bin/hosts-helper"
	cmd := exec.Command("sudo", "cp", absPath, targetPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	// 设置权限
	chownCmd := exec.Command("sudo", "chown", "root:wheel", targetPath)
	chownCmd.Stdout = os.Stdout
	chownCmd.Stderr = os.Stderr
	if err := chownCmd.Run(); err != nil {
		return fmt.Errorf("设置所有者失败: %w", err)
	}

	setuidCmd := exec.Command("sudo", "chmod", "u+s", targetPath)
	setuidCmd.Stdout = os.Stdout
	setuidCmd.Stderr = os.Stderr
	if err := setuidCmd.Run(); err != nil {
		return fmt.Errorf("设置权限失败: %w", err)
	}

	// 创建plist配置文件
	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
    <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
    <plist version="1.0">
    <dict>
       <key>Label</key>
       <string>com.example.hostshelper</string>
       <key>ProgramArguments</key>
       <array>
          <string>/usr/local/bin/hosts-helper</string>
       </array>
       <key>RunAtLoad</key>
       <true/>
       <key>KeepAlive</key>
       <true/>
       <key>StandardErrorPath</key>
       <string>/var/log/hosts-helper.log</string>
       <key>StandardOutPath</key>
       <string>/var/log/hosts-helper.log</string>
    </dict>
    </plist>`

	plistPath := "/Library/LaunchDaemons/com.example.hostshelper.plist"
	plistCmd := exec.Command("sudo", "sh", "-c", fmt.Sprintf("echo '%s' > %s", plistContent, plistPath))
	plistCmd.Stdout = os.Stdout
	plistCmd.Stderr = os.Stderr
	if err := plistCmd.Run(); err != nil {
		return fmt.Errorf("写入plist失败: %w", err)
	}

	// 加载服务
	loadCmd := exec.Command("sudo", "launchctl", "load", plistPath)
	loadCmd.Stdout = os.Stdout
	loadCmd.Stderr = os.Stderr
	if err := loadCmd.Run(); err != nil {
		return fmt.Errorf("加载服务失败: %w", err)
	}

	// 启动服务
	startCmd := exec.Command("sudo", "launchctl", "start", "com.example.hostshelper")
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	if err := startCmd.Run(); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}

	// 验证安装结果
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return fmt.Errorf("安装验证失败: 目标文件不存在")
	}

	return nil
}
