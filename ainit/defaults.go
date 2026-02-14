package main

import "strings"

// LangDefaults holds language-specific default values.
type LangDefaults struct {
	Language     string
	Framework    string
	Database     string
	Other        string
	BuildCmd     string
	TestCmd      string
	LintCmd      string
	DirStructure string
}

func detectLang(lang string) string {
	lower := strings.ToLower(lang)
	switch {
	case strings.Contains(lower, "go"):
		return "go"
	case strings.Contains(lower, "typescript") || strings.Contains(lower, "node") || strings.Contains(lower, "javascript"):
		return "ts"
	case strings.Contains(lower, "python"):
		return "python"
	default:
		return "other"
	}
}

func getDefaults(langKey string) LangDefaults {
	switch langKey {
	case "go":
		return LangDefaults{
			Language:  "Go 1.22",
			Framework: "Gin",
			Database:  "PostgreSQL",
			Other:     "Docker",
			BuildCmd:  "go build -o app ./cmd/...",
			TestCmd:   "go test ./...",
			LintCmd:   "golangci-lint run",
			DirStructure: `├── cmd/            # entrypoint
├── internal/       # business logic
├── pkg/            # shared packages
├── api/            # API definitions
└── tests/          # tests`,
		}
	case "ts":
		return LangDefaults{
			Language:  "TypeScript 5.x",
			Framework: "Next.js",
			Database:  "PostgreSQL",
			Other:     "Docker",
			BuildCmd:  "npm run build",
			TestCmd:   "npm test",
			LintCmd:   "eslint .",
			DirStructure: `├── src/            # source code
├── tests/          # tests
├── public/         # static assets
└── docs/           # documentation`,
		}
	case "python":
		return LangDefaults{
			Language:  "Python 3.12",
			Framework: "FastAPI",
			Database:  "PostgreSQL",
			Other:     "Docker",
			BuildCmd:  "python -m build",
			TestCmd:   "pytest",
			LintCmd:   "ruff check .",
			DirStructure: `├── src/            # source code
├── tests/          # tests
├── docs/           # documentation
└── scripts/        # scripts`,
		}
	default:
		return LangDefaults{
			Language:  "Go 1.22",
			Framework: "Gin",
			Database:  "PostgreSQL",
			Other:     "Docker",
			BuildCmd:  "make build",
			TestCmd:   "make test",
			LintCmd:   "make lint",
			DirStructure: `├── src/            # source code
├── tests/          # tests
└── docs/           # documentation`,
		}
	}
}
