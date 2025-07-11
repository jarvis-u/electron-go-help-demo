package hosts

import (
	"fmt"
	"net"
	"os"
)

// 操作类型常量
const (
	OpGetHosts = byte('g')
	OpUpdate   = byte('u')
)

// HandleRequest 处理客户端请求
func HandleRequest(conn *net.UnixConn) {
	defer conn.Close()

	// 读取操作类型
	op := make([]byte, 1)
	if _, err := conn.Read(op); err != nil {
		sendError(conn, "读取操作类型失败: %v", err)
		return
	}

	switch op[0] {
	case OpGetHosts:
		handleGetHosts(conn)
	case OpUpdate:
		handleUpdateHosts(conn)
	default:
		sendError(conn, "未知操作类型: %c", op[0])
	}
}

// 处理获取hosts文件请求
func handleGetHosts(conn *net.UnixConn) {
	content, err := os.ReadFile("/etc/hosts")
	if err != nil {
		sendError(conn, "读取hosts文件失败: %v", err)
		return
	}

	if _, err := conn.Write(content); err != nil {
		fmt.Printf("发送hosts内容失败: %v\n", err)
	}
}

// 处理更新hosts文件请求
func handleUpdateHosts(conn *net.UnixConn) {
	// 读取新内容长度
	lenBuf := make([]byte, 4)
	if _, err := conn.Read(lenBuf); err != nil {
		sendError(conn, "读取内容长度失败: %v", err)
		return
	}
	contentLen := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])
	
	// 读取内容
	content := make([]byte, contentLen)
	if _, err := conn.Read(content); err != nil {
		sendError(conn, "读取内容失败: %v", err)
		return
	}

	// 验证内容有效性
	if len(content) == 0 {
		sendError(conn, "内容不能为空")
		return
	}

	// 写入hosts文件
	if err := os.WriteFile("/etc/hosts", content, 0644); err != nil {
		sendError(conn, "写入hosts文件失败: %v", err)
		return
	}

	// 返回成功
	conn.Write([]byte("SUCCESS"))
}

// 发送错误响应
func sendError(conn net.Conn, format string, args ...interface{}) {
	errMsg := fmt.Sprintf(format, args...)
	fmt.Println(errMsg)
	conn.Write([]byte("ERROR: " + errMsg))
}