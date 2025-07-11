package auth

import (
	"net"
	"os"
	"syscall"
)

// VerifyClient 验证客户端身份
func VerifyClient(conn *net.UnixConn) bool {
	// 获取客户端凭证
	cred, err := getClientCredentials(conn)
	if err != nil {
		return false
	}

	// 验证UID匹配当前用户
	currentUID := os.Getuid()
	return cred.Uid == uint32(currentUID)
}

// 获取客户端凭证
func getClientCredentials(conn *net.UnixConn) (*syscall.Ucred, error) {
	// 获取原始文件描述符
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return nil, err
	}

	var cred *syscall.Ucred
	var credErr error
	err = rawConn.Control(func(fd uintptr) {
		cred, credErr = syscall.GetsockoptUcred(
			int(fd),
			syscall.SOL_SOCKET,
			syscall.SO_PEERCRED,
		)
	})
	if err != nil {
		return nil, err
	}
	if credErr != nil {
		return nil, credErr
	}

	return cred, nil
}