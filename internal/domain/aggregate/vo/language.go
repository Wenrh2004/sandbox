package vo

type Language struct {
	Type       string `json:"type"`
	FileSuffix string `json:"file_suffix"`
	Image      string `json:"image"`
}

func newLanguage(typ string, suffix string, img string) *Language {
	return &Language{
		Type:       typ,
		FileSuffix: suffix,
		Image:      img,
	}
}

var (
	GO         = newLanguage("GO", ".go", "golang:1.20")
	JAVA       = newLanguage("JAVA", ".java", "openjdk:17")
	PYTHON     = newLanguage("PYTHON", ".py", "python:3.10")
	JAVASCRIPT = newLanguage("JAVASCRIPT", ".js", "node:18")
	RUST       = newLanguage("RUST", ".rs", "rust:1.70")
	C          = newLanguage("C", ".c", "gcc:12")
	CPLUSPLUS  = newLanguage("C++", ".cpp", "gcc:12")
	C_SHARP    = newLanguage("C#", ".cs", "mcr.microsoft.com/dotnet/sdk:7.0")
)

func GetLanguageByType(typ string) *Language {
	switch typ {
	case GO.Type:
		return GO
	case JAVA.Type:
		return JAVA
	case PYTHON.Type:
		return PYTHON
	case JAVASCRIPT.Type:
		return JAVASCRIPT
	case RUST.Type:
		return RUST
	case C.Type:
		return C
	case CPLUSPLUS.Type:
		return CPLUSPLUS
	case C_SHARP.Type:
		return C_SHARP
	default:
		return nil
	}
}
