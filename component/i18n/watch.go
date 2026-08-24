package i18n

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WatchDir 周期重载语言资源目录(热更新,不改代码不重启):
// 每 interval 扫描 dir 下 *.json 的修改指纹(mtime+size+内容哈希),
// 任一文件变化时重新 LoadDir(同名 key 覆盖)并调用 onChange。
// 阻塞运行;ctx 取消后退出。首次扫描不触发回调(与配置中心 Watch 语义一致)。
//
//	go bundle.WatchDir(ctx, "langs", 10*time.Second, func() {
//	    logger.Info("language resources reloaded")
//	})
func (b *Bundle) WatchDir(ctx context.Context, dir string, interval time.Duration, onChange func()) error {
	if b == nil {
		return errors.New("i18n: bundle is nil")
	}
	if dir == "" {
		return errors.New("i18n: watch dir is empty")
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	last, err := dirFingerprint(dir)
	if err != nil {
		return fmt.Errorf("i18n: initial fingerprint %q: %w", dir, err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current, err := dirFingerprint(dir)
			if err != nil {
				continue // 目录暂时不可读,下轮重试
			}
			if current == last {
				continue
			}
			last = current
			if err := b.LoadDir(dir); err != nil {
				continue // 加载失败保留旧资源,下轮重试
			}
			if onChange != nil {
				onChange()
			}
		}
	}
}

// dirFingerprint 计算目录下全部 *.json 文件的修改指纹(排序拼接)。
func dirFingerprint(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	hasher := sha256.New()
	for _, name := range names {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		_, _ = hasher.Write([]byte(name))
		_, _ = hasher.Write([]byte(fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size())))
		_, _ = hasher.Write(content)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
