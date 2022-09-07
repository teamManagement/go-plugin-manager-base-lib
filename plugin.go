package pluginmanagerbaselib

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/go-base-lib/coderutils"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"sync"
	"time"
)

type PluginInfo struct {
	lock sync.Mutex
	// Id 插件ID
	Id string
	// Name 插件名称
	Name string
	// HandshakeConfig 插件握手协议
	plugin.HandshakeConfig

	// VersionedPlugins is a map of PluginSets for specific protocol versions.
	// These can be used to negotiate a compatible version between client and
	// server. If this is set, Handshake.ProtocolVersion is not required.
	VersionedPlugins map[int]plugin.PluginSet

	// SecureConfig 安全配置
	SecureConfig *plugin.SecureConfig

	// TLSConfig is used to enable TLS on the RPC client.
	TLSConfig *tls.Config

	// StartTimeout is the timeout to wait for the plugin to say it
	// has started successfully.
	StartTimeout time.Duration

	//  PrefixCmdAndArgs 前置命令以及参数, 例如: ["java", "-jar"]
	PrefixCmdAndArgs []string

	// PluginFilePath 插件文件路径
	PluginFilePath string

	// AllowedProtocols 允许的协议
	AllowedProtocols []plugin.Protocol

	// GRPCDialOptions grpc连接选项
	GRPCDialOptions []grpc.DialOption

	// client 客户端
	client *plugin.Client
	rpcCli plugin.ClientProtocol

	err       error
	pluginSet plugin.PluginSet
	stop      bool

	// listenSignal 监听信号
	listenSignal chan struct{}
}

func (p *PluginInfo) start() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.close()

	p.stop = false

	if p.SecureConfig != nil && p.SecureConfig.Hash != nil && p.SecureConfig.Checksum != nil {
		hResult, err := coderutils.HashByFilePath(p.SecureConfig.Hash, p.PluginFilePath)
		if err != nil {
			p.err = err
			return
		}

		if !bytes.Equal(hResult, p.SecureConfig.Checksum) {
			p.err = fmt.Errorf("插件文件[%s]与预期的HASH不一致", p.PluginFilePath)
			return
		}
	}

	p.client = plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  p.HandshakeConfig,
		Plugins:          p.pluginSet,
		AllowedProtocols: p.AllowedProtocols,
		VersionedPlugins: p.VersionedPlugins,
		TLSConfig:        p.TLSConfig,
		StartTimeout:     p.StartTimeout,
		GRPCDialOptions:  p.GRPCDialOptions,
	})

	p.rpcCli, p.err = p.client.Client()
	if p.err != nil {
		return
	}
	go p.listen()
}

func (p *PluginInfo) listen() {
	p.lock.Lock()
	if p.listenSignal != nil {
		return
	}
	p.listenSignal = make(chan struct{}, 1)
	p.lock.Unlock()

	for {
		timeout := time.After(30 * time.Second)
		select {
		case <-p.listenSignal:
			p.listenSignal <- struct{}{}
			return
		case <-timeout:
			if p.client == nil || p.client.Exited() || p.rpcCli == nil {
				p.start()
				continue
			}

			if err := p.rpcCli.Ping(); err != nil {
				p.start()
				continue
			}
		}
	}

}

func (p *PluginInfo) cancelListen() {
	if p.lock.TryLock() {
		defer p.lock.Unlock()
	}

	if p.listenSignal == nil {
		return
	}

	p.listenSignal <- struct{}{}
	<-p.listenSignal
	close(p.listenSignal)
	p.listenSignal = nil
}

func (p *PluginInfo) close() {
	if p.lock.TryLock() {
		defer p.lock.Unlock()
	}

	if p.stop {
		return
	}

	p.cancelListen()

	if p.rpcCli != nil {
		_ = p.rpcCli.Close()
	}

	p.rpcCli = nil

	if p.client == nil {
		p.client.Kill()
	}

	p.client = nil

	p.stop = true
}

func (p *PluginInfo) IsStop() bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.stop
}
