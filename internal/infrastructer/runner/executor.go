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
	Exec(ctx context.Context, language string, filename string) (output string, err error)
}

type codeRunner struct {
	pool *ContainerPool
}

func NewCodeRunner(pool *ContainerPool) CodeRunner {
	return &codeRunner{
		pool: pool,
	}
}

func (cr *codeRunner) Exec(ctx context.Context, language, filePath string) (string, error) {
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

	absPath, _ := filepath.Abs(filePath)
	hostDir := filepath.Dir(absPath)

	// 创建一个临时的Docker客户端
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return "", err
	}

	// 复制文件到容器
	copyOpts := []string{
		"docker", "cp", hostDir, c.ID + ":/app",
	}
	_, err = cli.ContainerExecCreate(ctx, c.ID, container.ExecOptions{
		Cmd: copyOpts,
	})
	if err != nil {
		return "", fmt.Errorf("failed to copy files to container: %v", err)
	}

	// 设置容器状态为running
	cr.pool.SetContainerRunning(c.ID)

	// 在容器中执行命令
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh", "-c", fmt.Sprintf("cd /app && %s", cmd)},
	}

	execResp, err := cli.ContainerExecCreate(ctx, c.ID, execConfig)
	if err != nil {
		return "", err
	}

	resp, err := cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
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
