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
