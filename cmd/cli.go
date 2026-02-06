package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/operation"
	flagPkg "codeberg.org/goern/forgejo-mcp/v2/pkg/flag"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// cliMode is set to true when --cli is detected in os.Args.
var cliMode bool

// hasCLIFlag checks os.Args for --cli before flag.Parse() runs.
func hasCLIFlag() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--cli" {
			return true
		}
	}
	return false
}

// toolDomains maps tool names to their domain for grouped listing.
// Built by registering tools per domain and tracking which names appear.
var toolDomains = map[string]string{}

// registerToolsWithDomains registers all tools and builds the domain mapping.
func registerToolsWithDomains(s *server.MCPServer) {
	beforeNames := toolNames(s)

	type domainReg struct {
		name string
		fn   func(*server.MCPServer)
	}
	domains := []domainReg{
		{"user", operation.RegisterUserTool},
		{"repo", operation.RegisterRepoTool},
		{"issue", operation.RegisterIssueTool},
		{"pull", operation.RegisterPullTool},
		{"pull", operation.RegisterPullReviewTool},
		{"search", operation.RegisterSearchTool},
		{"version", operation.RegisterVersionTool},
	}

	for _, d := range domains {
		d.fn(s)
		afterNames := toolNames(s)
		for _, name := range afterNames {
			if !contains(beforeNames, name) {
				toolDomains[name] = d.name
			}
		}
		beforeNames = afterNames
	}
}

