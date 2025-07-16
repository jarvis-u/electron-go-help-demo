package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strings"
	"syscall"
	"time"
)

const socketPath = "/var/run/com.example.ktctlhelper.sock"

const plistContent = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.example.ktctlhelper</string>

    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/ktctl-helper</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>StandardOutPath</key>
    <string>/var/log/ktctl-helper.log</string>

    <key>StandardErrorPath</key>
    <string>/var/log/ktctl-helper-error.log</string>

    <key>Sockets</key>
    <dict>
        <key>HostsHelperSocket</key>
        <dict>
            <key>SockPathName</key>
            <string>/var/run/com.example.ktctlhelper.sock</string>
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

var lastCmdPID int

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

	runService()
}

func runService() {
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
	return true
}

func handleRequest(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1)
	_, err := conn.Read(buf)
	if err != nil {
		return
	}

	switch buf[0] {
	case 'c':
		lenBuf := make([]byte, 4)
		conn.Read(lenBuf)
		length := binary.BigEndian.Uint32(lenBuf)
		command := make([]byte, length)
		conn.Read(command)
		cmdStr := string(command)
		fmt.Printf("执行命令: %s\n", cmdStr)

		logPath := fmt.Sprintf("/tmp/ktctl_%d.log", time.Now().UnixNano())

		absCmdStr := strings.Replace(cmdStr, "ktctl", "/usr/local/bin/ktctl", 1)
		cmd := exec.Command("/bin/sh", "-c", absCmdStr)

		usr, err := user.Current()
		if err == nil {
			cmd.Env = append(os.Environ(), "HOME="+usr.HomeDir)
			fmt.Println("设置HOME环境变量:", usr.HomeDir)
		} else {
			fmt.Println("无法获取用户主目录:", err)
		}

		fmt.Println("转换后的命令:", absCmdStr)

		// 创建日志文件
		logFile, err := os.Create(logPath)
		if err != nil {
			fmt.Printf("创建日志文件失败: %v\n", err)
			response := "命令启动失败: 无法创建日志文件"
			lenBuf = make([]byte, 4)
			binary.BigEndian.PutUint32(lenBuf, uint32(len(response)))
			conn.Write(lenBuf)
			conn.Write([]byte(response))
			return
		}
		defer logFile.Close()

		cmd.Stdout = logFile
		cmd.Stderr = logFile

		if err := cmd.Start(); err != nil {
			fmt.Printf("命令启动失败: %v\n", err)
			response := "命令启动失败: " + err.Error()
			lenBuf = make([]byte, 4)
			binary.BigEndian.PutUint32(lenBuf, uint32(len(response)))
			conn.Write(lenBuf)
			conn.Write([]byte(response))
			return
		}

		time.Sleep(200 * time.Millisecond)
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			logFile.Seek(0, 0)
			logContent, _ := io.ReadAll(logFile)

			fmt.Printf("命令立即退出，退出码: %d\n", cmd.ProcessState.ExitCode())
			response := fmt.Sprintf("命令执行失败 (退出码: %d)\n日志: %s",
				cmd.ProcessState.ExitCode(),
				strings.ReplaceAll(string(logContent), "\n", " "))

			lenBuf = make([]byte, 4)
			binary.BigEndian.PutUint32(lenBuf, uint32(len(response)))
			conn.Write(lenBuf)
			conn.Write([]byte(response))
			return
		}

		fmt.Printf("命令已启动 PID: %d, 日志: %s\n", cmd.Process.Pid, logPath)

		go func() {
			err := cmd.Wait()
			if err != nil {
				fmt.Printf("命令执行失败: %v\n", err)
				logFile.WriteString(fmt.Sprintf("\n命令执行失败: %v", err))
			}
		}()

		lastCmdPID = cmd.Process.Pid

		fmt.Printf("命令已启动 PID: %d, 日志: %s\n", cmd.Process.Pid, logPath)

		response := fmt.Sprintf("命令已在后台运行 (PID: %d)\n日志文件: %s", cmd.Process.Pid, logPath)
		lenBuf = make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(response)))
		conn.Write(lenBuf)
		conn.Write([]byte(response))

	case 'd':
		if lastCmdPID == 0 {
			conn.Write([]byte("没有正在运行的后台进程"))
			return
		}

		process, err := os.FindProcess(lastCmdPID)
		if err != nil {
			response := fmt.Sprintf("找不到进程 %d: %v", lastCmdPID, err)
			conn.Write([]byte(response))
			return
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			response := fmt.Sprintf("发送终止信号失败: %v", err)
			conn.Write([]byte(response))
			return
		}

		response := fmt.Sprintf("已发送终止信号到进程 %d", lastCmdPID)
		conn.Write([]byte(response))
		lastCmdPID = 0 // 重置PID

	}
}

func installService() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	targetPath := "/usr/local/bin/ktctl-helper"
	if err := copyFile(exePath, targetPath); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	if err := setPermissions(targetPath); err != nil {
		return err
	}

	plistDest := "/Library/LaunchDaemons/com.example.hostshelper.plist"
	if err := os.WriteFile(plistDest, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("写入plist文件失败: %w", err)
	}
	if err := exec.Command("chown", "root:wheel", plistDest).Run(); err != nil {
		return fmt.Errorf("设置plist权限失败: %w", err)
	}

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
