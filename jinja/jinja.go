// @todo: replace gonja with Go standard library text template.
// ptool uses gonja (https://github.com/noirbizarre/gonja) as template language.
// gonja is a Go port of Jinja which supported features & syntaxes is a subset of the latter.
// This package implements some Jinja filters that are currently missing in gonja.
package jinja

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/noirbizarre/gonja"
	"github.com/noirbizarre/gonja/exec"
	"github.com/pkg/errors"
)

func init() {
	// regex_search is a ansible filter that is commonly used in Jinja.
	// See https://docs.ansible.com/ansible/latest/collections/ansible/builtin/regex_search_filter.html .
	// Only basic (1-arg) usage is implemented for now.
	gonja.DefaultEnv.Filters.Register("regex_search", func(e *exec.Evaluator, in *exec.Value,
		params *exec.VarArgs) *exec.Value {
		if p := params.ExpectArgs(1); p.IsError() {
			return exec.AsValue(errors.Wrap(p, "Wrong signature for 'regex_search'"))
		}
		regxp, err := regexp.Compile(params.Args[0].String())
		if err != nil {
			return exec.AsValue("")
		}
		return exec.AsValue(regxp.FindString(in.String()))
	})
}

func Render(template string, context map[string]any) (string, error) {
	tpl, err := gonja.FromString(template)
	if err != nil {
		return "", fmt.Errorf("failed to parse jinja template: %w", err)
	}
	out, err := tpl.Execute(context)
	if err != nil {
		return "", fmt.Errorf("failed to render payload: %w", err)
	}
	return strings.TrimSpace(out), nil
}
