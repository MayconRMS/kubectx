package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// RenameOp indicates intention to rename contexts.
type RenameOp struct {
	New string // NAME of New context
	Old string // NAME of Old context (or '.' for current-context)
}

// parseRenameSyntax parses A=B form into [A,B] and returns
// whether it is parsed correctly.
func parseRenameSyntax(v string) (string, string, bool) {
	s := strings.Split(v, "=")
	if len(s) != 2 {
		return "", "", false
	}
	new, old := s[0], s[1]
	if new == "" || old == "" {
		return "", "", false
	}
	return new, old, true
}

// rename changes the old (NAME or '.' for current-context)
// to the "new" value. If the old refers to the current-context,
// current-context preference is also updated.
func (op RenameOp) Run(_, _ io.Writer) error {
	f, rootNode, err := openKubeconfig()
	if err != nil {
		return nil
	}
	defer f.Close()

	cur := getCurrentContext(rootNode)
	if op.Old == "." {
		op.Old = cur
	}

	if !checkContextExists(rootNode, op.Old) {
		return errors.Errorf("context %q not found, can't rename it", op.Old)
	}

	if checkContextExists(rootNode, op.New) {
		printWarning("context %q exists, overwriting it.", op.New)
		if err := modifyDocToDeleteContext(rootNode, op.New); err != nil {
			return errors.Wrap(err, "failed to delete new context to overwrite it")
		}
	}

	if err := modifyContextName(rootNode, op.Old, op.New); err != nil {
		return errors.Wrap(err, "failed to change context name")
	}
	if op.New == cur {
		if err := modifyCurrentContext(rootNode, op.New); err != nil {
			return errors.Wrap(err, "failed to set current-context to new name")
		}
	}
	if err := saveKubeconfigRaw(f, rootNode); err != nil {
		return errors.Wrap(err, "failed to save modified kubeconfig")
	}
	return nil
}

func modifyContextName(rootNode *yaml.Node, old, new string) error {
	if rootNode.Kind != yaml.MappingNode {
		return errors.New("root doc is not a mapping node")
	}
	contexts := valueOf(rootNode, "contexts")
	if contexts == nil {
		return errors.New("\"contexts\" entry is nil")
	} else if contexts.Kind != yaml.SequenceNode {
		return errors.New("\"contexts\" is not a sequence node")
	}

	var changed bool
	for _, contextNode := range contexts.Content {
		nameNode := valueOf(contextNode, "name")
		if nameNode.Kind == yaml.ScalarNode && nameNode.Value == old {
			nameNode.Value = new
			changed = true
			break
		}
	}
	if !changed {
		return errors.New("no changes were made")
	}
	// TODO use printSuccess
	// TODO consider moving printing logic to main
	fmt.Fprintf(os.Stderr, "Context %q renamed to %q.\n", old, new)
	return nil
}
