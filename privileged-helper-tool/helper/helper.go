package helper

import (
	"context"
	"fmt"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"kt-connect/privileged-helper-tool/helper/assets"
	"kt-connect/privileged-helper-tool/helper/logger"
)

type HelperRequest struct {
	Command string
}

type HelperResponse struct {
	Code    int
	Message string
}

type HelperRPC struct {
	manager *HelperManager
}

func (r *HelperRPC) Enable(req *struct{}, reply *string) error {
	if err := r.manager.enableKtctl(); err != nil {
		return rpc.ServerError(err.Error())
	}
	*reply = "installation successful"
	return nil
}

func (r *HelperRPC) StartDebug(req *HelperRequest, reply *string) error {
	if err := r.manager.startDebug(req.Command); err != nil {
		return rpc.ServerError(err.Error())
	}
	*reply = "successful startup and debugging"
	return nil
}

func (r *HelperRPC) EndDebug(req *struct{}, reply *string) error {
	if err := r.manager.endDebug(); err != nil {
		return rpc.ServerError(err.Error())
	}
	*reply = "successful completion of debugging"
	return nil
}

func (r *HelperRPC) CheckVersion(req *struct{}, hasNew *bool) error {
	newVersion, err := r.manager.checkNewVersion()
	if err != nil {
		return rpc.ServerError(err.Error())
	}
	*hasNew = newVersion
	return nil
}

// 保留原有的常量定义
const (
	ResponseCodeSuccess = 0
	ResponseCodeError   = 1
)

type HelperManager struct {
	version  string
	hc       JfrogClient
	mutex    sync.Mutex
	ktctlPid int64
	server   *rpc.Server
}

func NewHelperManager(version string) *HelperManager {
	manager := &HelperManager{
		hc:      NewJfrogClient(),
		version: version,
		server:  rpc.NewServer(),
	}
	rpcHandler := &HelperRPC{manager: manager}
	if err := manager.server.Register(rpcHandler); err != nil {
		logger.Error("RPC registration failed: %v", err)
	}
	return manager
}

func (h *HelperManager) handleConnection(conn net.Conn) {
	defer conn.Close()
	codec := jsonrpc.NewServerCodec(conn)
	h.server.ServeCodec(codec)
}

func (h *HelperManager) startDebug(originCmd string) error {
	process, _ := os.FindProcess(int(h.ktctlPid))
	if err := process.Signal(syscall.Signal(0)); err == nil {
		return fmt.Errorf("other applications are already undergoing local debugging")
	}

	if originCmd == "" {
		return fmt.Errorf("command is empty")
	}
	absCmdStr := strings.Replace(originCmd, "ktctl", "/usr/local/bin/ktctl", 1)
	cmd := exec.Command("/bin/sh", "-c", absCmdStr)

	logPath := "/tmp/ktctl.log"
	os.Remove(logPath)
	logFile, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("can not current user: %w", err)
	}
	cmd.Env = append(os.Environ(), "HOME="+usr.HomeDir)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to execute the command: %s, %w", absCmdStr, err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			logger.Error("command %s finished with error: %w", absCmdStr, err)
			return
		}
	}()
	logger.Info("the debugging command %s has been started", absCmdStr)
	atomic.StoreInt64(&h.ktctlPid, int64(cmd.Process.Pid))
	return nil
}

func (h *HelperManager) endDebug() error {
	if h.ktctlPid == 0 {
		logger.Warn("there are no running ktctl process")
		return nil
	}
	process, err := os.FindProcess(int(h.ktctlPid))
	if err != nil {
		logger.Warn("can not find process %d, %w", h.ktctlPid, err)
		return nil
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("the termination signal failed to be sent: %w", err)
	}
	logger.Info("successfully terminate the process: %d", h.ktctlPid)
	return nil
}

func (h *HelperManager) checkNewVersion() (bool, error) {
	if _, err := os.Stat("/Library/PrivilegedHelperTools/ktctl-helper"); os.IsNotExist(err) {
		return true, nil
	}
	cmd := exec.Command("/bin/sh", "-c", "/Library/PrivilegedHelperTools/ktctl-helper", "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to get installed version: %w", err)
	}
	curVersion := strings.TrimSpace(string(output))
	newVersion := strings.TrimSpace(h.version)
	compare, err := CompareVersion(curVersion, newVersion)
	if err != nil {
		return false, fmt.Errorf("failed to check ktctl helper version: %w old version: %s, new version: %s", err, curVersion, newVersion)
	}
	if compare > 0 {
		return true, nil
	}
	return false, nil
}

