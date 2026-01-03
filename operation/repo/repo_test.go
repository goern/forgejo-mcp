package repo

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

// TestListRepoLabelsTool verifies the tool definition is correctly configured
func TestListRepoLabelsTool(t *testing.T) {
	tool := ListRepoLabelsTool

	assert.Equal(t, "list_repo_labels", tool.Name)
	assert.NotNil(t, tool.Description)

	// Check required parameters
	params := tool.InputSchema.Properties
	assert.Contains(t, params, "owner")
	assert.Contains(t, params, "repo")

	// Verify owner is required
	assert.Contains(t, tool.InputSchema.Required, "owner")
	assert.Contains(t, tool.InputSchema.Required, "repo")
}

// TestCreateLabelTool verifies the tool definition is correctly configured
func TestCreateLabelTool(t *testing.T) {
	tool := CreateLabelTool

	assert.Equal(t, "create_label", tool.Name)
	assert.NotNil(t, tool.Description)

	// Check required parameters
	params := tool.InputSchema.Properties
	assert.Contains(t, params, "owner")
	assert.Contains(t, params, "repo")
	assert.Contains(t, params, "name")
	assert.Contains(t, params, "color")

	// Verify required fields
	assert.Contains(t, tool.InputSchema.Required, "owner")
	assert.Contains(t, tool.InputSchema.Required, "repo")
	assert.Contains(t, tool.InputSchema.Required, "name")
	assert.Contains(t, tool.InputSchema.Required, "color")
}

// TestEditLabelTool verifies the tool definition is correctly configured
func TestEditLabelTool(t *testing.T) {
	tool := EditLabelTool

	assert.Equal(t, "edit_label", tool.Name)
	assert.NotNil(t, tool.Description)

	// Check required parameters
	params := tool.InputSchema.Properties
	assert.Contains(t, params, "owner")
	assert.Contains(t, params, "repo")
	assert.Contains(t, params, "id")

	// Verify required fields
	assert.Contains(t, tool.InputSchema.Required, "owner")
	assert.Contains(t, tool.InputSchema.Required, "repo")
	assert.Contains(t, tool.InputSchema.Required, "id")

	// Check optional parameters
	assert.Contains(t, params, "name")
	assert.Contains(t, params, "color")
	assert.Contains(t, params, "description")
}

// TestDeleteLabelTool verifies the tool definition is correctly configured
func TestDeleteLabelTool(t *testing.T) {
	tool := DeleteLabelTool

	assert.Equal(t, "delete_label", tool.Name)
	assert.NotNil(t, tool.Description)

	// Check required parameters
	params := tool.InputSchema.Properties
	assert.Contains(t, params, "owner")
	assert.Contains(t, params, "repo")
	assert.Contains(t, params, "id")

	// Verify required fields
	assert.Contains(t, tool.InputSchema.Required, "owner")
	assert.Contains(t, tool.InputSchema.Required, "repo")
	assert.Contains(t, tool.InputSchema.Required, "id")
}

// TestIsValidHexColor tests the color validation function with various inputs
func TestIsValidHexColor(t *testing.T) {
	tests := []struct {
		name  string
		color string
		want  bool
	}{
		{
			name:  "valid 6-digit lowercase",
			color: "#ff0000",
			want:  true,
		},
		{
			name:  "valid 6-digit uppercase",
			color: "#FF0000",
			want:  true,
		},
		{
			name:  "valid 6-digit mixed case",
			color: "#Ff00Aa",
			want:  true,
		},
		{
			name:  "valid green",
			color: "#00ff00",
			want:  true,
		},
		{
			name:  "valid blue",
			color: "#0000ff",
			want:  true,
		},
		{
			name:  "valid white",
			color: "#ffffff",
			want:  true,
		},
		{
			name:  "valid black",
			color: "#000000",
			want:  true,
		},
		{
			name:  "missing hash",
			color: "ff0000",
			want:  false,
		},
		{
			name:  "only 5 digits",
			color: "#00000",
			want:  false,
		},
		{
			name:  "only 4 digits",
			color: "#0000",
			want:  false,
		},
		{
			name:  "only 3 digits",
			color: "#000",
			want:  false,
		},
		{
			name:  "only 2 digits",
			color: "#00",
			want:  false,
		},
		{
			name:  "only 1 digit",
			color: "#0",
			want:  false,
		},
		{
			name:  "only hash",
			color: "#",
			want:  false,
		},
		{
			name:  "empty string",
			color: "",
			want:  false,
		},
		{
			name:  "invalid characters",
			color: "#gggggg",
			want:  false,
		},
		{
			name:  "with spaces",
			color: "#ff 00",
			want:  false,
		},
		{
			name:  "too many digits",
			color: "#0000000",
			want:  false,
		},
		{
			name:  "name instead of hex",
			color: "red",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidHexColor(tt.color)
			assert.Equal(t, tt.want, got, "isValidHexColor(%q) = %v, want %v", tt.color, got, tt.want)
		})
	}
}

