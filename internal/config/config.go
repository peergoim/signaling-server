package config

import (
	"errors"
	"github.com/fsnotify/fsnotify"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
)

type CorsConfig struct {
	Enabled          bool     `json:",optional"`
	AllowOrigins     []string `json:",optional"`
	AllowHeaders     []string `json:",optional"`
	AllowMethods     []string `json:",optional"`
	ExposeHeaders    []string `json:",optional"`
	AllowCredentials bool     `json:",optional"`
}

type IpWhitelistConfig struct {
	Enabled    bool     `json:",optional"`
	IpList     []string `json:",optional"`
	File       string   `json:",optional"`
	ipListLock sync.RWMutex
}

type WebSocketConfig struct {
	ListenOn    string             `json:",default=0.0.0.0:21480"`
	IpWhitelist *IpWhitelistConfig `json:",optional"`
	CallTimeout int                `json:",default=10"` // 单位：秒
}

type Config struct {
	Mode      string       `json:",default=dev,options=dev|pro"`
	Cors      CorsConfig   `json:",optional"`
	Log       logx.LogConf `json:",optional"`
	Telemetry trace.Config `json:",optional"`
	WebSocket WebSocketConfig
}

var (
	ErrInvalidMode = errors.New("invalid mode, mode must be in [dev, pro]")
)

func (c *Config) Validate() error {
	if c.Mode != "dev" && c.Mode != "pro" {
		return ErrInvalidMode
	}
	if e := c.WebSocket.IpWhitelist.Validate(); e != nil {
		return e
	}
	logx.MustSetup(c.Log)
	trace.StartAgent(c.Telemetry)
	return nil
}

func (c *IpWhitelistConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.File != "" {
		c.readIpWhitelistFromFile()
	}
	go c.listenIpWhitelistChange()
	return nil
}

func (c *IpWhitelistConfig) readIpWhitelistFromFile() {
	filepath := c.File
	// 逐行读取文件，如果符合IP格式，就加入白名单
	ipList := make([]string, 0)
	// 读取文件
	content, err := os.ReadFile(filepath)
	if err != nil {
		logx.Errorf("read file %s error: %v", filepath, err)
		return
	}
	// 逐行读取
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		// 去掉空格
		line = strings.TrimSpace(line)
		// 如果是空行，就跳过
		if line == "" {
			continue
		}
		// 如果是注释，就跳过
		if strings.HasPrefix(line, "#") {
			continue
		}
		// 如果符合IP格式，就加入白名单
		reg := `^((25[0-5]|2[0-4]\d|1\d{2}|[1-9]\d|\d)\.){3}(25[0-5]|2[0-4]\d|1\d{2}|[1-9]\d|\d)(:\d{1,5})?$`
		if ok, _ := regexp.MatchString(reg, line); ok {
			ipList = append(ipList, line)
		}
	}
	// 更新白名单
	c.ipListLock.Lock()
	defer c.ipListLock.Unlock()
	c.IpList = ipList
}

func (c *IpWhitelistConfig) listenIpWhitelistChange() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	// 监听文件
	err = watcher.Add(c.File)
	if err != nil {
		log.Fatal(err)
	}
	for {
		select {
		case event, ok := <-watcher.Events:
			// 如果文件被删除，就重新监听
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				err = watcher.Add(c.File)
				if err != nil {
					log.Fatal(err)
				}
			}
			// 如果文件被修改，就重新读取
			if event.Op&fsnotify.Write == fsnotify.Write {
				c.readIpWhitelistFromFile()
			}
			if !ok {
				return
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logx.Errorf("watcher error: %v", err)
		}
	}
}

func (c *IpWhitelistConfig) InIpWhitelist(ip string) bool {
	ips := strings.Split(ip, ",")
	if !c.Enabled {
		return true
	}
	// 只要有一个ip在白名单中，就返回true
	c.ipListLock.RLock()
	defer c.ipListLock.RUnlock()
	for _, v := range ips {
		// 把后面端口号去掉
		if strings.Contains(v, ":") {
			v = strings.Split(v, ":")[0]
		}
		for _, ip := range c.IpList {
			if ip == v {
				return true
			}
		}
	}
	return false
}
