package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// 容器状态
const (
	ContainerStatusCreating   = "creating"   // 容器创建中
	ContainerStatusPending    = "pending"    // 容器等待中
	ContainerStatusRunning    = "running"    // 容器执行中
	ContainerStatusReleasing  = "releasing"  // 容器释放中
	ContainerStatusIdle       = "idle"       // 容器空闲中
	ContainerStatusDestroying = "destroying" // 容器销毁中
)

// Container 代表容器池中的一个容器
type Container struct {
	ID       string
	Image    string
	Language string
	Status   string
	LastUsed time.Time
}

// ContainerPool 管理容器的池子
type ContainerPool struct {
	containers  map[string][]*Container // 按语言类型分组的容器
	cli         *client.Client
	mutex       sync.Mutex
	maxPerLang  int           // 每种语言最大容器数
	idleTimeout time.Duration // 空闲容器超时时间
}

// NewContainerPool 创建一个新的容器池
func NewContainerPool(maxPerLang int, idleTimeout time.Duration) (*ContainerPool, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	pool := &ContainerPool{
		containers:  make(map[string][]*Container),
		cli:         cli,
		maxPerLang:  maxPerLang,
		idleTimeout: idleTimeout,
	}

	// 启动清理协程
	go pool.cleanupIdleContainers()

	return pool, nil
}

// GetContainer 从池中获取一个容器，如果没有可用的则创建一个新的
func (p *ContainerPool) GetContainer(ctx context.Context, language string) (*Container, error) {
	strategy := GetStrategy(language)
	if strategy == nil {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	image := strategy.GetImage()

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 检查是否有空闲容器
	containers := p.containers[language]
	for _, c := range containers {
		if c.Status == ContainerStatusIdle {
			c.Status = ContainerStatusPending // 容器状态变为等待
			c.LastUsed = time.Now()
			return c, nil
		}
	}

	// 检查是否达到该语言的容器上限
	if len(containers) >= p.maxPerLang {
		return nil, fmt.Errorf("reach max containers for language: %s", language)
	}

	// 创建新容器
	containerConfig := &container.Config{
		Image: image,
		Cmd:   []string{"tail", "-f", "/dev/null"}, // 让容器保持运行
		Tty:   false,
	}
	hostConfig := &container.HostConfig{
		AutoRemove: false,
	}

	// 设置创建状态
	newContainer := &Container{
		Image:    image,
		Language: language,
		Status:   ContainerStatusCreating,
		LastUsed: time.Now(),
	}

	// 将新容器添加到池中
	p.containers[language] = append(p.containers[language], newContainer)

	// 创建容器但不启动
	containerResp, err := p.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		// 移除失败的容器
		p.removeContainer(newContainer)
		return nil, err
	}

	// 更新容器ID
	newContainer.ID = containerResp.ID

	// 启动容器
	if err := p.cli.ContainerStart(ctx, containerResp.ID, container.StartOptions{}); err != nil {
		// 移除失败的容器
		p.removeContainer(newContainer)
		return nil, err
	}

	// 更新状态为等待
	newContainer.Status = ContainerStatusPending

	return newContainer, nil
}

// SetContainerRunning 将容器状态设置为执行中
func (p *ContainerPool) SetContainerRunning(containerID string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, containers := range p.containers {
		for _, c := range containers {
			if c.ID == containerID {
				c.Status = ContainerStatusRunning
				return
			}
		}
	}
}

// ReleaseContainer 开始释放容器，并清理容器内的临时文件
func (p *ContainerPool) ReleaseContainer(containerID string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, containers := range p.containers {
		for _, c := range containers {
			if c.ID == containerID {
				c.Status = ContainerStatusReleasing
				// 清理容器内的文件
				ctx := context.Background()
				cli, err := client.NewClientWithOpts(client.FromEnv)
				if err == nil {
					execConfig := container.ExecOptions{
						Cmd: []string{"sh", "-c", "rm -rf /app/*"},
					}

					execResp, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
					if err == nil {
						_ = cli.ContainerExecStart(ctx, execResp.ID, container.ExecStartOptions{})
					}
				}

				c.Status = ContainerStatusIdle
				c.LastUsed = time.Now()
				return
			}
		}
	}
}

// removeContainer 从容器池中移除容器
func (p *ContainerPool) removeContainer(container *Container) {
	for lang, containers := range p.containers {
		for i, c := range containers {
			if c == container {
				p.containers[lang] = append(containers[:i], containers[i+1:]...)
				return
			}
		}
	}
}

// cleanupIdleContainers 定期清理空闲超时的容器
func (p *ContainerPool) cleanupIdleContainers() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.mutex.Lock()
		now := time.Now()

		for lang, containers := range p.containers {
			var active []*Container
			for _, c := range containers {
				if c.Status == ContainerStatusIdle && now.Sub(c.LastUsed) > p.idleTimeout {
					// 标记为销毁中
					c.Status = ContainerStatusDestroying

					// 停止并删除容器
					ctx := context.Background()
					p.cli.ContainerStop(ctx, c.ID, container.StopOptions{})
					p.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
				} else {
					active = append(active, c)
				}
			}
			p.containers[lang] = active
		}

		p.mutex.Unlock()
	}
}

// Close 关闭容器池并清理资源
func (p *ContainerPool) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	ctx := context.Background()
	for _, containers := range p.containers {
		for _, c := range containers {
			// 停止并删除容器
			p.cli.ContainerStop(ctx, c.ID, container.StopOptions{})
			p.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
		}
	}

	return nil
}

// 全局容器池实例
var globalPool *ContainerPool
var poolOnce sync.Once

// GetContainerPool 获取全局容器池实例
func GetContainerPool() (*ContainerPool, error) {
	var poolErr error
	poolOnce.Do(func() {
		globalPool, poolErr = NewContainerPool(5, 30*time.Minute)
	})
	return globalPool, poolErr
}
