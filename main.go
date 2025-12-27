package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "init":
		runInit(args)
	case "route":
		runGenerate("route", args)
	case "service":
		runGenerate("service", args)
	case "validator":
		runGenerate("validator", args)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("Usage:")
	fmt.Println("  vii init <dir_name> <module_path>   Scaffold a new project in <dir_name>")
	fmt.Println("  vii route <name>                    Create a new route file")
	fmt.Println("  vii service <name>                  Create a new service file")
	fmt.Println("  vii validator <name>                Create a new validator file")
}

// -- Commands --

func runInit(args []string) {
	if len(args) < 2 {
		fmt.Println("Error: missing arguments.")
		fmt.Println("Usage: vii init <dir_name> <module_path>")
		fmt.Println("Example: vii init my-app github.com/me/my-app")
		os.Exit(1)
	}

	dirName := args[0]
	modPath := args[1]

	// 1. Create Directory
	if _, err := os.Stat(dirName); err == nil {
		fmt.Printf("Error: directory '%s' already exists.\n", dirName)
		os.Exit(1)
	}
	if err := os.MkdirAll(dirName, 0755); err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created directory %s/\n", dirName)

	// 2. Initialize Go Module
	fmt.Printf("Initializing module %s...\n", modPath)
	runCmd(dirName, "go", "mod", "init", modPath)

	// 3. Create Files (Main, Route, Service, Validator)
	createFile(dirName, "name_validator.go", initTmplValidator())
	createFile(dirName, "hello_service.go", initTmplService())
	createFile(dirName, "home_route.go", initTmplRoute())
	createFile(dirName, "main.go", initTmplMain(modPath))

	// 4. Install vii & Tidy
	fmt.Println("Installing dependencies...")
	runCmd(dirName, "go", "get", "github.com/phillip-england/vii@latest")
	runCmd(dirName, "go", "mod", "tidy")

	fmt.Println("\nSuccess! Project initialized.")
	fmt.Printf("cd %s && go run .\n", dirName)
}

func runGenerate(kind string, args []string) {
	if len(args) == 0 {
		fmt.Printf("Error: missing filename for %s\n", kind)
		os.Exit(1)
	}

	rawName := args[0]
	structName := toPascalCase(rawName)
	fileName := strings.ToLower(rawName)
	if !strings.HasSuffix(fileName, ".go") {
		fileName += ".go"
	}

	if _, err := os.Stat(fileName); err == nil {
		fmt.Printf("Error: file %s already exists\n", fileName)
		os.Exit(1)
	}

	var content string
	switch kind {
	case "route":
		content = tmplRoute(structName)
	case "service":
		content = tmplService(structName)
	case "validator":
		content = tmplValidator(structName)
	}

	if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
		fmt.Printf("Error creating %s: %v\n", kind, err)
		os.Exit(1)
	}
	fmt.Printf("Created %s: %s (%s)\n", kind, fileName, structName)
}

// -- Helpers --

func runCmd(dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running '%s %v': %v\n", name, args, err)
		os.Exit(1)
	}
}

func createFile(dir, name, content string) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Printf("Error writing %s: %v\n", name, err)
		os.Exit(1)
	}
	fmt.Printf("Created %s\n", name)
}

func toPascalCase(s string) string {
	ext := filepath.Ext(s)
	s = strings.TrimSuffix(s, ext)
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")

	var spaced strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			if unicode.IsLower(prev) && unicode.IsUpper(r) {
				spaced.WriteRune(' ')
			}
		}
		spaced.WriteRune(r)
	}
	s = spaced.String()

	parts := strings.Fields(s)
	for i := range parts {
		if len(parts[i]) > 0 {
			r := []rune(parts[i])
			r[0] = unicode.ToUpper(r[0])
			parts[i] = string(r)
		}
	}
	return strings.Join(parts, "")
}

// -- Scaffold Templates (for Init) --

