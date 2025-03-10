package oci_hook_render

import (
	"fmt"
	"os"
	"text/template"
)

type Renderer struct {
	hookTemplatePath string
	hookScriptPath   string
	runtimeCmd       string
}

func New(templatePath, hookPath, runtime string) *Renderer {
	return &Renderer{
		hookTemplatePath: templatePath,
		hookScriptPath:   hookPath,
		runtimeCmd:       runtime,
	}
}

type TemplateData struct {
	RuntimeCmd string
}

func (r *Renderer) Render() error {
	const RuncBinary = "runc"
	const CrunBinary = "crun"

	var err error
	var data TemplateData

	switch r.runtimeCmd {
	case RuncBinary:
		data.RuntimeCmd = fmt.Sprintf("%s update $CONTAINERID --memory-swap -1\n", RuncBinary)
	case CrunBinary:
		data.RuntimeCmd = fmt.Sprintf("%s update --memory-swap=-1 $CONTAINERID\n", CrunBinary)
	default:
		return fmt.Errorf("unsupported OCI runtime %s", r.runtimeCmd)
	}

	dstFile, err := os.Create(r.hookScriptPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	tmpl, err := template.ParseFiles(r.hookTemplatePath)
	if err != nil {
		return fmt.Errorf("error while parsing: %v", err)
	}

	err = tmpl.Execute(dstFile, data)
	if err != nil {
		return fmt.Errorf("error while executing: %v", err)
	}

	return nil
}
