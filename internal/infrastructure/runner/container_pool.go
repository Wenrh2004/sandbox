package runner

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/spf13/viper"
	
	"github.com/Wenrh2004/sandbox/pkg/quene"
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
	containers      map[string]*quene.RingQueue[*Container] // 使用环形队列按语言类型分组的容器
	cli             *client.Client                          // Docker 客户端
	mutex           sync.Mutex
	maxPerLang      int           // 每种语言最大容器数
	idleTimeout     time.Duration // 空闲容器超时时间
	reservedPerLang int           // 每种语言预留的容器数
}

// NewContainerPool 创建一个新的容器池
func NewContainerPool(conf *viper.Viper, cli *client.Client) (*ContainerPool, error) {
	maxPerLang := conf.GetInt("app.container.max_num")
	reservedPerLang := conf.GetInt("app.container.reserved_num") // 从配置中读取预留容器数
	if maxPerLang <= 0 || maxPerLang < reservedPerLang {
		return nil, fmt.Errorf("invalid maxPerLang (%d) or reservedPerLang (%d)", maxPerLang, reservedPerLang)
	}
	
	pool := &ContainerPool{
		containers:      make(map[string]*quene.RingQueue[*Container]),
		cli:             cli,
		maxPerLang:      maxPerLang,
		idleTimeout:     conf.GetDuration("app.container.timeout") * time.Hour,
		reservedPerLang: reservedPerLang,
	}
	
	// 启动清理协程
	go pool.cleanupIdleContainers()
	
	// 初始化预留容器
	if err := pool.initReservedContainers(); err != nil {
		return nil, fmt.Errorf("failed to initialize reserved containers: %v", err)
	}
	
	return pool, nil
}

// initReservedContainers 初始化预留容器
func (p *ContainerPool) initReservedContainers() error {
	if p.reservedPerLang <= 0 {
		return nil // 如果预留数量为0或负数，则不创建预留容器
	}
	
	// 获取所有支持的语言
	languages := p.getSupportedLanguages()
	
	ctx := context.Background()
	for _, lang := range languages {
		// 为每种语言创建预留容器
		for i := 0; i < p.reservedPerLang; i++ {
			c, err := p.GetContainer(ctx, lang)
			if err != nil {
				return fmt.Errorf("failed to create reserved c for %s: %v", lang, err)
			}
			
			// 设置状态为空闲
			c.Status = ContainerStatusIdle
		}
	}
	
	return nil
}

// getSupportedLanguages 返回所有支持的编程语言
func (p *ContainerPool) getSupportedLanguages() []string {
	// 从策略中获取所有支持的语言
	// TODO: 根据策略注册表获取
	return []string{"go", "python", "java", "javascript", "cpp"}
}

// GetContainer 从池中获取一个容器，如果没有可用的则创建一个新的
func (p *ContainerPool) GetContainer(ctx context.Context, language string) (*Container, error) {
	strategy := GetStrategy(language)
	if strategy == nil {
		return nil, fmt.Errorf("[ContainerPool.GetContainer]unsupported language: %s", language)
	}
	
	image := strategy.GetImage()
	
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	// 确保该语言的队列已初始化
	if _, exists := p.containers[language]; !exists {
		p.containers[language] = quene.NewRingQueue[*Container](p.maxPerLang)
	}
	
	queue := p.containers[language]
	
	// 检查是否有空闲容器
	var idleContainer *Container
	var activeContainers []*Container
	
	// 搜索空闲容器
	for !queue.IsEmpty() {
		c, err := queue.Dequeue()
		if err != nil {
			break
		}
		
		if c.Status == ContainerStatusIdle {
			idleContainer = c
			c.Status = ContainerStatusPending
			c.LastUsed = time.Now()
			break
		}
		
		// 保存非空闲容器
		activeContainers = append(activeContainers, c)
	}
	
	// 将活跃的容器重新入队
	for _, c := range activeContainers {
		_ = queue.Enqueue(c)
	}
	
	// 如果找到空闲容器，直接返回
	if idleContainer != nil {
		_ = queue.Enqueue(idleContainer)
		return idleContainer, nil
	}
	
	// 检查是否达到该语言的容器上限
	if queue.IsFull() {
		return nil, fmt.Errorf("[ContainerPool.GetContainer]reach max containers for language: %s", language)
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
	
	// 创建容器但不启动
	containerResp, err := p.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return nil, err
	}
	
	// 更新容器ID
	newContainer.ID = containerResp.ID
	
	// 启动容器
	if err := p.cli.ContainerStart(ctx, containerResp.ID, container.StartOptions{}); err != nil {
		return nil, err
	}
	
	// 更新状态为等待
	newContainer.Status = ContainerStatusPending
	
	// 将新容器添加到队列中
	_ = queue.Enqueue(newContainer)
	
	return newContainer, nil
}

// SetContainerRunning 将容器状态设置为执行中
func (p *ContainerPool) SetContainerRunning(containerID string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	for _, queue := range p.containers {
		queue.ForEach(func(c *Container) {
			if c.ID == containerID {
				c.Status = ContainerStatusRunning
			}
		})
	}
}

// ReleaseContainer 开始释放容器，并清理容器内的临时文件
func (p *ContainerPool) ReleaseContainer(containerID string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	for _, queue := range p.containers {
		queue.ForEach(func(c *Container) {
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
			}
		})
	}
}

// cleanupIdleContainers 定期清理空闲超时的容器
func (p *ContainerPool) cleanupIdleContainers() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		p.mutex.Lock()
		now := time.Now()
		
		for lang, queue := range p.containers {
			var containersToKeep []*Container
			
			// 检查队列中的每个容器
			queue.ForEach(func(c *Container) {
				if c.Status == ContainerStatusIdle && now.Sub(c.LastUsed) > p.idleTimeout && queue.Size() > p.reservedPerLang {
					// 标记为销毁中
					c.Status = ContainerStatusDestroying
					
					// 停止并删除容器
					ctx := context.Background()
					p.cli.ContainerStop(ctx, c.ID, container.StopOptions{})
					p.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
				} else {
					containersToKeep = append(containersToKeep, c)
				}
			})
			
			// 重建队列，只保留非超时容器
			newQueue := quene.NewRingQueue[*Container](p.maxPerLang)
			for _, c := range containersToKeep {
				_ = newQueue.Enqueue(c)
			}
			p.containers[lang] = newQueue
		}
		
		p.mutex.Unlock()
	}
}

// Close 关闭容器池并清理资源
func (p *ContainerPool) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	ctx := context.Background()
	for _, queue := range p.containers {
		queue.ForEach(func(c *Container) {
			// 停止并删除非预留容器
			p.cli.ContainerStop(ctx, c.ID, container.StopOptions{})
			p.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
		})
	}
	
	return nil
}

// 全局容器池实例
var globalPool *ContainerPool
var poolOnce sync.Once

// GetContainerPool 获取全局容器池实例
func GetContainerPool(conf *viper.Viper, cli *client.Client) (*ContainerPool, error) {
	var poolErr error
	poolOnce.Do(func() {
		globalPool, poolErr = NewContainerPool(conf, cli)
	})
	return globalPool, poolErr
}
