package runner

type CodeExecutor interface {
	GetImage() string
	GetExecCommand(filename string) string
}

var languageStrategyMap = map[string]CodeExecutor{}

func RegisterStrategy(lang string, strategy CodeExecutor) {
	languageStrategyMap[lang] = strategy
}

func GetStrategy(lang string) CodeExecutor {
	return languageStrategyMap[lang]
}

func GetLanguageStrategyMap() map[string]CodeExecutor {
	return languageStrategyMap
}

type PythonExecutor struct{}

func (p PythonExecutor) GetImage() string {
	return "python:3.11.12"
}

func (p PythonExecutor) GetExecCommand(filename string) string {
	return "python " + filename
}

func init() {
	RegisterStrategy("python", PythonExecutor{})
	RegisterStrategy("cpp", CppExecutor{})
}

type CppExecutor struct{}

func (c CppExecutor) GetImage() string {
	return "gcc:12.4.0"
}

func (c CppExecutor) GetExecCommand(filename string) string {
	return "sh -c 'g++ " + filename + " -o a.out && a.out'"
}

type GoExecutor struct{}

func (g GoExecutor) GetImage() string {
	return "golang:1.23.8"
}

func (g GoExecutor) GetExecCommand(filename string) string {
	return "go mod init awesome && go mod tidy && go run " + filename
}
