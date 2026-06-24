package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/lutefd/fabric/protocol"
)

func runContext(args []string) error {
	if len(args) == 0 || args[0] != "acknowledge" {
		return errors.New(`expected "fabric context acknowledge"`)
	}
	fs := flag.NewFlagSet("context acknowledge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectionID := fs.String("projection", "", "projection ID")
	threadID := fs.String("thread", "", "thread ID override")
	state := fs.String("state", protocol.ReceiptExposed, "acknowledgement state: delivered or exposed")
	provider := fs.String("provider", "", "provider that exposed the context")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *projectionID == "" {
		return errors.New("context acknowledge requires --projection")
	}
	if *state != protocol.ReceiptDelivered && *state != protocol.ReceiptExposed {
		return errors.New("--state must be delivered or exposed")
	}
	projection, err := loadProjection(*projectionID)
	if err != nil {
		return err
	}
	if *threadID != "" {
		projection.ThreadID = *threadID
	}
	if projection.ThreadID == "" {
		return errors.New("projection has no thread; pass --thread")
	}
	receipt, err := recordProjectionReceipt(projection, *state, *provider)
	if err != nil {
		return err
	}
	setMachineResult(receipt)
	fmt.Printf("Acknowledged projection %s as %s for thread %s.\n", projection.ProjectionID, *state, projection.ThreadID)
	return nil
}
