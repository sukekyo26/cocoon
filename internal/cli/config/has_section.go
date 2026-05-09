package configcli

import (
	"io"

	"github.com/sukekyo26/cocoon/internal/logx"
)

func cmdHasSection(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 2, "has-section", stderr); err != nil {
		return err
	}
	data, err := decodeRaw(args[0])
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	out := "false"
	if _, ok := data[args[1]]; ok {
		out = "true"
	}
	log.Info(out)
	return nil
}

func cmdListSidecars(args []string, stdout, stderr io.Writer) error {
	log := logx.New(stdout, stderr)
	if err := requireArgs(args, 1, "list-sidecars", stderr); err != nil {
		return err
	}
	data, err := decodeRaw(args[0])
	if err != nil {
		log.Errorf("ERROR: %s", err)
		return ErrFailure
	}
	services := asMap(data["services"])
	for _, name := range sortedKeys(services) {
		log.Info(name)
	}
	return nil
}