func toolNames(s *server.MCPServer) []string {
	tools := s.ListTools()
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	return names
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// RunCLI is the entry point for --cli mode.
func RunCLI(version string) {
	// Parse CLI-specific flags using a separate FlagSet.
	fs := flag.NewFlagSet("cli", flag.ExitOnError)
	argsFlag := fs.String("args", "", "JSON arguments for tool invocation")
	outputFlag := fs.String("output", "", "Output format: json or text")
	helpFlag := fs.Bool("help", false, "Show tool parameter help")

	// Find the positional command (first non-flag arg after --cli).
	// os.Args has been filtered by init() to remove --cli and preceding flags.
	cliArgs := cliArgsToParse()
	if len(cliArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: forgejo-mcp --cli <command> [options]")
		fmt.Fprintln(os.Stderr, "Commands: list, <tool-name>")
		fmt.Fprintln(os.Stderr, "Options: --args '{json}', --output=json|text, --help")
		os.Exit(1)
	}

	command := cliArgs[0]
	_ = fs.Parse(cliArgs[1:])

	// Build the MCPServer and register tools with domain tracking.
	flagPkg.Version = version
	mcpSrv := server.NewMCPServer("Forgejo MCP Server", version, server.WithLogging())
	registerToolsWithDomains(mcpSrv)

	switch command {
	case "list":
		outputMode := *outputFlag
		if outputMode == "" {
			outputMode = "text"
		}
		if err := cliList(mcpSrv, outputMode); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		if *helpFlag {
			if err := cliHelp(mcpSrv, command); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		outputMode := *outputFlag
		if outputMode == "" {
			outputMode = "json"
		}

		argsJSON, err := resolveArgs(*argsFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading arguments: %v\n", err)
			os.Exit(1)
		}

		if err := cliExec(mcpSrv, command, argsJSON, outputMode); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// cliArgsToParse extracts the args after --cli from os.Args.
func cliArgsToParse() []string {
	for i, arg := range os.Args {
		if arg == "--cli" {
			return os.Args[i+1:]
		}
	}
	return nil
}

// resolveArgs returns JSON args from --args flag or stdin pipe.
// --args takes precedence. If neither provided, returns "{}".
func resolveArgs(argsFlag string) (string, error) {
	if argsFlag != "" {
		return argsFlag, nil
	}

	// Check if stdin is a pipe (not a terminal).
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "{}", nil
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		if len(data) > 0 {
			return string(data), nil
		}
	}

	return "{}", nil
}

// cliList prints all registered tools.
func cliList(s *server.MCPServer, outputMode string) error {
	tools := s.ListTools()
	if tools == nil {
		fmt.Println("No tools registered.")
		return nil
	}

	type toolInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Domain      string `json:"domain"`
	}

	// Build sorted list.
	var infos []toolInfo
	for name, st := range tools {
		domain := toolDomains[name]
		if domain == "" {
			domain = "other"
		}
		infos = append(infos, toolInfo{
			Name:        name,
			Description: st.Tool.Description,
			Domain:      domain,
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Domain != infos[j].Domain {
			return infos[i].Domain < infos[j].Domain
		}
		return infos[i].Name < infos[j].Name
	})

	if outputMode == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(infos)
	}

	// Text mode: grouped by domain.
	grouped := map[string][]toolInfo{}
	domainOrder := []string{}
	for _, info := range infos {
		if _, exists := grouped[info.Domain]; !exists {
			domainOrder = append(domainOrder, info.Domain)
		}
		grouped[info.Domain] = append(grouped[info.Domain], info)
	}

	for _, domain := range domainOrder {
		fmt.Printf("\n%s:\n", strings.ToUpper(domain))
		for _, info := range grouped[domain] {
			fmt.Printf("  %-40s %s\n", info.Name, info.Description)
		}
	}
	fmt.Println()

	return nil
}

// cliHelp prints the parameter schema for a tool.
func cliHelp(s *server.MCPServer, toolName string) error {
	st := s.GetTool(toolName)
	if st == nil {
		return fmt.Errorf("unknown tool: %s", toolName)
	}

	fmt.Printf("Tool: %s\n", st.Tool.Name)
	if st.Tool.Description != "" {
		fmt.Printf("Description: %s\n", st.Tool.Description)
	}
	fmt.Println()

	props := st.Tool.InputSchema.Properties
	required := st.Tool.InputSchema.Required

	if len(props) == 0 {
		fmt.Println("No parameters.")
		return nil
	}

	fmt.Println("Parameters:")
	// Sort parameter names for consistent output.
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)

	requiredSet := map[string]bool{}
	for _, r := range required {
		requiredSet[r] = true
	}

	for _, name := range names {
		prop := props[name]
		reqStr := "optional"
		if requiredSet[name] {
			reqStr = "required"
		}

		// Property is stored as map[string]any.
		propMap, ok := prop.(map[string]any)
		if !ok {
			fmt.Printf("  %-20s (%s)\n", name, reqStr)
			continue
		}

		typStr, _ := propMap["type"].(string)
		desc, _ := propMap["description"].(string)

		fmt.Printf("  %-20s %-10s %-10s %s\n", name, typStr, reqStr, desc)
	}

	return nil
}

// cliExec invokes a tool handler and prints the result.
func cliExec(s *server.MCPServer, toolName, argsJSON, outputMode string) error {
	st := s.GetTool(toolName)
	if st == nil {
		return fmt.Errorf("unknown tool: %s\nRun 'forgejo-mcp --cli list' to see available tools", toolName)
	}

	// Parse JSON arguments.
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Errorf("invalid JSON arguments: %w", err)
	}

	// Construct CallToolRequest.
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	}

	// Call the handler.
	result, err := st.Handler(context.Background(), req)
	if err != nil {
		return fmt.Errorf("tool execution failed: %w", err)
	}

	// Check IsError flag.
	if result.IsError {
		if outputMode == "json" {
			enc := json.NewEncoder(os.Stderr)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result.Content)
		} else {
			for _, c := range result.Content {
				if tc, ok := c.(mcp.TextContent); ok {
					fmt.Fprintln(os.Stderr, tc.Text)
				}
			}
		}
		os.Exit(1)
	}

	// Output result.
	if outputMode == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result.Content)
	}

	// Text mode: print text content line by line.
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			fmt.Println(tc.Text)
		}
	}

	return nil
}