// TestListRepoLabelsFn_MissingOwner tests error handling when owner is missing
func TestListRepoLabelsFn_MissingOwner(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"repo": "test-repo",
			},
		},
	}

	result, err := ListRepoLabelsFn(nil, req)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestListRepoLabelsFn_MissingRepo tests error handling when repo is missing
func TestListRepoLabelsFn_MissingRepo(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"owner": "test-owner",
			},
		},
	}

	result, err := ListRepoLabelsFn(nil, req)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestCreateLabelFn_MissingRequiredParams tests error handling for missing required parameters
func TestCreateLabelFn_MissingRequiredParams(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		wantErr  bool
		errField string
	}{
		{
			name: "missing owner",
			args: map[string]interface{}{
				"repo":  "test-repo",
				"name":  "bug",
				"color": "#ff0000",
			},
			wantErr:  true,
			errField: "owner",
		},
		{
			name: "missing repo",
			args: map[string]interface{}{
				"owner": "test-owner",
				"name":  "bug",
				"color": "#ff0000",
			},
			wantErr:  true,
			errField: "repo",
		},
		{
			name: "missing name",
			args: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
				"color": "#ff0000",
			},
			wantErr:  true,
			errField: "name",
		},
		{
			name: "missing color",
			args: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
				"name":  "bug",
			},
			wantErr:  true,
			errField: "color",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: tt.args,
				},
			}

			result, err := CreateLabelFn(nil, req)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.errField)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestCreateLabelFn_InvalidColorFormat tests color validation
func TestCreateLabelFn_InvalidColorFormat(t *testing.T) {
	tests := []struct {
		name        string
		color       string
		wantErr     bool
		errContains string
	}{
		{
			name:        "missing hash",
			color:       "ff0000",
			wantErr:     true,
			errContains: "invalid color format",
		},
		{
			name:        "only 5 digits",
			color:       "#00000",
			wantErr:     true,
			errContains: "invalid color format",
		},
		{
			name:        "only 3 digits",
			color:       "#000",
			wantErr:     true,
			errContains: "invalid color format",
		},
		{
			name:        "invalid chars",
			color:       "#gggggg",
			wantErr:     true,
			errContains: "invalid color format",
		},
		{
			name:        "empty",
			color:       "",
			wantErr:     true,
			errContains: "invalid color format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]interface{}{
						"owner": "test-owner",
						"repo":  "test-repo",
						"name":  "bug",
						"color": tt.color,
					},
				},
			}

			result, err := CreateLabelFn(nil, req)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestEditLabelFn_InvalidColorFormat tests color validation in edit
func TestEditLabelFn_InvalidColorFormat(t *testing.T) {
	tests := []struct {
		name        string
		color       string
		wantErr     bool
		errContains string
	}{
		{
			name:        "missing hash",
			color:       "ff0000",
			wantErr:     true,
			errContains: "invalid color format",
		},
		{
			name:        "only 5 digits",
			color:       "#00000",
			wantErr:     true,
			errContains: "invalid color format",
		},
		{
			name:        "invalid chars",
			color:       "#gggggg",
			wantErr:     true,
			errContains: "invalid color format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]interface{}{
						"owner": "test-owner",
						"repo":  "test-repo",
						"id":    float64(123),
						"color": tt.color,
					},
				},
			}

			result, err := EditLabelFn(nil, req)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestEditLabelFn_NoChanges tests error handling when no changes are provided
func TestEditLabelFn_NoChanges(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
				"id":    float64(123),
			},
		},
	}

	result, err := EditLabelFn(nil, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "at least one of name, color, or description must be provided")
}

// TestDeleteLabelFn_MissingRequiredParams tests error handling for missing required parameters
func TestDeleteLabelFn_MissingRequiredParams(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		wantErr  bool
		errField string
	}{
		{
			name: "missing owner",
			args: map[string]interface{}{
				"repo": "test-repo",
				"id":   float64(123),
			},
			wantErr:  true,
			errField: "owner",
		},
		{
			name: "missing repo",
			args: map[string]interface{}{
				"owner": "test-owner",
				"id":    float64(123),
			},
			wantErr:  true,
			errField: "repo",
		},
		{
			name: "missing id",
			args: map[string]interface{}{
				"owner": "test-owner",
				"repo":  "test-repo",
			},
			wantErr:  true,
			errField: "id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: tt.args,
				},
			}

			result, err := DeleteLabelFn(nil, req)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tt.errField)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
