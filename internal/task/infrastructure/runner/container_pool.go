package runner

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
	
	"github.com/docker/docker/api/types/container"
	image2 "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	
	"github.com/Wenrh2004/sandbox/pkg/log"
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
	logger          *log.Logger
	cli             *client.Client // Docker 客户端
	mutex           sync.Mutex
	maxPerLang      int           // 每种语言最大容器数
	idleTimeout     time.Duration // 空闲容器超时时间
	reservedPerLang int           // 每种语言预留的容器数
}

// NewContainerPool 创建一个新的容器池
func NewContainerPool(conf *viper.Viper, log *log.Logger, cli *client.Client) (*ContainerPool, func(), error) {
	maxPerLang := conf.GetInt("app.container.max_num")
	reservedPerLang := conf.GetInt("app.container.reserved_num") // 从配置中读取预留容器数
	idleTimeout := conf.GetDuration("app.container.timeout") * time.Hour
	
	if maxPerLang <= 0 || maxPerLang < reservedPerLang {
		panic(fmt.Errorf("invalid maxPerLang (%d) or reservedPerLang (%d)", maxPerLang, reservedPerLang))
	}
	
	pool := &ContainerPool{
		containers:      make(map[string]*quene.RingQueue[*Container]),
		logger:          log,
		cli:             cli,
		maxPerLang:      maxPerLang,
		idleTimeout:     idleTimeout,
		reservedPerLang: reservedPerLang,
	}
	
	pool.logger.Info("creating container pool",
		zap.Int("maxPerLang", maxPerLang),
		zap.Int("reservedPerLang", reservedPerLang),
		zap.Duration("idleTimeout", idleTimeout))
	
	// 启动清理协程
	go pool.cleanupIdleContainers()
	
	// 初始化预留容器
	if err := pool.initReservedContainers(); err != nil {
		panic(fmt.Errorf("failed to initialize reserved containers: %v", err))
	}
	
	return pool, func() {
		pool.Close()
	}, nil
}

// initReservedContainers 初始化预留容器
func (p *ContainerPool) initReservedContainers() error {
	if p.reservedPerLang <= 0 {
		p.logger.Info("no reserved containers needed, skipping initialization")
		return nil // 如果预留数量为0或负数，则不创建预留容器
	}
	
	// 获取所有支持的语言
	languages := p.getSupportedLanguages()
	p.logger.Info("initializing reserved containers",
		zap.Int("reservedPerLang", p.reservedPerLang),
		zap.Strings("languages", languages))
	
	ctx := context.Background()
	for _, lang := range languages {
		// 为每种语言创建预留容器
		p.logger.Info("creating reserved containers for language", zap.String("language", lang))
		for i := 0; i < p.reservedPerLang; i++ {
			p.logger.Debug("creating reserved container",
				zap.String("language", lang),
				zap.Int("index", i+1),
				zap.Int("total", p.reservedPerLang))
			
			c, err := p.GetContainer(ctx, lang)
			if err != nil {
				p.logger.Error("failed to create reserved container",
					zap.String("language", lang),
					zap.Error(err))
				return fmt.Errorf("failed to create reserved c for %s: %v", lang, err)
			}
			
			// 设置状态为空闲
			c.Status = ContainerStatusIdle
			p.logger.Debug("created reserved container",
				zap.String("containerId", c.ID),
				zap.String("language", lang))
		}
		p.logger.Info("finished creating reserved containers",
			zap.String("language", lang),
			zap.Int("count", p.reservedPerLang))
	}
	
	p.logger.Info("all reserved containers initialized successfully")
	return nil
}

// getSupportedLanguages 返回所有支持的编程语言
func (p *ContainerPool) getSupportedLanguages() []string {
	// 从策略中获取所有支持的语言
	var supportedLanguages []string
	strategyMap := GetLanguageStrategyMap()
	for k := range strategyMap {
		supportedLanguages = append(supportedLanguages, k)
	}
	return supportedLanguages
}

