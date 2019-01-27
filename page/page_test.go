package page

import (
	"html/template"
	"io/ioutil"
	"testing"
)

func TestTemplates(t *testing.T) {
	pages := map[string]string{
		"index":  Index,
		"error":  Error,
		"result": Result,
		"read":   Read,
	}
	for name, p := range pages {
		tpl, err := template.New(name).Parse(p)
		if err != nil {
			t.Errorf("failed parse '%v': %v", name, err)
		}
		err = tpl.Execute(ioutil.Discard, nil)
		if err != nil {
			t.Errorf("failed execute '%v': %v", name, err)
		}
	}
}
