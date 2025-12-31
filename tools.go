package mcpserve

import (
	"fmt"
	"reflect"

	"github.com/mark3labs/mcp-go/mcp"
)

// Loggable defines the interface for handlers that support logging
type Loggable interface {
	Name() string
	SetLog(logger func(message ...any))
}

// ToolExecutor defines how a tool should be executed
// Handlers implement this to provide execution logic without exposing internals
// args: map of parameter name to value from MCP request
type ToolExecutor func(args map[string]any)

// ToolMetadata provides MCP tool configuration metadata
// This is the standard interface that all handlers should implement
type ToolMetadata struct {
	Name        string
	Description string
	Parameters  []ParameterMetadata
	Execute     ToolExecutor // Handler provides execution function
}

// ParameterMetadata describes a tool parameter
type ParameterMetadata struct {
	Name        string
	Description string
	Required    bool
	Type        string // "string", "number", "boolean"
	EnumValues  []string
	Default     any
}

// mcpToolsFromHandler loads all MCP tools from a handler using reflection
// Looks for a method called "GetMCPToolsMetadata() []ToolMetadata"
func mcpToolsFromHandler(handler any) ([]ToolMetadata, error) {
	handlerValue := reflect.ValueOf(handler)
	method := handlerValue.MethodByName("GetMCPToolsMetadata")

	if !method.IsValid() {
		return nil, fmt.Errorf("method GetMCPToolsMetadata not found on handler")
	}

	// Call the method (should return []ToolMetadata or compatible slice)
	results := method.Call(nil)
	if len(results) != 1 {
		return nil, fmt.Errorf("method GetMCPToolsMetadata should return exactly one value")
	}

	// Convert result to []ToolMetadata using reflection
	return convertToToolMetadataSlice(results[0].Interface())
}

// convertToToolMetadataSlice converts any compatible slice to []ToolMetadata
func convertToToolMetadataSlice(source any) ([]ToolMetadata, error) {
	sourceValue := reflect.ValueOf(source)

	// Check if it's a slice
	if sourceValue.Kind() != reflect.Slice {
		return nil, fmt.Errorf("expected slice, got %T", source)
	}

	count := sourceValue.Len()
	result := make([]ToolMetadata, count)

	for i := 0; i < count; i++ {
		meta, err := convertToToolMetadata(sourceValue.Index(i).Interface())
		if err != nil {
			return nil, fmt.Errorf("tool %d: %w", i, err)
		}
		result[i] = meta
	}

	return result, nil
}

// convertToToolMetadata converts any compatible struct to ToolMetadata using reflection
func convertToToolMetadata(source any) (ToolMetadata, error) {
	sourceValue := reflect.ValueOf(source)

	// Check if it's already the correct type
	if meta, ok := source.(ToolMetadata); ok {
		return meta, nil
	}

	// Use reflection to extract fields
	meta := ToolMetadata{}

	// Extract Name field
	if nameField := sourceValue.FieldByName("Name"); nameField.IsValid() && nameField.Kind() == reflect.String {
		meta.Name = nameField.String()
	} else {
		return meta, fmt.Errorf("missing or invalid Name field in %T", source)
	}

	// Extract Description field
	if descField := sourceValue.FieldByName("Description"); descField.IsValid() && descField.Kind() == reflect.String {
		meta.Description = descField.String()
	}

	// Extract Parameters field
	if paramsField := sourceValue.FieldByName("Parameters"); paramsField.IsValid() && paramsField.Kind() == reflect.Slice {
		paramsCount := paramsField.Len()
		meta.Parameters = make([]ParameterMetadata, paramsCount)

		for i := 0; i < paramsCount; i++ {
			paramValue := paramsField.Index(i)
			param, err := convertToParameterMetadata(paramValue.Interface())
			if err != nil {
				return meta, fmt.Errorf("parameter %d: %w", i, err)
			}
			meta.Parameters[i] = param
		}
	}

	// Extract Execute field (function)
	if execField := sourceValue.FieldByName("Execute"); execField.IsValid() && execField.Kind() == reflect.Func {
		// Convert to ToolExecutor by wrapping the function
		meta.Execute = func(args map[string]any) {
			// Create reflection values for call
			execField.Call([]reflect.Value{
				reflect.ValueOf(args),
			})
		}
	}

	return meta, nil
}

