package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"
)

type Format string

const (
	FormatJSON  Format = "json"
	FormatText  Format = "text"
	FormatTable Format = "table"
)

// Envelope is the standard response contract for command output.
type Envelope struct {
	Success  bool           `json:"success"`
	Data     any            `json:"data,omitempty"`
	Error    *ErrorPayload  `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ErrorPayload provides structured error details.
type ErrorPayload struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// PrintSuccess prints a successful command envelope.
func PrintSuccess(data any, metadata map[string]any, format Format) error {
	return Print(Envelope{
		Success:  true,
		Data:     data,
		Metadata: metadata,
	}, format)
}

// PrintCodedError prints an error envelope with a machine-readable error code.
func PrintCodedError(code, message string, err error, metadata map[string]any, format Format) error {
	detail := ""
	if err != nil {
		detail = err.Error()
	}

	payload := Envelope{
		Success: false,
		Error: &ErrorPayload{
			Code:    code,
			Message: message,
			Detail:  detail,
		},
		Metadata: metadata,
	}

	if format == FormatJSON {
		if marshalErr := printJSON(payload, os.Stdout); marshalErr != nil {
			return marshalErr
		}
	} else {
		if marshalErr := printJSON(payload, os.Stderr); marshalErr != nil {
			return marshalErr
		}
	}

	if err != nil {
		return err
	}
	return errors.New(message)
}

// PrintErrorResponse prints an error envelope and returns an error for command flow control.
func PrintErrorResponse(message string, err error, metadata map[string]any, format Format) error {
	return PrintCodedError("", message, err, metadata, format)
}

// Print renders output according to requested format.
func Print(data any, format Format) error {
	switch format {
	case FormatText:
		return printText(data)
	case FormatTable:
		return printTable(data)
	default:
		return printJSON(data, os.Stdout)
	}
}

func printJSON(data any, out *os.File) error {
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(body))
	return err
}

func printText(data any) error {
	value := reflect.ValueOf(data)
	if !value.IsValid() {
		fmt.Println("null")
		return nil
	}
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			fmt.Println(formatValue(value.Index(i).Interface()))
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			fmt.Printf("%s: %v\n", key, value.MapIndex(key))
		}
	case reflect.Struct:
		typeInfo := value.Type()
		for i := 0; i < value.NumField(); i++ {
			fmt.Printf("%s: %v\n", typeInfo.Field(i).Name, value.Field(i).Interface())
		}
	default:
		fmt.Println(formatValue(data))
	}
	return nil
}

func printTable(data any) error {
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	value := reflect.ValueOf(data)
	if !value.IsValid() {
		return nil
	}
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Slice || value.Len() == 0 {
		return printText(data)
	}

	element := value.Index(0)
	if element.Kind() == reflect.Ptr {
		element = element.Elem()
	}
	if element.Kind() != reflect.Struct {
		return printText(data)
	}

	typeInfo := element.Type()
	headers := make([]string, typeInfo.NumField())
	for i := 0; i < typeInfo.NumField(); i++ {
		headers[i] = strings.ToUpper(typeInfo.Field(i).Name)
	}
	if _, err := fmt.Fprintln(writer, strings.Join(headers, "\t")); err != nil {
		return err
	}

	for i := 0; i < value.Len(); i++ {
		row := value.Index(i)
		if row.Kind() == reflect.Ptr {
			row = row.Elem()
		}
		cells := make([]string, row.NumField())
		for j := 0; j < row.NumField(); j++ {
			cells[j] = fmt.Sprintf("%v", row.Field(j).Interface())
		}
		if _, err := fmt.Fprintln(writer, strings.Join(cells, "\t")); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func formatValue(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(body)
}
