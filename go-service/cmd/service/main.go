package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

const socketPath = "/var/run/com.example.hostshelper.sock"

const plistContent = `<?xml version="1.0" encoding="UTF-8"?>
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
    
    <key>StandardOutPath</key>
    <string>/var/log/hosts-helper.log</string>
    
    <key>StandardErrorPath</key>
    <string>/var/log/hosts-helper-error.log</string>
    
    <key>Sockets</key>
    <dict>
        <key>HostsHelperSocket</key>
        <dict>
            <key>SockPathName</key>
            <string>/var/run/com.example.hostshelper.sock</string>
            <key>SockPathMode</key>
            <integer>384</integer> <!-- 0600 permissions in decimal -->
        </dict>
    </dict>
    
    <key>UserName</key>
    <string>root</string>
    
    <key>GroupName</key>
    <string>wheel</string>
    
    <key>SessionCreate</key>
    <true/>
</dict>
</plist>`

func main() {
	if len(os.Args) > 1 && os.Args[1] == "install" {
		fmt.Println("服务开始安装")
		if err := installService(); err != nil {
			fmt.Printf("安装失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ 服务部署完成")
		return
	}

	// 原始服务逻辑
	runService()
}

func runService() {
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

func installService() error {
	// 1. 获取当前运行的可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	// 2. 复制到 /usr/local/bin/hosts-helper
	targetPath := "/usr/local/bin/hosts-helper"
	if err := copyFile(exePath, targetPath); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	// 3. 设置权限
	if err := setPermissions(targetPath); err != nil {
		return err
	}

	// 4. 部署LaunchDaemon配置
	plistDest := "/Library/LaunchDaemons/com.example.hostshelper.plist"
	// 直接写入内置的plist内容
	if err := os.WriteFile(plistDest, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("写入plist文件失败: %w", err)
	}
	if err := exec.Command("chown", "root:wheel", plistDest).Run(); err != nil {
		return fmt.Errorf("设置plist权限失败: %w", err)
	}

	// 5. 加载服务
	fmt.Printf("加载服务: %s\n", plistDest)
	cmdLoad := exec.Command("launchctl", "load", plistDest)
	if output, err := cmdLoad.CombinedOutput(); err != nil {
		fmt.Printf("加载服务失败: %v\n输出: %s\n", err, string(output))
		return fmt.Errorf("加载服务失败: %w", err)
	} else {
		fmt.Printf("服务加载成功\n")
	}

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func setPermissions(path string) error {
	cmdChown := exec.Command("sudo", "chown", "root:wheel", path)
	if err := cmdChown.Run(); err != nil {
		return fmt.Errorf("chown失败: %w", err)
	}

	cmdChmod := exec.Command("sudo", "chmod", "u+s", path)
	if err := cmdChmod.Run(); err != nil {
		return fmt.Errorf("chmod失败: %w", err)
	}

	return nil
}