// GetContainer 从池中获取一个容器，如果没有可用的则创建一个新的
func (p *ContainerPool) GetContainer(ctx context.Context, language string) (*Container, error) {
	p.logger.Debug("getting container for language", zap.String("language", language))
	
	strategy := GetStrategy(language)
	if strategy == nil {
		p.logger.Error("unsupported language requested", zap.String("language", language))
		return nil, fmt.Errorf("[ContainerPool.GetContainer]unsupported language: %s", language)
	}
	
	image := strategy.GetImage()
	
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	// 确保该语言的队列已初始化
	if _, exists := p.containers[language]; !exists {
		p.logger.Debug("initializing container queue for language", zap.String("language", language))
		p.containers[language] = quene.NewRingQueue[*Container](p.maxPerLang)
	}
	
	queue := p.containers[language]
	
	// 检查是否有空闲容器
	var idleContainer *Container
	var activeContainers []*Container
	
	// 搜索空闲容器
	p.logger.Debug("searching for idle container",
		zap.String("language", language),
		zap.Int("queueSize", queue.Size()))
	
	for !queue.IsEmpty() {
		c, err := queue.Dequeue()
		if err != nil {
			p.logger.Error("error dequeuing container", zap.Error(err))
			break
		}
		
		if c.Status == ContainerStatusIdle {
			idleContainer = c
			c.Status = ContainerStatusPending
			c.LastUsed = time.Now()
			p.logger.Debug("found idle container",
				zap.String("containerId", c.ID),
				zap.String("language", language))
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
		p.logger.Info("reusing idle container",
			zap.String("containerId", idleContainer.ID),
			zap.String("language", language),
			zap.String("status", idleContainer.Status))
		return idleContainer, nil
	}
	
	// 检查是否达到该语言的容器上限
	if queue.IsFull() {
		p.logger.Warn("reached max containers for language",
			zap.String("language", language),
			zap.Int("maxPerLang", p.maxPerLang))
		return nil, fmt.Errorf("[ContainerPool.GetContainer]reach max containers for language: %s", language)
	}
	
	// 创建新容器
	p.logger.Info("creating new container",
		zap.String("language", language),
		zap.String("image", image))
	
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
	
	// 检查镜像是否存在
	imageExists := false
	images, err := p.cli.ImageList(ctx, image2.ListOptions{All: true})
	if err == nil {
		for _, img := range images {
			for _, tag := range img.RepoTags {
				if tag == image || tag == image+":latest" {
					imageExists = true
					break
				}
			}
			if imageExists {
				break
			}
		}
	}
	
	// 如果镜像不存在，则拉取镜像
	if !imageExists {
		p.logger.Info("pulling image", zap.String("image", image))
		reader, err := p.cli.ImagePull(ctx, image, image2.PullOptions{})
		if err != nil {
			p.logger.Error("failed to pull image",
				zap.String("image", image),
				zap.Error(err))
			return nil, fmt.Errorf("[ContainerPool.GetContainer]failed to pull image %s: %v", image, err)
		}
		defer reader.Close()
		
		// 等待拉取完成
		_, err = io.Copy(io.Discard, reader)
		if err != nil {
			p.logger.Error("failed to pull image",
				zap.String("image", image),
				zap.Error(err))
			return nil, fmt.Errorf("[ContainerPool.GetContainer]failed to pull image %s: %v", image, err)
		}
		p.logger.Info("image pulled successfully", zap.String("image", image))
	}
	
	// 创建容器但不启动
	p.logger.Debug("creating container", zap.String("image", image))
	containerResp, err := p.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		p.logger.Error("failed to create container",
			zap.String("image", image),
			zap.Error(err))
		return nil, err
	}
	
	// 更新容器ID
	newContainer.ID = containerResp.ID
	
	// 启动容器
	p.logger.Debug("starting container", zap.String("containerId", newContainer.ID))
	if err := p.cli.ContainerStart(ctx, containerResp.ID, container.StartOptions{}); err != nil {
		p.logger.Error("failed to start container",
			zap.String("containerId", newContainer.ID),
			zap.Error(err))
		return nil, err
	}
	
	// 更新状态为等待
	newContainer.Status = ContainerStatusPending
	
	// 将新容器添加到队列中
	_ = queue.Enqueue(newContainer)
	
	p.logger.Info("container created successfully",
		zap.String("containerId", newContainer.ID),
		zap.String("language", language),
		zap.String("status", newContainer.Status))
	
	return newContainer, nil
}

// SetContainerRunning 将容器状态设置为执行中
func (p *ContainerPool) SetContainerRunning(containerID string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	p.logger.Debug("setting container to running state", zap.String("containerId", containerID))
	containerFound := false
	
	for lang, queue := range p.containers {
		queue.ForEach(func(c *Container) {
			if c.ID == containerID {
				prevStatus := c.Status
				c.Status = ContainerStatusRunning
				containerFound = true
				p.logger.Info("container status changed to running",
					zap.String("containerId", containerID),
					zap.String("language", lang),
					zap.String("previousStatus", prevStatus),
					zap.String("newStatus", c.Status))
			}
		})
	}
	
	if !containerFound {
		p.logger.Warn("container not found when setting running state",
			zap.String("containerId", containerID))
	}
}

// ReleaseContainer 开始释放容器，并清理容器内的临时文件
func (p *ContainerPool) ReleaseContainer(containerID string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	p.logger.Debug("releasing container", zap.String("containerId", containerID))
	containerFound := false
	
	for lang, queue := range p.containers {
		queue.ForEach(func(c *Container) {
			if c.ID == containerID {
				containerFound = true
				prevStatus := c.Status
				c.Status = ContainerStatusReleasing
				p.logger.Info("releasing container",
					zap.String("containerId", containerID),
					zap.String("language", lang),
					zap.String("previousStatus", prevStatus))
				
				// 清理容器内的文件
				ctx := context.Background()
				cli, err := client.NewClientWithOpts(client.FromEnv)
				if err == nil {
					execConfig := container.ExecOptions{
						Cmd: []string{"sh", "-c", "rm -rf /app/*"},
					}
					
					p.logger.Debug("cleaning up container files", zap.String("containerId", containerID))
					execResp, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
					if err == nil {
						_ = cli.ContainerExecStart(ctx, execResp.ID, container.ExecStartOptions{})
						p.logger.Debug("container cleanup command executed",
							zap.String("containerId", containerID),
							zap.String("execId", execResp.ID))
					} else {
						p.logger.Warn("failed to create cleanup exec",
							zap.String("containerId", containerID),
							zap.Error(err))
					}
				} else {
					p.logger.Error("failed to create docker client for cleanup", zap.Error(err))
				}
				
				c.Status = ContainerStatusIdle
				c.LastUsed = time.Now()
				p.logger.Info("container released and set to idle",
					zap.String("containerId", containerID),
					zap.String("language", lang))
			}
		})
	}
	
	if !containerFound {
		p.logger.Warn("container not found during release",
			zap.String("containerId", containerID))
	}
}

