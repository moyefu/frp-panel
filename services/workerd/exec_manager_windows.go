//go:build windows

package workerd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/VaalaCat/frp-panel/services/app"
	"github.com/VaalaCat/frp-panel/utils"
	"github.com/VaalaCat/frp-panel/utils/logger"
	"github.com/sirupsen/logrus"
)

type workerExecManager struct {
	// 用于外层循环的退出
	signMap *utils.SyncMap[string, bool]
	// 用于执行cancel函数
	chanMap *utils.SyncMap[string, chan struct{}]
	// 用于等待命令完全退出
	doneMap *utils.SyncMap[string, chan struct{}]
	// 可执行文件路径
	binaryPath string
	// 默认参数
	defaultArgs []string
}

func NewExecManager(binPath string, defaultArgs []string) app.WorkerExecManager {
	if len(defaultArgs) == 0 {
		defaultArgs = []string{"serve", "--verbose"}
	} else {
		// Filter out --watch because workerd on Windows does not support file watching:
		// "workerd serve: --watch: File watching is not yet implemented on your OS"
		filteredArgs := make([]string, 0, len(defaultArgs))
		for _, arg := range defaultArgs {
			if arg != "--watch" && arg != "-w" {
				filteredArgs = append(filteredArgs, arg)
			}
		}
		defaultArgs = filteredArgs
	}

	return &workerExecManager{
		signMap:     new(utils.SyncMap[string, bool]),
		chanMap:     new(utils.SyncMap[string, chan struct{}]),
		doneMap:     new(utils.SyncMap[string, chan struct{}]),
		binaryPath:  binPath,
		defaultArgs: defaultArgs,
	}
}

func (m *workerExecManager) resolveBinaryPath() string {
	if m.binaryPath != "" {
		if _, err := os.Stat(m.binaryPath); err == nil {
			return m.binaryPath
		}
		if _, err := os.Stat(m.binaryPath + ".exe"); err == nil {
			return m.binaryPath + ".exe"
		}
	}

	candidates := []string{}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "workerd.exe"),
			filepath.Join(cwd, "workerd"),
		)
	}
	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates,
			filepath.Join(filepath.Dir(exePath), "workerd.exe"),
			filepath.Join(filepath.Dir(exePath), "workerd"),
		)
	}
	candidates = append(candidates,
		`D:\Program Files\frpp\workerd.exe`,
		`C:\Program Files\frpp\workerd.exe`,
	)

	for _, cand := range candidates {
		if _, err := os.Stat(cand); err == nil {
			m.binaryPath = cand
			return cand
		}
	}

	if lp, err := exec.LookPath("workerd.exe"); err == nil {
		m.binaryPath = lp
		return lp
	}
	if lp, err := exec.LookPath("workerd"); err == nil {
		m.binaryPath = lp
		return lp
	}

	return m.binaryPath
}

func (m *workerExecManager) RunCmd(uid string, cwd string, argv []string) {
	ctx := context.Background()
	logger.Logger(ctx).Infof("start to run command, command id: [%s], argv: %s", uid, utils.MarshalForJson(argv))
	if _, ok := m.chanMap.Load(uid); ok {
		logger.Logger(ctx).Infof("command id: [%s] is already running, ignore", uid)
		return
	}

	c := make(chan struct{})
	m.chanMap.Store(uid, c)

	done := make(chan struct{})
	m.doneMap.Store(uid, done)

	ctx, cancel := context.WithCancel(context.Background())

	binPath := m.resolveBinaryPath()
	if binPath == "" {
		logger.Logger(ctx).Errorf("command id: [%s] run failed: workerd binary not found on Windows", uid)
		return
	}

	go func(ctx context.Context, uid string, argv []string, m *workerExecManager, binPath string, done chan struct{}) {
		defer func() {
			m.signMap.Delete(uid)
			close(done)
			m.doneMap.Delete(uid)
		}()

		logger.Logger(ctx).Infof("command id: [%s] is running!", uid)

		for {
			args := []string{}
			args = append(args, m.defaultArgs...)
			args = append(args, argv...)

			cmd := exec.CommandContext(ctx, binPath, args...)
			cmd.Dir = cwd
			cmd.SysProcAttr = &syscall.SysProcAttr{
				HideWindow: true,
			}
			cmd.Stdout = logger.LoggerWriter("workerd", logrus.InfoLevel)
			cmd.Stderr = logger.LoggerWriter("workerd", logrus.ErrorLevel)
			if err := cmd.Run(); err != nil {
				if exit, ok := m.signMap.Load(uid); ok && exit {
					logger.Logger(ctx).Infof("command id: [%s] stopped successfully", uid)
					return
				}
				if ctx.Err() != nil {
					logger.Logger(ctx).Infof("command id: [%s] stopped by context cancel", uid)
					return
				}
				logger.Logger(ctx).WithError(err).Errorf("command id: [%s] run failed, binary path: [%s], args: %s", uid, binPath, utils.MarshalForJson(args))
			}

			if exit, ok := m.signMap.Load(uid); ok && exit {
				return
			}
			if ctx.Err() != nil {
				return
			}
			time.Sleep(3 * time.Second)
		}
	}(ctx, uid, argv, m, binPath, done)

	go func(cancel context.CancelFunc, uid string, m *workerExecManager) {
		defer func(uid string, m *workerExecManager) {
			m.chanMap.Delete(uid)
		}(uid, m)

		if channel, ok := m.chanMap.Load(uid); ok {
			<-channel
			m.signMap.Store(uid, true)
			cancel()
			return
		} else {
			logger.Logger(ctx).Errorf("command id: [%s] is not running!", uid)
			return
		}
	}(cancel, uid, m)
}

func (m *workerExecManager) ExitCmd(uid string) {
	done, hasDone := m.doneMap.Load(uid)
	if channel, ok := m.chanMap.Load(uid); ok {
		m.signMap.Store(uid, true)
		select {
		case channel <- struct{}{}:
		default:
		}
	}
	if hasDone && done != nil {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
}

func (m *workerExecManager) ExitAllCmd() {
	for uid := range m.chanMap.ToMap() {
		m.ExitCmd(uid)
	}
}

func (m *workerExecManager) UpdateBinaryPath(path string) {
	m.binaryPath = path
}