func initTmplMain(modPath string) string {
	return fmt.Sprintf(`package main

import (
	"fmt"
	"net/http"
	"github.com/phillip-england/vii/vii"
)

func main() {
	app := vii.New()

	// Global services run on every request
	app.Use(vii.LoggerService{})

	// Mount our HomeRoute at root
	app.Mount(http.MethodGet, "/", HomeRoute{})

	fmt.Println("Server running on http://localhost:8080")
	fmt.Println("Try: curl \"http://localhost:8080?name=Jace\"")
	
	if err := http.ListenAndServe(":8080", app); err != nil {
		panic(err)
	}
}
`)
}

func initTmplRoute() string {
	return `package main

import (
	"fmt"
	"net/http"
	"github.com/phillip-england/vii/vii"
)

type HomeRoute struct{}

// OnMount is called when the route is registered
func (HomeRoute) OnMount(app *vii.App) error {
	return nil
}

// Services defines middleware specific to this route
func (HomeRoute) Services() []vii.Service {
	return []vii.Service{
		HelloService{},
	}
}

// Validators ensures we have the data we need before Handle is called
func (HomeRoute) Validators() []vii.AnyValidator {
	return []vii.AnyValidator{
		vii.SV(NameValidator{}),
	}
}

// Handle is the core logic. Validated data is already in context.
func (HomeRoute) Handle(r *http.Request, w http.ResponseWriter) error {
	// Retrieve typed data from NameValidator
	data, ok := vii.Validated[NameData](r)
	if !ok {
		return fmt.Errorf("missing NameData")
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Hello, %s!", data.Name)))
	return nil
}

// OnErr handles any error returned by Validators, Services, or Handle
func (HomeRoute) OnErr(r *http.Request, w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
}
`
}

func initTmplService() string {
	return `package main

import (
	"fmt"
	"net/http"
	"github.com/phillip-england/vii/vii"
)

// HelloService is an example service (middleware)
type HelloService struct{}

func (HelloService) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	fmt.Println("[HelloService] Before request...")
	return r, nil
}

func (HelloService) After(r *http.Request, w http.ResponseWriter) error {
	fmt.Println("[HelloService] After request...")
	return nil
}
`
}

func initTmplValidator() string {
	return `package main

import (
	"fmt"
	"net/http"
)

// NameData is the typed data we want to extract
type NameData struct {
	Name string
}

// NameValidator extracts NameData from the request query params
type NameValidator struct{}

func (NameValidator) Validate(r *http.Request) (NameData, error) {
	name := r.URL.Query().Get("name")
	if name == "" {
		return NameData{}, fmt.Errorf("missing 'name' query parameter")
	}
	return NameData{Name: name}, nil
}
`
}

// -- Generator Templates (for vii route/service/validator) --

func tmplRoute(name string) string {
	return fmt.Sprintf(`package main

import (
	"net/http"
	"github.com/phillip-england/vii/vii"
)

type %s struct{}

func (%s) OnMount(app *vii.App) error {
	return nil
}

func (%s) Services() []vii.Service {
	return []vii.Service{}
}

func (%s) Validators() []vii.AnyValidator {
	return []vii.AnyValidator{}
}

func (%s) Handle(r *http.Request, w http.ResponseWriter) error {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("%s endpoint"))
	return nil
}

func (%s) OnErr(r *http.Request, w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
`, name, name, name, name, name, name, name)
}

func tmplService(name string) string {
	return fmt.Sprintf(`package main

import (
	"net/http"
)

type %s struct{}

func (%s) Before(r *http.Request, w http.ResponseWriter) (*http.Request, error) {
	return r, nil
}

func (%s) After(r *http.Request, w http.ResponseWriter) error {
	return nil
}
`, name, name, name)
}

func tmplValidator(name string) string {
	return fmt.Sprintf(`package main

import (
	"net/http"
)

type %sData struct {
	// Add fields
}

type %s struct{}

func (%s) Validate(r *http.Request) (%sData, error) {
	return %sData{}, nil
}
`, name, name, name, name, name)
}