func (h *HelperManager) enableKtctl() error {
	logger.Info("begin download ktctl from remote")
	// download from remote
	artifactsInfo, err := h.hc.GetArtifactsInfo()
	if err != nil {
		return fmt.Errorf("failed to get artifacts info: %w", err)
	}

	if _, err := os.Stat("/usr/local/bin/ktctl"); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		// example 4.0.0
		cmd := exec.Command("/bin/sh", "-c", "/usr/local/bin/ktctl", "-v")
		output, _ := cmd.CombinedOutput()
		curVersion := strings.TrimSpace(strings.Split(string(output), "ktctl version ")[1])
		// URI example: ktctl-4.0.1-macos-arm64
		newVersion := strings.Split(artifactsInfo.Children[0].URI, "-")[1]
		compare, err := CompareVersion(curVersion, newVersion)
		if err != nil {
			return fmt.Errorf("fialed to check ktctl version: %w，old version: %s, new version: %s", err, curVersion, newVersion)
		}
		if compare <= 0 {
			return nil
		}
	}

	var (
		MacKtctlArm string
		MacktctlAmd string
	)
	for _, ktctl := range artifactsInfo.Children {
		if strings.ContainsAny(ktctl.URI, "macos-arm64") {
			MacKtctlArm = ktctl.URI
		}
		if strings.Contains(ktctl.URI, "macos-amd64") {
			MacktctlAmd = ktctl.URI
		}
	}
	switch runtime.GOARCH {
	case "arm64":
		// download arm
		err = h.hc.DownloadArtifact(MacKtctlArm)
	case "amd64":
		// download amd
		err = h.hc.DownloadArtifact(MacktctlAmd)
	default:
		return fmt.Errorf("unsupported architecture")
	}
	return err
}

func (h *HelperManager) Install() error {
	cmdList := exec.Command("launchctl", "list", "|", "grep", "com.shouqianba.ktctl")
	if output, _ := cmdList.CombinedOutput(); output != nil {
		unloadCmd := exec.Command("launchctl", "unload", "/Library/LaunchDaemons/com.shouqianba.ktctl.plist")
		if _, err := unloadCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to unload plist: %w", err)
		}
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to obtain the path of the executable file: %w", err)
	}

	if err := CopyFile(exePath, "/Library/PrivilegedHelperTools/ktctl-helper"); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	cmdChown := exec.Command("chown", "root:wheel", "/Library/PrivilegedHelperTools/ktctl-helper")
	if err := cmdChown.Run(); err != nil {
		return fmt.Errorf("failed to chown pkg: %w", err)
	}

	cmdChmod := exec.Command("chmod", "u+s", "/Library/PrivilegedHelperTools/ktctl-helper")
	if err := cmdChmod.Run(); err != nil {
		return fmt.Errorf("failed to chmod pkg: %w", err)
	}

	if err := os.WriteFile("/Library/LaunchDaemons/com.shouqianba.ktctl.plist", []byte(assets.PlistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}
	if err := exec.Command("chown", "root:wheel", "/Library/LaunchDaemons/com.shouqianba.ktctl.plist").Run(); err != nil {
		return fmt.Errorf("failed to chown plist: %w", err)
	}

	cmdLoad := exec.Command("launchctl", "load", "/Library/LaunchDaemons/com.shouqianba.ktctl.plist")
	if output, err := cmdLoad.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to load plist: %w, output: %s", err, string(output))
	}
	return nil
}

func (h *HelperManager) Serve() {
	// remove old
	os.Remove("/var/run/com.shouqianba.ktctl.sock")
	os.Remove("/var/log/ktctl-helper.log")
	os.Remove("/var/log/ktctl-helper-error.log")

	listener, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: "/var/run/com.shouqianba.ktctl.sock",
		Net:  "unix",
	})
	if err != nil {
		panic(err)
	}

	// change socket permission
	if err := os.Chmod("/var/run/com.shouqianba.ktctl.sock", 0666); err != nil {
		logger.Error("Failed to set socket permissions: %w\n", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		listener.Close()
	}()

	go func() {
		ticker := time.NewTicker(time.Minute * 10)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				h.enableKtctl()
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				logger.Error("accept error: %w", err)
			}
			continue
		}
		go h.handleConnection(conn)
	}
}