// cleanupIdleContainers 定期清理空闲超时的容器
func (p *ContainerPool) cleanupIdleContainers() {
	ticker := time.NewTicker(10 * p.idleTimeout)
	defer ticker.Stop()
	p.logger.Info("idle container cleanup goroutine started",
		zap.Duration("checkInterval", 10*p.idleTimeout),
		zap.Duration("idleTimeout", p.idleTimeout))
	
	for range ticker.C {
		p.mutex.Lock()
		now := time.Now()
		p.logger.Info("starting cleanup of idle containers")
		
		var removedCount int
		var skippedCount int
		
		for lang, queue := range p.containers {
			var containersToKeep []*Container
			langRemovedCount := 0
			
			p.logger.Debug("checking containers for language",
				zap.String("language", lang),
				zap.Int("queueSize", queue.Size()),
				zap.Int("reservedCount", p.reservedPerLang))
			
			// 检查队列中的每个容器
			queue.ForEach(func(c *Container) {
				timeIdle := now.Sub(c.LastUsed)
				
				if c.Status == ContainerStatusIdle &&
					timeIdle > p.idleTimeout &&
					queue.Size() > p.reservedPerLang {
					// 标记为销毁中
					c.Status = ContainerStatusDestroying
					
					// 停止并删除容器
					ctx := context.Background()
					p.logger.Info("removing idle container",
						zap.String("containerId", c.ID),
						zap.String("language", lang),
						zap.Duration("idleTime", timeIdle))
					
					if err := p.cli.ContainerStop(ctx, c.ID, container.StopOptions{}); err != nil {
						p.logger.Warn("error stopping container during cleanup",
							zap.String("containerId", c.ID),
							zap.Error(err))
					}
					
					if err := p.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
						p.logger.Warn("error removing container during cleanup",
							zap.String("containerId", c.ID),
							zap.Error(err))
					}
					
					langRemovedCount++
				} else {
					if c.Status == ContainerStatusIdle && timeIdle > p.idleTimeout {
						p.logger.Debug("keeping idle container due to reserved minimum",
							zap.String("containerId", c.ID),
							zap.String("language", lang),
							zap.Duration("idleTime", timeIdle))
						skippedCount++
					}
					containersToKeep = append(containersToKeep, c)
				}
			})
			
			// 重建队列，只保留非超时容器
			newQueue := quene.NewRingQueue[*Container](p.maxPerLang)
			for _, c := range containersToKeep {
				_ = newQueue.Enqueue(c)
			}
			p.containers[lang] = newQueue
			
			removedCount += langRemovedCount
			if langRemovedCount > 0 {
				p.logger.Info("removed idle containers for language",
					zap.String("language", lang),
					zap.Int("removedCount", langRemovedCount),
					zap.Int("remainingCount", len(containersToKeep)))
			}
		}
		
		p.logger.Info("idle container cleanup completed",
			zap.Int("removedCount", removedCount),
			zap.Int("skippedCount", skippedCount))
		
		p.mutex.Unlock()
	}
}

// Close 关闭容器池并清理资源
func (p *ContainerPool) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	p.logger.Info("closing container pool and cleaning up resources")
	ctx := context.Background()
	
	totalContainers := 0
	for lang, queue := range p.containers {
		langContainers := queue.Size()
		totalContainers += langContainers
		
		p.logger.Info("removing all containers for language",
			zap.String("language", lang),
			zap.Int("containerCount", langContainers))
		
		queue.ForEach(func(c *Container) {
			// 停止并删除所有容器
			p.logger.Debug("stopping and removing container",
				zap.String("containerId", c.ID),
				zap.String("language", lang),
				zap.String("status", c.Status))
			
			if err := p.cli.ContainerStop(ctx, c.ID, container.StopOptions{}); err != nil {
				p.logger.Warn("error stopping container during pool close",
					zap.String("containerId", c.ID),
					zap.Error(err))
			}
			
			if err := p.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
				p.logger.Warn("error removing container during pool close",
					zap.String("containerId", c.ID),
					zap.Error(err))
			}
		})
	}
	
	p.logger.Info("container pool closed successfully", zap.Int("totalContainersRemoved", totalContainers))
}
