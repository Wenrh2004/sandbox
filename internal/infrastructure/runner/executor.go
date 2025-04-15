package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type CodeRunner interface {
	Exec(ctx context.Context, language string, filename string, fileContent string) (output string, err error)
}

type codeRunner struct {
	cli  *client.Client
	pool *ContainerPool
}

func NewCodeRunner(pool *ContainerPool) CodeRunner {
	// 创建一个临时的Docker客户端
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	return &codeRunner{
		cli:  cli,
		pool: pool,
	}
}

func (cr *codeRunner) Exec(ctx context.Context, language, filePath, fileContent string) (string, error) {
	strategy := GetStrategy(language)
	if strategy == nil {
		return "", fmt.Errorf("unsupported language: %s", language)
	}
	
	cmd := strategy.GetExecCommand(filepath.Base(filePath))
	
	// 从池中获取一个容器（此时容器状态为pending）
	c, err := cr.pool.GetContainer(ctx, language)
	if err != nil {
		return "", fmt.Errorf("failed to get container from pool: %v", err)
	}
	
	// 使用完后将容器状态设置为releasing并最终归还到池中
	defer func() {
		cr.pool.ReleaseContainer(c.ID)
	}()
	
	// 在容器中创建文件并写入内容
	fileName := filepath.Base(filePath)
	createFileCmd := fmt.Sprintf("mkdir -p /app && echo '%s' > /app/%s",
		strings.ReplaceAll(fileContent, "'", "'\"'\"'"), fileName)
	
	_, err = cr.cli.ContainerExecCreate(ctx, c.ID, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh", "-c", createFileCmd},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create file in container: %v", err)
	}
	
	// 设置容器状态为running
	cr.pool.SetContainerRunning(c.ID)
	
	// 在容器中执行命令
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh", "-c", fmt.Sprintf("cd /app && %s", cmd)},
	}
	
	execResp, err := cr.cli.ContainerExecCreate(ctx, c.ID, execConfig)
	if err != nil {
		return "", err
	}
	
	resp, err := cr.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", err
	}
	defer resp.Close()
	
	var outBuf, errBuf strings.Builder
	_, err = stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader)
	if err != nil {
		return "", err
	}
	
	output := outBuf.String()
	errOutput := errBuf.String()
	
	if errOutput != "" {
		return output + "\n" + errOutput, nil
	}
	
	return output, nil
}
