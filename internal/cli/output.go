package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/lutefd/fabric/protocol"
)

const CLIVersion = "0.1.0"

var resultState struct {
	sync.Mutex
	value any
}

var jsonRunMu sync.Mutex

func setMachineResult(value any) {
	resultState.Lock()
	defer resultState.Unlock()
	resultState.value = value
}

func takeMachineResult() any {
	resultState.Lock()
	defer resultState.Unlock()
	value := resultState.value
	resultState.value = nil
	return value
}

type renderedError struct{ cause error }

func (e renderedError) Error() string { return e.cause.Error() }
func (e renderedError) Unwrap() error { return e.cause }

func IsRenderedError(err error) bool {
	var rendered renderedError
	return errors.As(err, &rendered)
}

type commandError struct {
	code    string
	message string
	details map[string]any
}

func (e commandError) Error() string { return e.message }

func typedError(code, message string, details map[string]any) error {
	return commandError{code: code, message: message, details: details}
}

func Run(args []string) error {
	format, clean, err := extractOutputFormat(args)
	if err != nil {
		return err
	}
	if format == "human" {
		takeMachineResult()
		return runCommand(clean)
	}

	jsonRunMu.Lock()
	defer jsonRunMu.Unlock()
	takeMachineResult()
	command := "help"
	if len(clean) > 0 {
		command = clean[0]
	}
	humanOutput, runErr := captureCommandStdout(func() error { return runCommand(clean) })
	data := takeMachineResult()
	if data == nil && strings.TrimSpace(humanOutput) != "" {
		data = map[string]any{"message": strings.TrimSpace(humanOutput)}
	}
	response := protocol.APIResponse{
		ProtocolVersion: protocol.SchemaVersion,
		Command:         command,
		OK:              runErr == nil,
		Data:            data,
	}
	if runErr != nil {
		response.Data = nil
		response.Error = &protocol.APIError{Code: errorCode(runErr), Message: runErr.Error(), Details: errorDetails(runErr)}
	}
	encoded, encodeErr := json.Marshal(response)
	if encodeErr != nil {
		return encodeErr
	}
	fmt.Fprintln(os.Stdout, string(encoded))
	if runErr != nil {
		return renderedError{cause: runErr}
	}
	return nil
}

func extractOutputFormat(args []string) (string, []string, error) {
	format := "human"
	clean := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--json":
			format = "json"
		case args[i] == "--format":
			i++
			if i >= len(args) {
				return "", nil, errors.New("--format requires human or json")
			}
			format = args[i]
		case strings.HasPrefix(args[i], "--format="):
			format = strings.TrimPrefix(args[i], "--format=")
		default:
			clean = append(clean, args[i])
		}
	}
	if format != "human" && format != "json" {
		return "", nil, errors.New("--format must be human or json")
	}
	return format, clean, nil
}

func captureCommandStdout(run func() error) (string, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return "", err
	}
	old := os.Stdout
	os.Stdout = writer
	done := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(reader)
		done <- data
	}()
	runErr := run()
	writer.Close()
	os.Stdout = old
	data := <-done
	reader.Close()
	return string(data), runErr
}

func errorCode(err error) string {
	var typed commandError
	if errors.As(err, &typed) {
		return typed.code
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "not found"), strings.Contains(message, "unknown thread"), strings.Contains(message, "unknown direction"):
		return "not_found"
	case strings.Contains(message, "conflict"), strings.Contains(message, "competing"), strings.Contains(message, "immutable"):
		return "conflict"
	case strings.Contains(message, "requires"), strings.Contains(message, "expected"), strings.Contains(message, "must be"), strings.Contains(message, "invalid"):
		return "invalid_argument"
	default:
		return "internal_error"
	}
}

func errorDetails(err error) map[string]any {
	var typed commandError
	if errors.As(err, &typed) {
		return typed.details
	}
	return nil
}

func runVersion(args []string) error {
	if len(args) > 0 {
		return errors.New("version accepts no arguments")
	}
	data := map[string]string{"cli_version": CLIVersion, "protocol_version": protocol.SchemaVersion}
	setMachineResult(data)
	fmt.Printf("Fabric %s (protocol %s)\n", CLIVersion, protocol.SchemaVersion)
	return nil
}

func runCapabilities(args []string) error {
	if len(args) > 0 {
		return errors.New("capabilities accepts no arguments")
	}
	data := map[string]any{
		"protocol_version": protocol.SchemaVersion,
		"event_types": []string{
			protocol.EventRecordCreated, protocol.EventRecordStateChanged, protocol.EventRelationCreated,
			protocol.EventThreadStarted, protocol.EventThreadScopeChanged, protocol.EventProjectionCreated, protocol.EventReceiptRecorded,
		},
		"relation_types": []string{
			protocol.RelationDerivedFrom, protocol.RelationInformedBy, protocol.RelationImplements,
			protocol.RelationSupersedes, protocol.RelationChallenges, protocol.RelationResolves,
			protocol.RelationDeliveredTo, protocol.RelationExposedTo,
		},
		"transports":     []string{"local-git"},
		"output_formats": []string{"human", "json"},
		"operations": []string{
			"thread.start", "projection.create", "projection.acknowledge",
			"record.create", "record.state_change", "relation.create", "graph.explain",
		},
		"explanation": map[string]bool{
			"causal_relations": true, "availability_relations": true,
			"resolved_record_details": true, "relation_assertion_details": true,
			"projection_details": true, "thread_details": true,
		},
	}
	setMachineResult(data)
	fmt.Println("Protocol:", protocol.SchemaVersion)
	fmt.Println("Transports: local-git")
	fmt.Println("Output formats: human, json")
	return nil
}