// convertToParameterMetadata converts any compatible struct to ParameterMetadata
func convertToParameterMetadata(source any) (ParameterMetadata, error) {
	sourceValue := reflect.ValueOf(source)

	// Check if it's already the correct type
	if param, ok := source.(ParameterMetadata); ok {
		return param, nil
	}

	param := ParameterMetadata{}

	// Extract Name
	if field := sourceValue.FieldByName("Name"); field.IsValid() && field.Kind() == reflect.String {
		param.Name = field.String()
	}

	// Extract Description
	if field := sourceValue.FieldByName("Description"); field.IsValid() && field.Kind() == reflect.String {
		param.Description = field.String()
	}

	// Extract Required
	if field := sourceValue.FieldByName("Required"); field.IsValid() && field.Kind() == reflect.Bool {
		param.Required = field.Bool()
	}

	// Extract Type
	if field := sourceValue.FieldByName("Type"); field.IsValid() && field.Kind() == reflect.String {
		param.Type = field.String()
	}

	// Extract EnumValues
	if field := sourceValue.FieldByName("EnumValues"); field.IsValid() && field.Kind() == reflect.Slice {
		count := field.Len()
		param.EnumValues = make([]string, count)
		for i := 0; i < count; i++ {
			if elem := field.Index(i); elem.Kind() == reflect.String {
				param.EnumValues[i] = elem.String()
			}
		}
	}

	// Extract Default
	if field := sourceValue.FieldByName("Default"); field.IsValid() {
		param.Default = field.Interface()
	}

	return param, nil
}

// buildMCPTool constructs MCP tool from metadata
func buildMCPTool(meta ToolMetadata) *mcp.Tool {
	options := []mcp.ToolOption{
		mcp.WithDescription(meta.Description),
	}

	for _, param := range meta.Parameters {
		switch param.Type {
		case "string":
			// Build string parameter options directly
			var strOpts []mcp.PropertyOption

			if param.Required {
				strOpts = append(strOpts, mcp.Required())
			}
			if param.Description != "" {
				strOpts = append(strOpts, mcp.Description(param.Description))
			}
			if len(param.EnumValues) > 0 {
				strOpts = append(strOpts, mcp.Enum(param.EnumValues...))
			}
			if param.Default != nil {
				if defaultStr, ok := param.Default.(string); ok {
					strOpts = append(strOpts, mcp.DefaultString(defaultStr))
				}
			}

			options = append(options, mcp.WithString(param.Name, strOpts...))

		case "number":
			// Build number parameter options directly
			var numOpts []mcp.PropertyOption

			if param.Required {
				numOpts = append(numOpts, mcp.Required())
			}
			if param.Description != "" {
				numOpts = append(numOpts, mcp.Description(param.Description))
			}
			if param.Default != nil {
				if defaultNum, ok := param.Default.(float64); ok {
					numOpts = append(numOpts, mcp.DefaultNumber(defaultNum))
				}
			}

			options = append(options, mcp.WithNumber(param.Name, numOpts...))

		case "boolean":
			// Build boolean parameter options directly
			var boolOpts []mcp.PropertyOption

			if param.Required {
				boolOpts = append(boolOpts, mcp.Required())
			}
			if param.Description != "" {
				boolOpts = append(boolOpts, mcp.Description(param.Description))
			}
			// Note: DefaultBoolean might not exist in mcp-go, skip for now

			options = append(options, mcp.WithBoolean(param.Name, boolOpts...))
		}
	}

	tool := mcp.NewTool(meta.Name, options...)
	return &tool
}
